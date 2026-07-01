package csrf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"

	"github.com/duxweb/runa/route"
)

type contextKey struct{}

// New creates CSRF middleware.
func New(options ...Option) route.Middleware {
	config := applyOptions(options...)
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if config.Next != nil && config.Next(ctx) {
				return next(ctx)
			}
			if skipPath(ctx, config.SkipPaths) {
				return next(ctx)
			}
			request := ctx.Request()
			if request == nil {
				return next(ctx)
			}
			cookieToken, hasCookie := ctx.CookieValue(config.CookieName)
			if safeMethod(request.Method) {
				token := cookieToken
				if token == "" {
					var err error
					token, err = generateToken()
					if err != nil {
						return err
					}
					writeCookie(ctx, config, token)
				}
				ctx.SetContext(context.WithValue(ctx.Context(), contextKey{}, token))
				return next(ctx)
			}
			if !hasCookie || cookieToken == "" {
				return reject(ctx, config)
			}
			requestToken := submittedToken(ctx, config)
			if !validToken(cookieToken, requestToken) {
				return reject(ctx, config)
			}
			ctx.SetContext(context.WithValue(ctx.Context(), contextKey{}, cookieToken))
			return next(ctx)
		}
	}
}

// Token returns the CSRF token stored on the current request.
func Token(ctx *route.Context) string {
	if ctx == nil {
		return ""
	}
	if token, ok := ctx.Context().Value(contextKey{}).(string); ok {
		return token
	}
	return ""
}

func submittedToken(ctx *route.Context, config Config) string {
	if token := ctx.Header[string](config.HeaderName); token != "" {
		return token
	}
	return ctx.Form[string](config.FormField)
}

func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func skipPath(ctx *route.Context, paths []string) bool {
	if len(paths) == 0 || ctx == nil || ctx.Request() == nil || ctx.Request().URL == nil {
		return false
	}
	current := ctx.Request().URL.Path
	for _, path := range paths {
		if path == current {
			return true
		}
	}
	return false
}

func writeCookie(ctx *route.Context, config Config, token string) {
	path := config.CookiePath
	domain := config.CookieDomain
	if hasHostPrefix(config.CookieName) {
		path = "/"
		domain = ""
	}
	http.SetCookie(ctx.Response(), &http.Cookie{
		Name:     config.CookieName,
		Value:    token,
		Path:     path,
		Domain:   domain,
		MaxAge:   config.MaxAge,
		Secure:   config.Secure || hasSecurePrefix(config.CookieName) || hasHostPrefix(config.CookieName),
		HttpOnly: config.HTTPOnly,
		SameSite: config.SameSite,
	})
}

func hasSecurePrefix(name string) bool {
	return len(name) >= len("__Secure-") && name[:len("__Secure-")] == "__Secure-"
}

func hasHostPrefix(name string) bool {
	return len(name) >= len("__Host-") && name[:len("__Host-")] == "__Host-"
}

func validToken(expected string, actual string) bool {
	if expected == "" || actual == "" || len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func reject(ctx *route.Context, config Config) error {
	if config.OnError != nil {
		return config.OnError(ctx)
	}
	return ctx.Error(http.StatusForbidden, "CSRF token mismatch")
}

func generateToken() (string, error) {
	const size = 32
	var bytes [size]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}
