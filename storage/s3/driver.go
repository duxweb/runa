package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/storage"
)

// Driver creates an S3-compatible storage driver.
func Driver(items ...Option) storage.Driver {
	opts := defaultOptions()
	for _, item := range items {
		if item != nil {
			item(&opts)
		}
	}
	if opts.name == "" {
		opts.name = "s3"
	}
	if opts.region == "" {
		opts.region = "auto"
	}
	client := opts.client
	if client == nil {
		cfg := aws.Config{Region: opts.region}
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
	presigner := opts.presigner
	if presigner == nil && client != nil {
		presigner = awss3.NewPresignClient(client)
	}
	return &driver{options: opts, client: client, presigner: presigner}
}

type driver struct {
	options   options
	client    *awss3.Client
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
	info := storage.FileInfo{Path: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: strings.Trim(aws.ToString(out.ETag), `"`), LastModified: aws.ToTime(out.LastModified), Meta: make(core.Map)}
	return out.Body, info, nil
}

func (driver *driver) Delete(ctx context.Context, paths ...string) error {
	if err := driver.ready(); err != nil {
		return err
	}
	for _, path := range paths {
		key, err := cleanPath(path)
		if err != nil {
			return err
		}
		if _, err := driver.client.DeleteObject(core.NormalizeContext(ctx), &awss3.DeleteObjectInput{Bucket: aws.String(driver.options.bucket), Key: aws.String(key)}); err != nil {
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
	return storage.FileInfo{Path: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), ETag: strings.Trim(aws.ToString(out.ETag), `"`), LastModified: aws.ToTime(out.LastModified), Meta: make(core.Map)}, nil
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
