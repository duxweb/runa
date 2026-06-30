package auth

import (
	"net/http"
	"strings"
)

// TokenSource resolves token from a request context.
type TokenSource interface {
	Name() string
	Token(ctx any) string
}

type tokenSource struct {
	name string
	fn   func(any) string
}

func (source tokenSource) Name() string { return source.name }
func (source tokenSource) Token(ctx any) string {
	if source.fn == nil {
		return ""
	}
	return strings.TrimSpace(source.fn(ctx))
}

// Header reads token from a header. Authorization removes Bearer prefix.
func Header(name string) TokenSource {
	return tokenSource{name: "header:" + name, fn: func(ctx any) string {
		request := requestOf(ctx)
		if request == nil {
			return ""
		}
		token := request.Header.Get(name)
		if strings.EqualFold(name, "Authorization") && strings.HasPrefix(strings.ToLower(token), "bearer ") {
			return strings.TrimSpace(token[7:])
		}
		return token
	}}
}

// Cookie reads token from a cookie.
func Cookie(name string) TokenSource {
	return tokenSource{name: "cookie:" + name, fn: func(ctx any) string {
		if value, ok := ctx.(interface{ CookieValue(string) (string, bool) }); ok {
			token, _ := value.CookieValue(name)
			return token
		}
		request := requestOf(ctx)
		if request == nil {
			return ""
		}
		cookie, err := request.Cookie(name)
		if err != nil {
			return ""
		}
		return cookie.Value
	}}
}

// Query reads token from a query parameter.
func Query(name string) TokenSource {
	return tokenSource{name: "query:" + name, fn: func(ctx any) string {
		request := requestOf(ctx)
		if request == nil {
			return ""
		}
		return request.URL.Query().Get(name)
	}}
}

func firstToken(ctx any, sources []TokenSource) string {
	for _, source := range sources {
		if source == nil {
			continue
		}
		if token := source.Token(ctx); token != "" {
			return token
		}
	}
	return ""
}

func requestOf(ctx any) *http.Request {
	if value, ok := ctx.(interface{ Request() *http.Request }); ok {
		return value.Request()
	}
	return nil
}
