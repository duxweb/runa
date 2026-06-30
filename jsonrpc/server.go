package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Server stores JSON-RPC methods.
type Server struct {
	mu          sync.RWMutex
	methods     map[string]Handler
	middlewares []Middleware
}

// New creates a JSON-RPC server.
func New() *Server {
	return &Server{methods: make(map[string]Handler)}
}

// Register registers a method handler.
func (server *Server) Register(name string, handler Handler) *Server {
	if server == nil || name == "" || handler == nil {
		return server
	}
	server.mu.Lock()
	if server.methods == nil {
		server.methods = make(map[string]Handler)
	}
	server.methods[name] = handler
	server.mu.Unlock()
	return server
}

// Use adds JSON-RPC method middleware.
func (server *Server) Use(middlewares ...Middleware) *Server {
	if server == nil || len(middlewares) == 0 {
		return server
	}
	server.mu.Lock()
	for _, middleware := range middlewares {
		if middleware != nil {
			server.middlewares = append(server.middlewares, middleware)
		}
	}
	server.mu.Unlock()
	return server
}

// Method registers a typed JSON-RPC method.
func Method[Input any, Output any](server *Server, name string, handler TypedHandler[Input, Output]) *Server {
	if server == nil || handler == nil {
		return server
	}
	return server.Register(name, func(ctx *Context) (any, error) {
		var input Input
		if len(ctx.request.Params) > 0 {
			if err := json.Unmarshal(ctx.request.Params, &input); err != nil {
				return nil, invalidParams(err)
			}
		}
		return handler(ctx, &input)
	})
}

// Call calls one method directly.
func (server *Server) Call(ctx context.Context, method string, params any) (any, error) {
	if server == nil {
		return nil, methodNotFound(method)
	}
	var raw json.RawMessage
	if params != nil {
		body, err := json.Marshal(params)
		if err != nil {
			return nil, invalidParams(err)
		}
		raw = body
	}
	rpcCtx := newContext(ctx, server, Request{Version: version, Method: method, Params: raw}, "direct")
	defer rpcCtx.close()
	handler := server.handler(method)
	if handler == nil {
		return nil, methodNotFound(method)
	}
	return safeCall(rpcCtx, handler)
}

func (server *Server) handler(name string) Handler {
	if server == nil {
		return nil
	}
	server.mu.RLock()
	handler := server.methods[name]
	middlewares := append([]Middleware(nil), server.middlewares...)
	server.mu.RUnlock()
	if handler == nil {
		return nil
	}
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

func (server *Server) handle(ctx context.Context, request Request, transport ...string) *Response {
	if err := validateRequest(request); err != nil {
		return &Response{Version: version, Error: err, ID: responseID(request)}
	}
	handler := server.handler(request.Method)
	if handler == nil {
		return &Response{Version: version, Error: methodNotFound(request.Method), ID: responseID(request)}
	}
	rpcCtx := newContext(ctx, server, request, transport...)
	defer rpcCtx.close()
	result, err := safeCall(rpcCtx, handler)
	if err != nil {
		return &Response{Version: version, Error: responseError(err), ID: responseID(request)}
	}
	return &Response{Version: version, Result: result, ID: responseID(request)}
}

func safeCall(ctx *Context, handler Handler) (result any, err error) {
	defer func() {
		if value := recover(); value != nil {
			err = panicError(value)
		}
	}()
	return handler(ctx)
}

func validateRequest(request Request) *Error {
	if request.Version != version {
		return invalidRequest("jsonrpc must be 2.0")
	}
	if request.Method == "" {
		return invalidRequest("method is required")
	}
	if len(request.Params) > 0 && !isStructured(request.Params) {
		return invalidRequest("params must be object or array")
	}
	return nil
}

func responseID(request Request) ID {
	if len(request.ID) == 0 {
		return nullID
	}
	return request.ID
}

func isNotification(request Request) bool {
	return len(request.ID) == 0
}

func isStructured(value json.RawMessage) bool {
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return false
	}
	switch decoded.(type) {
	case map[string]any, []any:
		return true
	default:
		return false
	}
}

func decodeRequest(body []byte) (requests []Request, batch bool, rpcErr *Error) {
	if len(body) == 0 {
		return nil, false, invalidRequest("request body is empty")
	}
	var first any
	if err := json.Unmarshal(body, &first); err != nil {
		return nil, false, parseError()
	}
	switch first.(type) {
	case []any:
		batch = true
		if len(first.([]any)) == 0 {
			return nil, true, invalidRequest("batch request is empty")
		}
	case map[string]any:
	default:
		return nil, false, invalidRequest("request must be an object or array")
	}
	if batch {
		if err := json.Unmarshal(body, &requests); err != nil {
			return nil, true, invalidRequest(fmt.Sprintf("invalid batch request: %v", err))
		}
		return requests, true, nil
	}
	var request Request
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, false, invalidRequest(fmt.Sprintf("invalid request: %v", err))
	}
	return []Request{request}, false, nil
}
