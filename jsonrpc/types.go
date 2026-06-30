package jsonrpc

import (
	"encoding/json"
)

const version = "2.0"

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ID is a JSON-RPC request identifier.
type ID = json.RawMessage

var nullID = ID("null")

// Request is a JSON-RPC 2.0 request object.
type Request struct {
	Version string          `json:"jsonrpc,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      ID              `json:"id,omitempty"`
}

// Response is a JSON-RPC 2.0 response object.
type Response struct {
	Version string `json:"jsonrpc"`
	Result  any    `json:"-"`
	Error   *Error `json:"error,omitempty"`
	ID      ID     `json:"id"`
}

func (response Response) MarshalJSON() ([]byte, error) {
	if response.Error != nil {
		type errorBody struct {
			Version string `json:"jsonrpc"`
			Error   *Error `json:"error"`
			ID      ID     `json:"id"`
		}
		return json.Marshal(errorBody{Version: response.Version, Error: response.Error, ID: response.ID})
	}
	result, err := json.Marshal(response.Result)
	if err != nil {
		return nil, err
	}
	type successBody struct {
		Version string `json:"jsonrpc"`
		Result  ID     `json:"result"`
		ID      ID     `json:"id"`
	}
	return json.Marshal(successBody{Version: response.Version, Result: result, ID: response.ID})
}

// Error is a JSON-RPC 2.0 error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	return err.Message
}

// Handler handles one JSON-RPC method call.
type Handler func(*Context) (any, error)

// Middleware wraps JSON-RPC method execution.
type Middleware func(Handler) Handler

// TypedHandler handles a typed JSON-RPC method call.
type TypedHandler[Input any, Output any] func(*Context, *Input) (*Output, error)
