package jsonrpc

import (
	"context"

	"github.com/duxweb/runa/scope"
)

// Context stores one JSON-RPC call context.
type Context struct {
	context   context.Context
	server    *Server
	request   Request
	transport string
	scope     *scope.Scope
}

func newContext(parent context.Context, server *Server, request Request, transport ...string) *Context {
	if parent == nil {
		parent = context.Background()
	}
	currentScope := scope.New(parent, scope.HTTP)
	value := "direct"
	if len(transport) > 0 && transport[0] != "" {
		value = transport[0]
	}
	return &Context{context: currentScope.Context(), server: server, request: request, transport: value, scope: currentScope}
}

// Context returns the underlying context.
func (ctx *Context) Context() context.Context {
	if ctx == nil || ctx.context == nil {
		return context.Background()
	}
	return ctx.context
}

// SetContext replaces the current call context.
func (ctx *Context) SetContext(value context.Context) {
	if ctx == nil {
		return
	}
	if value == nil {
		value = context.Background()
	}
	ctx.context = value
	if ctx.scope != nil {
		ctx.scope.SetContext(value)
	}
}

// Server returns the current JSON-RPC server.
func (ctx *Context) Server() *Server { return ctx.server }

// Request returns the current JSON-RPC request object.
func (ctx *Context) Request() Request { return ctx.request }

// Method returns the current method name.
func (ctx *Context) Method() string { return ctx.request.Method }

// Transport returns the current call transport.
func (ctx *Context) Transport() string {
	if ctx == nil || ctx.transport == "" {
		return "direct"
	}
	return ctx.transport
}

// Scope returns the current call scope.
func (ctx *Context) Scope() *scope.Scope { return ctx.scope }

func (ctx *Context) close() {
	if ctx != nil && ctx.scope != nil {
		ctx.scope.Close()
	}
}
