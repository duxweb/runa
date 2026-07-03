package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
)

type localDriver struct {
	options DriverOptions
	root    string
}

// LocalDriver creates a local filesystem storage driver.
func LocalDriver(options ...DriverOption) Driver {
	opts := applyDriverOptions(options...)
	return &localDriver{options: opts, root: filepath.Clean(opts.Root)}
}

func (driver *localDriver) Name() string { return driver.options.Name }

func (driver *localDriver) Put(ctx context.Context, path string, body io.Reader, options FileOptions) error {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	fullPath, err := driver.fullPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (driver *localDriver) Get(ctx context.Context, path string) (io.ReadCloser, FileInfo, error) {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, FileInfo{}, err
	}
	fullPath, err := driver.fullPath(path)
	if err != nil {
		return nil, FileInfo{}, err
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, FileInfo{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, FileInfo{}, err
	}
	return file, driver.fileInfo(path, info), nil
}

func (driver *localDriver) Delete(ctx context.Context, paths ...string) error {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, item := range paths {
		fullPath, err := driver.fullPath(item)
		if err != nil {
			return err
		}
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (driver *localDriver) Exists(ctx context.Context, path string) (bool, error) {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return false, err
	}
	fullPath, err := driver.fullPath(path)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (driver *localDriver) Info(ctx context.Context, path string) (FileInfo, error) {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}
	fullPath, err := driver.fullPath(path)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return FileInfo{}, err
	}
	return driver.fileInfo(path, info), nil
}

func (driver *localDriver) List(ctx context.Context, prefix string, options ListOptions) (FileList, error) {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return FileList{}, err
	}
	cleanedPrefix := cleanPrefix(prefix)
	root := driver.root
	if cleanedPrefix != "" {
		fullPath, err := driver.fullPath(cleanedPrefix)
		if err != nil {
			return FileList{}, err
		}
		root = fullPath
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return FileList{}, nil
		}
		return FileList{}, err
	}
	limit := options.Limit
	if limit <= 0 {
		limit = 1000
	}
	items := make([]FileInfo, 0)
	commonDirs := make([]string, 0)
	if options.Recursive {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if path == root {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			itemPath := filepath.ToSlash(rel)
			if cleanedPrefix != "" {
				itemPath = cleanedPrefix + "/" + itemPath
			}
			items = append(items, driver.fileInfo(itemPath, info))
			return nil
		})
		if err != nil {
			return FileList{}, err
		}
		return pageFileList(items, commonDirs, options.Cursor, limit), nil
	}
	for _, entry := range entries {
		name := entry.Name()
		path := name
		if cleanedPrefix != "" {
			path = cleanedPrefix + "/" + name
		}
		if entry.IsDir() {
			commonDirs = append(commonDirs, path)
		} else {
			info, err := entry.Info()
			if err != nil {
				return FileList{}, err
			}
			items = append(items, driver.fileInfo(path, info))
		}
	}
	return pageFileList(items, commonDirs, options.Cursor, limit), nil
}

func (driver *localDriver) Copy(ctx context.Context, from string, to string, options FileOptions) error {
	reader, info, err := driver.Get(ctx, from)
	if err != nil {
		return err
	}
	defer reader.Close()
	options = fileOptionsWithContentType(options, info.ContentType)
	return driver.Put(ctx, to, reader, options)
}

func (driver *localDriver) Move(ctx context.Context, from string, to string, options FileOptions) error {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return err
	}
	fromPath, err := driver.fullPath(from)
	if err != nil {
		return err
	}
	toPath, err := driver.fullPath(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(toPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(fromPath, toPath); err == nil {
		return nil
	}
	if err := driver.Copy(ctx, from, to, options); err != nil {
		return err
	}
	return driver.Delete(ctx, from)
}

func (driver *localDriver) URL(ctx context.Context, path string, options URLOptions) (string, error) {
	ctx = core.NormalizeContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cleaned, err := cleanPath(path)
	if err != nil {
		return "", err
	}
	domain := options.Domain
	if domain == "" {
		domain = driver.options.Domain
	}
	prefix := options.URLPrefix
	if prefix == "" {
		prefix = driver.options.URLPrefix
	}
	return joinURL(domain, prefix, cleaned), nil
}

func (driver *localDriver) TempURL(ctx context.Context, path string, ttl time.Duration, options URLOptions) (string, error) {
	baseURL, err := driver.URL(ctx, path, options)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		return baseURL, nil
	}
	if driver.options.Secret == "" {
		return "", fmt.Errorf("storage signing secret is not configured")
	}
	cleaned, err := cleanPath(path)
	if err != nil {
		return "", err
	}
	expires := core.Now().Add(ttl).Unix()
	signature := driver.sign("GET", cleaned, expires, "")
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("expires", strconv.FormatInt(expires, 10))
	query.Set("signature", signature)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (driver *localDriver) SignPut(ctx context.Context, path string, ttl time.Duration, options FileOptions) (SignedURL, error) {
	cleaned, err := cleanPath(path)
	if err != nil {
		return SignedURL{}, err
	}
	baseURL, err := driver.URL(ctx, cleaned, URLOptions{})
	if err != nil {
		return SignedURL{}, err
	}
	if driver.options.Secret == "" {
		return SignedURL{}, fmt.Errorf("storage signing secret is not configured")
	}
	expiresAt := core.Now().Add(ttl)
	if ttl <= 0 {
		expiresAt = core.Now()
	}
	expires := expiresAt.Unix()
	signature := driver.sign(http.MethodPut, cleaned, expires, options.ContentType)
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return SignedURL{}, err
	}
	query := parsed.Query()
	query.Set("expires", strconv.FormatInt(expires, 10))
	query.Set("signature", signature)
	parsed.RawQuery = query.Encode()
	headers := map[string]string{}
	if options.ContentType != "" {
		headers["Content-Type"] = options.ContentType
	}
	return SignedURL{URL: parsed.String(), Method: http.MethodPut, Header: headers, Expires: expiresAt}, nil
}

