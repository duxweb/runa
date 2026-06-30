package rate

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DefaultDriver = "memory"

	DefaultName = "api"
	API         = "api"
	Login       = "login"
	Admin       = "admin"
	SMS         = "sms"
)

type Algorithm string

const (
	AlgorithmTokenBucket   Algorithm = "token_bucket"
	AlgorithmFixedWindow   Algorithm = "fixed_window"
	AlgorithmSlidingWindow Algorithm = "sliding_window"
)

// Limiter is a named limiter.
type Limiter interface {
	Allow(ctx context.Context, keys ...string) (Result, error)
	Reset(ctx context.Context, keys ...string) error
}

// Driver handles limit primitives.
type Driver interface {
	Name() string
	Allow(ctx context.Context, rule Rule, key string) (Result, error)
	Reset(ctx context.Context, rule Rule, key string) error
	Close(ctx context.Context) error
}

// KeySource resolves one key part outside the core package.
type KeySource interface {
	Name() string
	Value(ctx any) string
}

// KeySourceFunc adapts a function to KeySource.
type KeySourceFunc struct {
	SourceName string
	Resolve    func(any) string
}

func (source KeySourceFunc) Name() string { return source.SourceName }
func (source KeySourceFunc) Value(ctx any) string {
	if source.Resolve == nil {
		return ""
	}
	return source.Resolve(ctx)
}

// Rule describes one limiter rule.
type Rule struct {
	Name      string
	Driver    string
	Algorithm Algorithm
	Limit     int
	Window    time.Duration
	Burst     int
	Key       []KeySource
	Meta      core.Map
}

// Result describes one allow decision.
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

// Info describes one configured limiter.
type Info struct {
	Name      string
	Driver    string
	Algorithm Algorithm
	Limit     int
	Window    time.Duration
	Burst     int
	Default   bool
	Meta      core.Map
}
