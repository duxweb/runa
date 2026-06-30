package route

import "net/http"

// SetCookie writes a response cookie.
func (ctx *Context) SetCookie(name string, value string, options ...func(*http.Cookie)) {
	cookie := &http.Cookie{Name: name, Value: value, Path: "/"}
	for _, option := range options {
		if option != nil {
			option(cookie)
		}
	}
	if len(name) >= len("__Secure-") && name[:len("__Secure-")] == "__Secure-" {
		cookie.Secure = true
	}
	if len(name) >= len("__Host-") && name[:len("__Host-")] == "__Host-" {
		cookie.Secure = true
		cookie.Path = "/"
		cookie.Domain = ""
	}
	http.SetCookie(ctx.writer, cookie)
}
