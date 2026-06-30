package storage

import (
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/duxweb/runa/core"
)

type disk struct {
	name    string
	driver  Driver
	options DiskOptions
}

func newDisk(name string, driver Driver, options DiskOptions) Disk {
	return &disk{name: name, driver: driver, options: options}
}

func (disk *disk) Put(ctx context.Context, path string, body io.Reader, options ...FileOption) error {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return err
	}
	return disk.driver.Put(ctx, fullPath, body, applyFileOptions(options...))
}

func (disk *disk) PutBytes(ctx context.Context, path string, data []byte, options ...FileOption) error {
	return disk.Put(ctx, path, bytes.NewReader(data), options...)
}

func (disk *disk) PutString(ctx context.Context, path string, data string, options ...FileOption) error {
	return disk.Put(ctx, path, strings.NewReader(data), options...)
}

func (disk *disk) Get(ctx context.Context, path string) (io.ReadCloser, FileInfo, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return nil, FileInfo{}, err
	}
	reader, info, err := disk.driver.Get(ctx, fullPath)
	if err != nil {
		return nil, FileInfo{}, err
	}
	info.Path = trimDiskPrefix(disk.options.Prefix, info.Path)
	return reader, info, nil
}

func (disk *disk) GetBytes(ctx context.Context, path string) ([]byte, error) {
	reader, _, err := disk.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (disk *disk) GetString(ctx context.Context, path string) (string, error) {
	data, err := disk.GetBytes(ctx, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (disk *disk) Delete(ctx context.Context, paths ...string) error {
	ctx = core.NormalizeContext(ctx)
	fullPaths := make([]string, 0, len(paths))
	for _, item := range paths {
		fullPath, err := joinPath(disk.options.Prefix, item)
		if err != nil {
			return err
		}
		fullPaths = append(fullPaths, fullPath)
	}
	return disk.driver.Delete(ctx, fullPaths...)
}

func (disk *disk) Exists(ctx context.Context, path string) (bool, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return false, err
	}
	return disk.driver.Exists(ctx, fullPath)
}

func (disk *disk) Size(ctx context.Context, path string) (int64, error) {
	info, err := disk.Info(ctx, path)
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (disk *disk) Info(ctx context.Context, path string) (FileInfo, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := disk.driver.Info(ctx, fullPath)
	if err != nil {
		return FileInfo{}, err
	}
	info.Path = trimDiskPrefix(disk.options.Prefix, info.Path)
	return info, nil
}

func (disk *disk) Copy(ctx context.Context, from string, to string, options ...FileOption) error {
	reader, info, err := disk.Get(ctx, from)
	if err != nil {
		return err
	}
	defer reader.Close()
	fileOptions := applyFileOptions(options...)
	fileOptions = fileOptionsWithContentType(fileOptions, info.ContentType)
	return disk.Put(ctx, to, reader, fileOptionsToOptions(fileOptions)...)
}

func (disk *disk) Move(ctx context.Context, from string, to string, options ...FileOption) error {
	if err := disk.Copy(ctx, from, to, options...); err != nil {
		return err
	}
	return disk.Delete(ctx, from)
}

func (disk *disk) URL(ctx context.Context, path string) (string, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return "", err
	}
	return disk.driver.URL(ctx, fullPath, URLOptions{
		Public:    disk.options.Public,
		Prefix:    disk.options.Prefix,
		URLPrefix: disk.options.URLPrefix,
		Domain:    disk.options.Domain,
		Meta:      core.CloneMap(disk.options.Meta),
	})
}

func (disk *disk) TempURL(ctx context.Context, path string, ttl time.Duration) (string, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return "", err
	}
	return disk.driver.TempURL(ctx, fullPath, ttl, URLOptions{
		Public:    disk.options.Public,
		Prefix:    disk.options.Prefix,
		URLPrefix: disk.options.URLPrefix,
		Domain:    disk.options.Domain,
		Meta:      core.CloneMap(disk.options.Meta),
	})
}

func (disk *disk) SignPut(ctx context.Context, path string, ttl time.Duration, options ...FileOption) (SignedURL, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return SignedURL{}, err
	}
	return disk.driver.SignPut(ctx, fullPath, ttl, applyFileOptions(options...))
}

func (disk *disk) SignPost(ctx context.Context, path string, ttl time.Duration, options ...FileOption) (SignedPost, error) {
	ctx = core.NormalizeContext(ctx)
	fullPath, err := joinPath(disk.options.Prefix, path)
	if err != nil {
		return SignedPost{}, err
	}
	return disk.driver.SignPost(ctx, fullPath, ttl, applyFileOptions(options...))
}

func trimDiskPrefix(prefix string, name string) string {
	prefix = cleanPrefix(prefix)
	name = strings.TrimPrefix(strings.TrimPrefix(name, "/"), "./")
	if prefix == "" {
		return name
	}
	name = strings.TrimPrefix(name, prefix)
	return strings.TrimPrefix(name, "/")
}

func fileOptionsToOptions(options FileOptions) []FileOption {
	output := []FileOption{}
	if options.ContentType != "" {
		output = append(output, ContentType(options.ContentType))
	}
	for key, value := range options.Meta {
		output = append(output, FileMeta(key, value))
	}
	return output
}
