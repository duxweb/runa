package middleware

import (
	"errors"
	"net/http"

	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/session"
)

// Use loads a named session before the handler and saves it afterwards.
func Use(name ...string) route.Middleware {
	value := session.Web
	if len(name) > 0 && name[0] != "" {
		value = name[0]
	}
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			registry := route.Service[*session.Registry](ctx)
			if registry == nil {
				return errors.New("session is not configured")
			}
			if sessionOf(ctx, registry, value) == nil {
				return errors.New("session is not configured")
			}
			return next(ctx)
		}
	}
}

func sessionOf(ctx *route.Context, registry *session.Registry, name string) *session.Session {
	localKey := "runa.session." + name
	if cached, ok := ctx.Locals(localKey).(*session.Session); ok {
		return cached
	}
	options, ok := registry.Options(name)
	if !ok {
		panic("session " + name + " is not registered")
	}
	raw := ""
	if cookie, err := ctx.Request().Cookie(options.CookieName); err == nil {
		raw = cookie.Value
	}
	sess, err := registry.Load(ctx.Context(), name, raw, func(name string, value string, options session.CookieOptions) {
		writeCookie(ctx, name, value, options)
	})
	if err != nil {
		panic(err)
	}
	ctx.Locals(localKey, sess)
	ctx.AddSaver(sess.Save)
	return sess
}

func writeCookie(ctx *route.Context, name string, value string, options session.CookieOptions) {
	cookie := &http.Cookie{
		Name:        name,
		Value:       value,
		Path:        options.Path,
		Domain:      options.Domain,
		Expires:     options.Expires,
		MaxAge:      options.MaxAge,
		Secure:      options.Secure,
		HttpOnly:    options.HTTPOnly,
		SameSite:    options.SameSite,
		Partitioned: options.Partitioned,
	}
	http.SetCookie(ctx.Response(), cookie)
}
