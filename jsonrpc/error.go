package jsonrpc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/duxweb/runa/core"
	"github.com/duxweb/runa/route"
)

func parseError() *Error {
	return &Error{Code: CodeParseError, Message: "Parse error"}
}

func invalidRequest(message string) *Error {
	if message == "" {
		message = "Invalid Request"
	}
	return &Error{Code: CodeInvalidRequest, Message: message}
}

func methodNotFound(method string) *Error {
	return &Error{Code: CodeMethodNotFound, Message: "Method not found", Data: core.Map{"method": method}}
}

func invalidParams(err error) *Error {
	message := "Invalid params"
	if err != nil {
		message = err.Error()
	}
	return &Error{Code: CodeInvalidParams, Message: message}
}

func internalError(err error) *Error {
	message := "Internal error"
	if err != nil && route.ErrorStatus(err) < http.StatusInternalServerError {
		message = route.ErrorMessage(err)
	}
	value := &Error{Code: CodeInternalError, Message: message}
	if code := route.ErrorCode(err); code != "" && code != "internal" {
		value.Data = core.Map{"code": code, "params": route.ErrorParams(err)}
	}
	return value
}

func responseError(err error) *Error {
	if err == nil {
		return nil
	}
	var rpcErr *Error
	if errors.As(err, &rpcErr) {
		return rpcErr
	}
	return internalError(err)
}

func panicError(value any) error {
	return fmt.Errorf("panic: %v", value)
}
