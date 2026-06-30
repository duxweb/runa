package errs

import (
	"errors"

	"github.com/duxweb/runa/core"
)

// Map stores error attributes.
type Map = core.Map

// Error is Runa's framework/business error.
type Error struct {
	Code    string
	Message string
	Params  Map
	Cause   error
	frames  []Frame
}

// Error returns the business error message.
func (err *Error) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Message
	}
	if err.Cause != nil {
		return err.Cause.Error()
	}
	return "error"
}

// Unwrap returns the cause error.
func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// WithCode sets a business error code.
func (err *Error) WithCode(code string) *Error {
	if err != nil {
		err.Code = code
	}
	return err
}

// WithParams sets business error parameters.
func (err *Error) WithParams(params Map) *Error {
	if err != nil {
		err.Params = params
	}
	return err
}

// ErrorParams returns translation/rendering parameters.
func (err *Error) ErrorParams() core.Map {
	if err == nil {
		return nil
	}
	return err.Params
}

// Source returns the top source frame captured when this error was created.
func (err *Error) Source() string {
	if err == nil || len(err.frames) == 0 {
		return ""
	}
	frame := err.frames[0]
	frame.File = trimSource(frame.File)
	return frame.String()
}

// Stack returns captured source frames.
func (err *Error) Stack() []Frame {
	if err == nil || len(err.frames) == 0 {
		return nil
	}
	return append([]Frame(nil), err.frames...)
}

// StackTrace returns captured source frames formatted as text.
func (err *Error) StackTrace() string {
	if err == nil {
		return ""
	}
	return formatStack(err.frames)
}

// Option configures a framework/business error.
type Option func(*Error)

// New creates a framework/business error.
func New(message string, options ...Option) error {
	err := &Error{Message: message, frames: captureFrames(1)}
	for _, option := range options {
		if option != nil {
			option(err)
		}
	}
	return err
}

// Wrap attaches caller information to an existing error.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Cause: err, frames: captureFrames(1)}
}

// Code sets the business error code.
func Code(code string) Option {
	return func(err *Error) {
		err.Code = code
	}
}

// Params sets business error parameters.
func Params(params Map) Option {
	return func(err *Error) {
		err.Params = params
	}
}

// Attr sets a single business error attribute.
func Attr(key string, value any) Option {
	return func(err *Error) {
		if err.Params == nil {
			err.Params = Map{}
		}
		err.Params[key] = value
	}
}

// Cause sets the cause error.
func Cause(cause error) Option {
	return func(err *Error) {
		err.Cause = cause
	}
}

// As returns a errs Error when err contains one.
func As(err error) *Error {
	var errsErr *Error
	if errors.As(err, &errsErr) {
		return errsErr
	}
	return nil
}
