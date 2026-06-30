package cache

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultName      = "default"
	Route            = "route"
	Config           = "config"
	View             = "view"
	Permission       = "permission"
	Session          = "session"
	DefaultDriver    = "memory"
	DefaultTTL       = 10 * time.Minute
	DefaultCapacity  = 64 * 1024 * 1024
	DefaultL1TTL     = time.Minute
	DefaultRedisPref = "runa:cache:"
)

// Cache is a typed cache pool.
type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, bool, error)
	GetMany(ctx context.Context, keys []string) (map[string]T, []string, error)
	Set(ctx context.Context, key string, value T, ttl time.Duration) error
	SetMany(ctx context.Context, values map[string]T, ttl time.Duration) error
	Remember(ctx context.Context, key string, ttl time.Duration, loader func(context.Context) (T, error)) (T, error)
	Has(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, keys ...string) error
	Purge(ctx context.Context) error
	Stats(ctx context.Context) Stats
}

// Driver is the byte-level cache storage contract.
type Driver interface {
	Name() string
	Get(ctx context.Context, key string) ([]byte, bool, error)
	GetMany(ctx context.Context, keys []string) (map[string][]byte, []string, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	SetMany(ctx context.Context, values map[string][]byte, ttl time.Duration) error
	Has(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, keys ...string) error
	Purge(ctx context.Context) error
	Close(ctx context.Context) error
	Stats(ctx context.Context) Stats
}

// Serializer converts typed values to bytes stored by Driver.
type Serializer interface {
	Marshal(value any) ([]byte, error)
	Unmarshal(data []byte, value any) error
}

// Stats describes runtime cache usage.
type Stats struct {
	Name     string
	Driver   string
	Hit      uint64
	Miss     uint64
	Set      uint64
	Delete   uint64
	Size     int64
	Capacity int64
	Meta     core.Map
}

// Info describes a configured cache pool.
type Info struct {
	Name   string
	Driver string
	Prefix string
	TTL    time.Duration
	Meta   core.Map
}
