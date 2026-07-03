package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/storage"
)

// Driver creates an S3-compatible storage driver.
func Driver(items ...Option) storage.Driver {
	opts := defaultOptions()
	applyOptions(&opts, items...)
	normalizeOptions(&opts)
	return newDriver(opts)
}

func newDriver(opts options) storage.Driver {
	client := opts.client
	uploader := opts.uploader
	if client == nil {
		loadOptions := []func(*awsconfig.LoadOptions) error{}
		if opts.region != "" {
			loadOptions = append(loadOptions, awsconfig.WithRegion(opts.region))
		}
		if opts.endpoint != "" {
			loadOptions = append(loadOptions, awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired))
		}
		cfg, err := awsconfig.LoadDefaultConfig(context.Background(), loadOptions...)
		if err != nil {
			cfg = aws.Config{Region: opts.region}
		}
		if cfg.Region == "" && opts.endpoint != "" {
			cfg.Region = "auto"
		}
		if opts.endpoint != "" {
			cfg.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		}
		if opts.accessKey != "" || opts.secretKey != "" || opts.sessionToken != "" {
			cfg.Credentials = credentials.NewStaticCredentialsProvider(opts.accessKey, opts.secretKey, opts.sessionToken)
		}
		client = awss3.NewFromConfig(cfg, func(options *awss3.Options) {
			if opts.endpoint != "" {
				options.BaseEndpoint = aws.String(strings.TrimRight(opts.endpoint, "/"))
			}
			options.UsePathStyle = opts.pathStyle
		})
	}
	if uploader == nil && client != nil {
		uploader = manager.NewUploader(client)
	}
	presigner := opts.presigner
	if presigner == nil && client != nil {
		presigner = awss3.NewPresignClient(client)
	}
	return &driver{options: opts, client: client, uploader: uploader, presigner: presigner}
}

type driver struct {
	options   options
	client    *awss3.Client
	uploader  *manager.Uploader
	presigner *awss3.PresignClient
}

func (driver *driver) Name() string { return driver.options.name }

func (driver *driver) Put(ctx context.Context, path string, body io.Reader, options storage.FileOptions) error {
	if err := driver.ready(); err != nil {
		return err
	}
	key, err := cleanPath(path)
	if err != nil {
		return err
	}
	input := &awss3.PutObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key), Body: body}
	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
	} else if value := mime.TypeByExtension(filepath.Ext(key)); value != "" {
		input.ContentType = aws.String(value)
	}
	if len(options.Meta) > 0 {
		input.Metadata = stringMeta(options.Meta)
	}
	if driver.uploader != nil {
		_, err = driver.uploader.Upload(core.NormalizeContext(ctx), input)
		return err
	}
	_, err = driver.client.PutObject(core.NormalizeContext(ctx), input)
	return err
}

func (driver *driver) Get(ctx context.Context, path string) (io.ReadCloser, storage.FileInfo, error) {
	if err := driver.ready(); err != nil {
		return nil, storage.FileInfo{}, err
	}
	key, err := cleanPath(path)
	if err != nil {
		return nil, storage.FileInfo{}, err
	}
	out, err := driver.client.GetObject(core.NormalizeContext(ctx), &awss3.GetObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, storage.FileInfo{}, err
	}
	info := storage.FileInfo{Path: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: strings.Trim(aws.ToString(out.ETag), `"`), LastModified: aws.ToTime(out.LastModified), Meta: core.Map{}}
	for key, value := range out.Metadata {
		info.Meta[key] = value
	}
	return out.Body, info, nil
}

