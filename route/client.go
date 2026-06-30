package route

import (
	"net"
	"strings"
)

const (
	LocalRequestID = "request_id"
	LocalIP        = "ip"
	LocalScheme    = "scheme"
	LocalHost      = "host"
)

// RequestID returns the current request id.
func (ctx *Context) RequestID() string {
	return ctx.scope.GetAs[string](LocalRequestID)
}

// IP returns the client IP.
func (ctx *Context) IP() string {
	if ip := ctx.scope.GetAs[string](LocalIP); ip != "" {
		return ip
	}
	if ctx.request == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(ctx.request.RemoteAddr)
	if err == nil {
		return host
	}
	return ctx.request.RemoteAddr
}

// Scheme returns the request scheme.
func (ctx *Context) Scheme() string {
	if scheme := ctx.scope.GetAs[string](LocalScheme); scheme != "" {
		return scheme
	}
	if ctx.request == nil {
		return ""
	}
	if ctx.request.TLS != nil {
		return "https"
	}
	return "http"
}

// Host returns the request host.
func (ctx *Context) Host() string {
	if host := ctx.scope.GetAs[string](LocalHost); host != "" {
		return host
	}
	if ctx.request == nil {
		return ""
	}
	return ctx.request.Host
}

// Hostname returns the request hostname without port.
func (ctx *Context) Hostname() string {
	host := ctx.Host()
	name, _, err := net.SplitHostPort(host)
	if err == nil {
		return name
	}
	return strings.Trim(host, "[]")
}