func (driver *localDriver) SignPost(ctx context.Context, path string, ttl time.Duration, options FileOptions) (SignedPost, error) {
	cleaned, err := cleanPath(path)
	if err != nil {
		return SignedPost{}, err
	}
	baseURL, err := driver.URL(ctx, cleaned, URLOptions{})
	if err != nil {
		return SignedPost{}, err
	}
	if driver.options.Secret == "" {
		return SignedPost{}, fmt.Errorf("storage signing secret is not configured")
	}
	expiresAt := core.Now().Add(ttl)
	if ttl <= 0 {
		expiresAt = core.Now()
	}
	expires := expiresAt.Unix()
	fields := map[string]string{
		"key":       cleaned,
		"expires":   strconv.FormatInt(expires, 10),
		"signature": driver.sign(http.MethodPost, cleaned, expires, options.ContentType),
	}
	if options.ContentType != "" {
		fields["Content-Type"] = options.ContentType
	}
	return SignedPost{URL: baseURL, Fields: fields, Expires: expiresAt}, nil
}

func (driver *localDriver) Close(context.Context) error { return nil }

func (driver *localDriver) fullPath(path string) (string, error) {
	cleaned, err := cleanPath(path)
	if err != nil {
		return "", err
	}
	rootReal, err := realPath(driver.root)
	if err != nil {
		return "", err
	}
	fullPath := filepath.Clean(filepath.Join(driver.root, filepath.FromSlash(cleaned)))
	fullReal, err := realPath(fullPath)
	if err != nil {
		return "", err
	}
	if fullReal != rootReal && !strings.HasPrefix(fullReal, rootReal+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage path escapes root: %s", path)
	}
	return fullReal, nil
}

func realPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	current := abs
	parts := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			items := append([]string{resolved}, parts...)
			return filepath.Clean(filepath.Join(items...)), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(abs), nil
		}
		parts = append([]string{filepath.Base(current)}, parts...)
		current = parent
	}
}

func (driver *localDriver) fileInfo(path string, info os.FileInfo) FileInfo {
	cleaned, _ := cleanPath(path)
	return FileInfo{
		Path:         cleaned,
		Size:         info.Size(),
		ContentType:  contentType(cleaned),
		LastModified: info.ModTime(),
		Meta:         make(core.Map),
	}
}

func pageFileList(items []FileInfo, commonDirs []string, cursor string, limit int32) FileList {
	output := FileList{Items: make([]FileInfo, 0, len(items)), CommonDirs: make([]string, 0, len(commonDirs))}
	started := cursor == ""
	for _, item := range items {
		if !started {
			if item.Path <= cursor {
				continue
			}
			started = true
		}
		output.Items = append(output.Items, item)
		if int32(len(output.Items)) >= limit {
			output.Cursor = item.Path
			output.HasMore = true
			return output
		}
	}
	if cursor == "" {
		output.CommonDirs = append(output.CommonDirs, commonDirs...)
	}
	return output
}

func (driver *localDriver) sign(method string, path string, expires int64, contentType string) string {
	secret := driver.options.Secret
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(method + "\n" + path + "\n" + strconv.FormatInt(expires, 10) + "\n" + contentType))
	return hex.EncodeToString(mac.Sum(nil))
}

func contentType(name string) string {
	if value := mime.TypeByExtension(filepath.Ext(name)); value != "" {
		return value
	}
	return core.MIMEOctetStream
}