func (driver *driver) Delete(ctx context.Context, paths ...string) error {
	if err := driver.ready(); err != nil {
		return err
	}
	keys := make([]types.ObjectIdentifier, 0, len(paths))
	for _, path := range paths {
		key, err := cleanPath(path)
		if err != nil {
			return err
		}
		keys = append(keys, types.ObjectIdentifier{Key: aws.String(key)})
	}
	ctx = core.NormalizeContext(ctx)
	if driver.options.endpoint != "" {
		for _, key := range keys {
			if _, err := driver.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(driver.options.bucket), Key: key.Key}); err != nil {
				return err
			}
		}
		return nil
	}
	for len(keys) > 0 {
		batchSize := 1000
		if len(keys) < batchSize {
			batchSize = len(keys)
		}
		batch := keys[:batchSize]
		keys = keys[batchSize:]
		if len(batch) == 1 {
			if _, err := driver.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(driver.options.bucket), Key: batch[0].Key}); err != nil {
				return err
			}
			continue
		}
		if _, err := driver.client.DeleteObjects(ctx, &awss3.DeleteObjectsInput{
			Bucket: aws.String(driver.options.bucket),
			Delete: &types.Delete{Objects: batch, Quiet: aws.Bool(true)},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (driver *driver) Exists(ctx context.Context, path string) (bool, error) {
	_, err := driver.Info(ctx, path)
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

func (driver *driver) Info(ctx context.Context, path string) (storage.FileInfo, error) {
	if err := driver.ready(); err != nil {
		return storage.FileInfo{}, err
	}
	key, err := cleanPath(path)
	if err != nil {
		return storage.FileInfo{}, err
	}
	out, err := driver.client.HeadObject(core.NormalizeContext(ctx), &awss3.HeadObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)})
	if err != nil {
		return storage.FileInfo{}, err
	}
	info := storage.FileInfo{Path: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: strings.Trim(aws.ToString(out.ETag), `"`), LastModified: aws.ToTime(out.LastModified), Meta: core.Map{}}
	for key, value := range out.Metadata {
		info.Meta[key] = value
	}
	return info, nil
}

func (driver *driver) List(ctx context.Context, prefix string, options storage.ListOptions) (storage.FileList, error) {
	if err := driver.ready(); err != nil {
		return storage.FileList{}, err
	}
	cleanedPrefix := cleanPrefix(prefix)
	limit := options.Limit
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	input := &awss3.ListObjectsV2Input{
		Bucket:            aws.String(driver.options.bucket),
		Prefix:            aws.String(cleanedPrefix),
		MaxKeys:           aws.Int32(limit),
		ContinuationToken: emptyStringNil(options.Cursor),
	}
	if !options.Recursive {
		input.Delimiter = aws.String("/")
	}
	out, err := driver.client.ListObjectsV2(core.NormalizeContext(ctx), input)
	if err != nil {
		return storage.FileList{}, err
	}
	list := storage.FileList{Items: make([]storage.FileInfo, 0, len(out.Contents)), Cursor: aws.ToString(out.NextContinuationToken), HasMore: aws.ToBool(out.IsTruncated)}
	for _, item := range out.Contents {
		key := aws.ToString(item.Key)
		if key == "" || strings.HasSuffix(key, "/") {
			continue
		}
		list.Items = append(list.Items, storage.FileInfo{
			Path:         key,
			Size:         aws.ToInt64(item.Size),
			ContentType:  contentType(key),
			ETag:         strings.Trim(aws.ToString(item.ETag), `"`),
			LastModified: aws.ToTime(item.LastModified),
			Meta:         core.Map{},
		})
	}
	for _, prefix := range out.CommonPrefixes {
		if value := aws.ToString(prefix.Prefix); value != "" {
			list.CommonDirs = append(list.CommonDirs, strings.TrimSuffix(value, "/"))
		}
	}
	return list, nil
}

func (driver *driver) Copy(ctx context.Context, from string, to string, options storage.FileOptions) error {
	if err := driver.ready(); err != nil {
		return err
	}
	fromKey, err := cleanPath(from)
	if err != nil {
		return err
	}
	toKey, err := cleanPath(to)
	if err != nil {
		return err
	}
	input := &awss3.CopyObjectInput{
		Bucket:     aws.String(driver.options.bucket),
		Key:        aws.String(toKey),
		CopySource: aws.String(copySource(driver.options.bucket, fromKey)),
	}
	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	if len(options.Meta) > 0 {
		input.Metadata = stringMeta(options.Meta)
		input.MetadataDirective = types.MetadataDirectiveReplace
	}
	_, err = driver.client.CopyObject(core.NormalizeContext(ctx), input)
	return err
}

func (driver *driver) Move(ctx context.Context, from string, to string, options storage.FileOptions) error {
	if err := driver.Copy(ctx, from, to, options); err != nil {
		return err
	}
	return driver.Delete(ctx, from)
}

func (driver *driver) URL(ctx context.Context, path string, options storage.URLOptions) (string, error) {
	if err := ctxErr(ctx); err != nil {
		return "", err
	}
	key, err := cleanPath(path)
	if err != nil {
		return "", err
	}
	domain := options.Domain
	if domain == "" {
		domain = driver.options.domain
	}
	prefix := options.URLPrefix
	if prefix == "" {
		prefix = driver.options.urlPrefix
	}
	if domain != "" {
		return joinURL(domain, prefix, key), nil
	}
	if driver.options.endpoint != "" {
		if driver.options.pathStyle {
			return joinURL(driver.options.endpoint, driver.options.bucket, key), nil
		}
		return joinURL(strings.TrimRight(driver.options.endpoint, "/")+"/"+driver.options.bucket, key), nil
	}
	return joinURL("https://"+driver.options.bucket+".s3."+driver.options.region+".amazonaws.com", key), nil
}

func (driver *driver) TempURL(ctx context.Context, path string, ttl time.Duration, options storage.URLOptions) (string, error) {
	if driver.presigner == nil {
		return "", fmt.Errorf("s3 presigner is not configured")
	}
	key, err := cleanPath(path)
	if err != nil {
		return "", err
	}
	request, err := driver.presigner.PresignGetObject(core.NormalizeContext(ctx), &awss3.GetObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)}, func(options *awss3.PresignOptions) { options.Expires = ttl })
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func (driver *driver) SignPut(ctx context.Context, path string, ttl time.Duration, options storage.FileOptions) (storage.SignedURL, error) {
	if driver.presigner == nil {
		return storage.SignedURL{}, fmt.Errorf("s3 presigner is not configured")
	}
	key, err := cleanPath(path)
	if err != nil {
		return storage.SignedURL{}, err
	}
	input := &awss3.PutObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)}
	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
	}
	expires := ttl
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	request, err := driver.presigner.PresignPutObject(core.NormalizeContext(ctx), input, func(options *awss3.PresignOptions) { options.Expires = expires })
	if err != nil {
		return storage.SignedURL{}, err
	}
	headers := map[string]string{}
	for key, values := range request.SignedHeader {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return storage.SignedURL{URL: request.URL, Method: http.MethodPut, Header: headers, Expires: core.Now().Add(expires)}, nil
}

