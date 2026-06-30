package auth

import (
	"context"
	"net/http"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/session"
)

const (
	MethodSession = "session"
	MethodAPIKey  = "api_key"
	MethodJWT     = "jwt"
)

// Context is the minimal request context needed by authenticators.
type Context interface {
	Context() context.Context
	Request() *http.Request
	CookieValue(string) (string, bool)
	Session(...string) *session.Session
}

// Authenticator resolves request identity.
type Authenticator interface {
	Authenticate(ctx any) (*Info, error)
}

// AuthFunc adapts a function to Authenticator.
type AuthFunc func(ctx any) (*Info, error)

func (fn AuthFunc) Authenticate(ctx any) (*Info, error) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx)
}

// Info is request auth information.
type Info struct {
	Name   string
	Method string
	Data   core.Map
}

// PermissionChecker checks one permission id.
type PermissionChecker interface {
	Check(ctx any, info *Info, id string) error
}

// PermissionFunc adapts a function to PermissionChecker.
type PermissionFunc func(ctx any, info *Info, id string) error

func (fn PermissionFunc) Check(ctx any, info *Info, id string) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, info, id)
}

// PermissionInfo describes one route permission.
type PermissionInfo struct {
	ID          string
	Name        string
	Label       string
	Group       string
	GroupLabel  string
	Method      string
	Path        string
	Tags        []string
	Description string
	Meta        core.Map
}

// ErrNoCredentials marks missing credentials.
type ErrNoCredentials struct{}

func (ErrNoCredentials) Error() string { return "auth credentials are missing" }

func IsNoCredentials(err error) bool {
	_, ok := err.(ErrNoCredentials)
	return ok
}
