package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/duxweb/runa/auth"
	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
	"github.com/duxweb/runa/session"
)

// Use enforces one of the named authenticators.
func Use(names ...string) route.Middleware {
	return authMiddleware(false, names...)
}

// Optional tries authenticators and allows missing credentials.
func Optional(names ...string) route.Middleware {
	return authMiddleware(true, names...)
}

// Permission checks current route permission id.
func Permission(checkers ...auth.PermissionChecker) route.Middleware {
	var checker auth.PermissionChecker
	if len(checkers) > 0 {
		checker = checkers[0]
	}
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if routeSkipPermission(ctx.Route()) {
				return next(ctx)
			}
			id := routeID(ctx.Route())
			if id == "" {
				return ctx.Error(http.StatusForbidden, "权限标识不能为空")
			}
			active := checker
			if active == nil {
				active = permissionChecker(ctx)
			} else {
				ctx.Locals("runa.auth.permission_checker", active)
			}
			if err := active.Check(ctx, authInfo(ctx), id); err != nil {
				return ctx.Error(http.StatusForbidden, err)
			}
			return next(ctx)
		}
	}
}

// PermissionInfo returns route permission snapshots.
func PermissionInfo(routes []*route.Route) []auth.PermissionInfo {
	items := []auth.PermissionInfo{}
	for _, item := range routes {
		if item == nil || routeSkipPermission(item) {
			continue
		}
		id := routeID(item)
		if id == "" {
			continue
		}
		items = append(items, auth.PermissionInfo{
			ID:          id,
			Name:        auth.ShortName(id),
			Label:       item.SummaryText,
			Group:       auth.GroupName(id),
			Method:      item.Method,
			Path:        item.Path,
			Tags:        append([]string(nil), item.TagList...),
			Description: item.DescriptionText,
			Meta:        cloneMeta(item.MetaData),
		})
	}
	return items
}

func authMiddleware(optional bool, names ...string) route.Middleware {
	return func(next route.Handler) route.Handler {
		return func(ctx *route.Context) error {
			if routeSkipAuth(ctx.Route()) {
				return next(ctx)
			}
			mode := routeAuthMode(ctx.Route())
			isOptional := optional || mode == "optional"
			info, err := authenticate(ctx, names...)
			if err != nil {
				if isOptional && auth.IsNoCredentials(err) {
					return next(ctx)
				}
				return ctx.Error(http.StatusUnauthorized, err)
			}
			if info == nil {
				if isOptional {
					return next(ctx)
				}
				return ctx.Error(http.StatusUnauthorized, "请登录后再访问")
			}
			ctx.Locals("runa.auth", info)
			return next(ctx)
		}
	}
}

func authenticate(ctx *route.Context, names ...string) (*auth.Info, error) {
	if len(names) == 0 {
		names = []string{"default"}
	}
	var missing bool
	var errs []error
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		authenticator := authenticator(ctx, name)
		if authenticator == nil {
			errs = append(errs, errors.New("auth "+name+" is not registered"))
			continue
		}
		info, err := authenticator.Authenticate(authContext{Context: ctx})
		if err == nil && info != nil {
			info.Name = name
			if info.Data == nil {
				info.Data = make(core.Map)
			}
			return info, nil
		}
		if err != nil && auth.IsNoCredentials(err) {
			missing = true
			continue
		}
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if missing {
		return nil, auth.ErrNoCredentials{}
	}
	return nil, nil
}

type authContext struct {
	*route.Context
}

func (ctx authContext) Session(name ...string) *session.Session {
	registry := ctx.Context.Service[*session.Registry]()
	if registry == nil {
		return nil
	}
	value := session.Web
	if len(name) > 0 && name[0] != "" {
		value = name[0]
	}
	localKey := "runa.session." + value
	if cached, ok := ctx.Locals(localKey).(*session.Session); ok {
		return cached
	}
	options, ok := registry.Options(value)
	if !ok {
		return nil
	}
	raw := ""
	if cookie, err := ctx.Request().Cookie(options.CookieName); err == nil {
		raw = cookie.Value
	}
	sess, err := registry.Load(ctx.Context.Context(), value, raw, func(name string, value string, options session.CookieOptions) {
		writeCookie(ctx.Context, name, value, options)
	})
	if err != nil {
		return nil
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

func registry(ctx *route.Context) *auth.Registry {
	return ctx.Service[*auth.Registry]()
}

func authenticator(ctx *route.Context, name string) auth.Authenticator {
	registry := registry(ctx)
	if registry == nil {
		return nil
	}
	return registry.Of(name)
}

func authInfo(ctx *route.Context) *auth.Info {
	info, _ := ctx.Locals("runa.auth").(*auth.Info)
	return info
}

func permissionChecker(ctx *route.Context) auth.PermissionChecker {
	if checker, ok := ctx.Locals("runa.auth.permission_checker").(auth.PermissionChecker); ok {
		return checker
	}
	registry := registry(ctx)
	if registry == nil {
		return auth.DefaultPermissionChecker()
	}
	return registry.Checker()
}

func routeSkipAuth(item *route.Route) bool {
	if item == nil || item.MetaData == nil {
		return false
	}
	value, ok := item.MetaData["auth"].(bool)
	return ok && !value
}

func routeAuthMode(item *route.Route) string {
	if item == nil {
		return ""
	}
	return item.MetaAs[string]("auth")
}

func routeSkipPermission(item *route.Route) bool {
	return item != nil && item.MetaAs[bool]("can") == false && item.MetaData != nil && item.MetaData["can"] != nil
}

func routeID(item *route.Route) string {
	if item == nil {
		return ""
	}
	return item.RouteID()
}

func cloneMeta(value core.Map) core.Map {
	if value == nil {
		return nil
	}
	cloned := make(core.Map, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}
