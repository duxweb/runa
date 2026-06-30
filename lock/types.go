package lock

import (
	"context"
	"errors"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultName          = "default"
	Local                = "local"
	Schedule             = "schedule"
	Queue                = "queue"
	Cache                = "cache"
	DefaultDriver        = "memory"
	DefaultTTL           = 30 * time.Second
	DefaultWait          = 3 * time.Second
	DefaultRetryInterval = 100 * time.Millisecond
	DefaultRedisPrefix   = "runa:lock:"
)

var (
	ErrTimeout = errors.New("lock wait timeout")
	ErrNotHeld = errors.New("lock is not held")
)

// Locker is a named lock pool.
type Locker interface {
	Try(ctx context.Context, key string, options ...LockOption) (Lease, bool, error)
	Wait(ctx context.Context, key string, options ...LockOption) (Lease, error)
	With(ctx context.Context, key string, fn func(context.Context) error, options ...LockOption) error
}

// Lease is an acquired lock handle.
type Lease interface {
	Key() string
	Token() string
	Fencing() uint64
	Renew(ctx context.Context, ttl time.Duration) error
	Release(ctx context.Context) error
}

// State is the store-level lock state.
type State struct {
	Key       string
	Token     string
	Fencing   uint64
	ExpiresAt time.Time
}

// Driver is the primitive lock storage contract.
type Driver interface {
	Name() string
	Try(ctx context.Context, key string, token string, ttl time.Duration) (State, bool, error)
	Renew(ctx context.Context, key string, token string, ttl time.Duration) error
	Release(ctx context.Context, key string, token string) error
	Close(ctx context.Context) error
}

// Info describes a configured lock pool.
type Info struct {
	Name          string
	Driver        string
	Prefix        string
	TTL           time.Duration
	Wait          time.Duration
	RetryInterval time.Duration
	AutoRenew     bool
	Meta          core.Map
}