func (driver *driver) SignPost(ctx context.Context, path string, ttl time.Duration, options storage.FileOptions) (storage.SignedPost, error) {
	if driver.presigner == nil {
		return storage.SignedPost{}, fmt.Errorf("s3 presigner is not configured")
	}
	key, err := cleanPath(path)
	if err != nil {
		return storage.SignedPost{}, err
	}
	input := &awss3.PutObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)}
	if options.ContentType != "" {
		input.ContentType = aws.String(options.ContentType)
	}
	expires := ttl
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	request, err := driver.presigner.PresignPostObject(core.NormalizeContext(ctx), input, func(options *awss3.PresignPostOptions) { options.Expires = expires })
	if err != nil {
		return storage.SignedPost{}, err
	}
	return storage.SignedPost{URL: request.URL, Fields: request.Values, Expires: core.Now().Add(expires)}, nil
}

func (driver *driver) Close(context.Context) error { return nil }

func (driver *driver) ready() error {
	if driver.client == nil {
		return fmt.Errorf("s3 client is nil")
	}
	if driver.options.bucket == "" {
		return fmt.Errorf("s3 bucket is required")
	}
	return nil
}

func cleanPath(value string) (string, error) {
	value = strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if value == "" || value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.Contains(value, "/../") {
		return "", fmt.Errorf("storage path is invalid: %s", value)
	}
	return value, nil
}

func cleanPrefix(value string) string {
	value = strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if value == "" || value == "." {
		return ""
	}
	if strings.HasPrefix(value, "../") || strings.Contains(value, "/../") || value == ".." {
		return ""
	}
	return strings.TrimSuffix(value, "/") + "/"
}

func joinURL(domain string, parts ...string) string {
	domain = strings.TrimRight(strings.TrimSpace(domain), "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.ReplaceAll(part, "\\", "/"), "/")
		if part != "" {
			clean = append(clean, part)
		}
	}
	path := strings.Join(clean, "/")
	if domain == "" {
		return "/" + path
	}
	if path == "" {
		return domain
	}
	return domain + "/" + path
}

func contentType(name string) string {
	if value := mime.TypeByExtension(filepath.Ext(name)); value != "" {
		return value
	}
	return core.MIMEOctetStream
}

func stringMeta(meta core.Map) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	output := make(map[string]string, len(meta))
	for key, value := range meta {
		if key == "" || value == nil {
			continue
		}
		output[key] = fmt.Sprint(value)
	}
	return output
}

func emptyStringNil(value string) *string {
	if value == "" {
		return nil
	}
	return aws.String(value)
}

func copySource(bucket string, key string) string {
	escaped := (&url.URL{Path: bucket + "/" + key}).EscapedPath()
	return strings.ReplaceAll(escaped, "+", "%2B")
}

func ctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func isNotFound(err error) bool {
	var api smithy.APIError
	if errors.As(err, &api) {
		code := strings.ToLower(api.ErrorCode())
		return code == "notfound" || code == "nosuchkey" || code == "404"
	}
	return false
}
