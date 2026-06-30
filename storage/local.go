package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
