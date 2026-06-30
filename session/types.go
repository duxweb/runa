package session

import (
	"context"
	"time"

	"github.com/duxweb/runa/core"
)

const (
	DriverMemory = "memory"
	DriverCache  = "cache"
	DriverCookie = "cookie"

	DefaultName = "web"
	Web         = "web"
	Admin       = "admin"
	API         = "api"

	DefaultCookieName = "__Host-runa_session"
	DefaultTTL        = 2 * time.Hour
)

// Driver stores session payloads.
type Driver interface {
	Name() string
	Load(ctx context.Context, id string) (core.Map, bool, error)
	Save(ctx context.Context, id string, data core.Map, ttl time.Duration) error
	Delete(ctx context.Context, id string) error
	Close(ctx context.Context) error
}

// Stateless stores session data in the cookie value itself.
type Stateless interface {
	Driver
	LoadValue(ctx context.Context, value string, options CookieOptions) (core.Map, bool, error)
	SaveValue(ctx context.Context, data core.Map, ttl time.Duration, options CookieOptions) (string, error)
}

// CookieSetter writes a Set-Cookie header.
type CookieSetter func(name string, value string, options CookieOptions)

// Info describes one configured session pool.
type Info struct {
	Name       string
	Driver     string
	CookieName string
	TTL        time.Duration
	Shared     bool
	Default    bool
	Meta       core.Map
}
