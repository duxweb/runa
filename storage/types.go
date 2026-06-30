package storage

import (
	"context"
	"io"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultDriver = "local"

	DiskLocal   = "local"
	DiskPublic  = "public"
	DiskPrivate = "private"
	DiskCloud   = "cloud"
)

// Disk is the business-facing named filesystem.
type Disk interface {
	Put(ctx context.Context, path string, body io.Reader, options ...FileOption) error
	PutBytes(ctx context.Context, path string, data []byte, options ...FileOption) error
	PutString(ctx context.Context, path string, data string, options ...FileOption) error

	Get(ctx context.Context, path string) (io.ReadCloser, FileInfo, error)
	GetBytes(ctx context.Context, path string) ([]byte, error)
	GetString(ctx context.Context, path string) (string, error)

	Delete(ctx context.Context, paths ...string) error
	Exists(ctx context.Context, path string) (bool, error)
	Size(ctx context.Context, path string) (int64, error)
	Info(ctx context.Context, path string) (FileInfo, error)
	Copy(ctx context.Context, from string, to string, options ...FileOption) error
	Move(ctx context.Context, from string, to string, options ...FileOption) error

	URL(ctx context.Context, path string) (string, error)
	TempURL(ctx context.Context, path string, ttl time.Duration) (string, error)
	SignPut(ctx context.Context, path string, ttl time.Duration, options ...FileOption) (SignedURL, error)
	SignPost(ctx context.Context, path string, ttl time.Duration, options ...FileOption) (SignedPost, error)
}

// Driver is the low-level storage adapter.
type Driver interface {
	Name() string
	Put(ctx context.Context, path string, body io.Reader, options FileOptions) error
	Get(ctx context.Context, path string) (io.ReadCloser, FileInfo, error)
	Delete(ctx context.Context, paths ...string) error
	Exists(ctx context.Context, path string) (bool, error)
	Info(ctx context.Context, path string) (FileInfo, error)
	URL(ctx context.Context, path string, options URLOptions) (string, error)
	TempURL(ctx context.Context, path string, ttl time.Duration, options URLOptions) (string, error)
	SignPut(ctx context.Context, path string, ttl time.Duration, options FileOptions) (SignedURL, error)
	SignPost(ctx context.Context, path string, ttl time.Duration, options FileOptions) (SignedPost, error)
	Close(ctx context.Context) error
}

// FileInfo describes one stored object.
type FileInfo struct {
	Path         string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	Meta         core.Map
}

// SignedURL describes a pre-signed request URL.
type SignedURL struct {
	URL     string
	Method  string
	Header  map[string]string
	Expires time.Time
}

// SignedPost describes a pre-signed form post.
type SignedPost struct {
	URL     string
	Fields  map[string]string
	Expires time.Time
}

// Info describes one configured disk.
type Info struct {
	Name    string
	Driver  string
	Prefix  string
	Public  bool
	Default bool
	Meta    core.Map
}
