package validate

import (
	"errors"
	"net/http"

	"github.com/duxweb/runa/core"
)

// FieldError describes a single validation field error.
type FieldError struct {
	Source  string
	Field   string
	Name    string
	Code    string
	Message string
	Params  core.Map
}

// ValidationError describes validation errors.
type ValidationError struct {
	Errors []FieldError
	Cause  error
}

// Error returns the first field error message or a default message.
func (err *ValidationError) Error() string {
	if err == nil || len(err.Errors) == 0 {
		return "Validation Error"
	}
	return err.Errors[0].Message
}

// Unwrap returns the cause error.
func (err *ValidationError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

// ErrorStatus returns the HTTP status.
func (err *ValidationError) ErrorStatus() int { return http.StatusBadRequest }

// ErrorCode returns the standard error code.
func (err *ValidationError) ErrorCode() string { return "validation_error" }

// ErrorMessage returns the public error message.
func (err *ValidationError) ErrorMessage() string { return err.Error() }

// ErrorParams returns translation/rendering parameters for the first field error.
func (err *ValidationError) ErrorParams() core.Map {
	if err == nil || len(err.Errors) == 0 {
		return nil
	}
	return err.Errors[0].Params
}

// ErrorFields returns validation field errors for renderers.
func (err *ValidationError) ErrorFields() []FieldError {
	if err == nil {
		return nil
	}
	return append([]FieldError(nil), err.Errors...)
}

// Invalid returns a validation error.
func Invalid(items ...FieldError) error { return &ValidationError{Errors: items} }

// AsError returns a ValidationError when err contains one.
func AsError(err error) *ValidationError {
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return validationErr
	}
	return nil
}
