package rate

type ipContext interface {
	IP() string
}

type userContext interface {
	UserID() string
}

type sessionContext interface {
	Session(...string) interface{ ID() string }
}

type routeInfo interface {
	RouteID() string
}

type routeContext interface {
	Route() routeInfo
}

type headerContext interface {
	HeaderString(string) string
}

type cookieContext interface {
	CookieValue(string) (string, bool)
}

type paramContext interface {
	ParamString(string) string
}

type queryContext interface {
	QueryString(string) string
}

// ByIP uses ctx.IP() as a rate key source.
func ByIP() KeySource {
	return KeySourceFunc{SourceName: "ip", Resolve: func(ctx any) string {
		if value, ok := ctx.(ipContext); ok {
			return value.IP()
		}
		return ""
	}}
}

// ByUser uses current auth user id when auth is installed.
func ByUser() KeySource {
	return KeySourceFunc{SourceName: "user", Resolve: func(ctx any) string {
		if value, ok := ctx.(interface{ UserID() string }); ok {
			return value.UserID()
		}
		return ""
	}}
}

// BySession uses current session id as a rate key source.
func BySession() KeySource {
	return KeySourceFunc{SourceName: "session", Resolve: func(ctx any) string {
		if value, ok := ctx.(sessionContext); ok {
			if sess := value.Session(); sess != nil {
				return sess.ID()
			}
		}
		return ""
	}}
}

// ByRoute uses route name or method+path as a rate key source.
func ByRoute() KeySource {
	return KeySourceFunc{SourceName: "route", Resolve: func(ctx any) string {
		if value, ok := ctx.(routeContext); ok {
			item := value.Route()
			if item == nil {
				return ""
			}
			return item.RouteID()
		}
		return ""
	}}
}

// ByHeader uses a request header as a rate key source.
func ByHeader(name string) KeySource {
	return KeySourceFunc{SourceName: "header:" + name, Resolve: func(ctx any) string {
		if value, ok := ctx.(headerContext); ok {
			return value.HeaderString(name)
		}
		return ""
	}}
}

// ByCookie uses a request cookie as a rate key source.
func ByCookie(name string) KeySource {
	return KeySourceFunc{SourceName: "cookie:" + name, Resolve: func(ctx any) string {
		if value, ok := ctx.(interface{ CookieValue(string) (string, bool) }); ok {
			item, _ := value.CookieValue(name)
			return item
		}
		return ""
	}}
}

// ByParam uses a route param as a rate key source.
func ByParam(name string) KeySource {
	return KeySourceFunc{SourceName: "param:" + name, Resolve: func(ctx any) string {
		if value, ok := ctx.(paramContext); ok {
			return value.ParamString(name)
		}
		return ""
	}}
}

// ByQuery uses a query value as a rate key source.
func ByQuery(name string) KeySource {
	return KeySourceFunc{SourceName: "query:" + name, Resolve: func(ctx any) string {
		if value, ok := ctx.(queryContext); ok {
			return value.QueryString(name)
		}
		return ""
	}}
}

// ByFunc uses a custom function as a rate key source.
func ByFunc(fn func(any) string) KeySource {
	return KeySourceFunc{SourceName: "custom", Resolve: func(ctx any) string {
		if fn == nil {
			return ""
		}
		return fn(ctx)
	}}
}
