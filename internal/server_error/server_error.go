package server_error

import (
	"errors"
	"fmt"
	"immo-lux/internal/models"
)

// Kind classifies a ServerError for transport-level handling. The HTTP adapter
// in internal/server maps each kind to a status code; the rest of the backend
// deals in kinds, never in status codes.
type Kind int

const (
	KindInternal     Kind = iota // unexpected failure
	KindInvalid                  // input rejected
	KindUnauthorized             // missing or bad credentials
	KindForbidden                // authenticated but not allowed
	KindNotFound                 // target does not exist
	KindRateLimited              // throttled
)

type ServerError struct {
	Kind    Kind
	Code    string
	Message string
	Cause   error
}

func New(code, message string) *ServerError {
	return newServerError(KindInternal, code, message, nil)
}

func Wrap(code, message string, cause error) *ServerError {
	return newServerError(KindInternal, code, message, cause)
}

func Invalid(code, message string) *ServerError {
	return newServerError(KindInvalid, code, message, nil)
}

// BadRequest is the Invalid error for malformed requests at the HTTP edge; it
// keeps the BAD_REQUEST wire code defined in one place.
func BadRequest(message string) *ServerError {
	return Invalid("BAD_REQUEST", message)
}

func Unauthorized(code, message string) *ServerError {
	return newServerError(KindUnauthorized, code, message, nil)
}

func Forbidden(code, message string) *ServerError {
	return newServerError(KindForbidden, code, message, nil)
}

func NotFound(code, message string) *ServerError {
	return newServerError(KindNotFound, code, message, nil)
}

func RateLimited(code, message string) *ServerError {
	return newServerError(KindRateLimited, code, message, nil)
}

// WithCause attaches an underlying error without changing kind, code or message.
func (e *ServerError) WithCause(cause error) *ServerError {
	e.Cause = cause
	return e
}

func (e *ServerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s - Caused by: \n\t-%v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *ServerError) Unwrap() error {
	return e.Cause
}

func (e *ServerError) Is(target error) bool {
	var other *ServerError
	if errors.As(target, &other) {
		return e.Code == other.Code && e.Message == other.Message && errors.Is(e.Cause, other.Cause)
	}
	return false
}

func (e *ServerError) ToServerAPIError() *models.ServerAPIError {
	return &models.ServerAPIError{
		Code:    e.Code,
		Message: e.Message,
	}
}

func (e *ServerError) String() string {
	return e.Error()
}

func IsServerError(err error, code string) bool {
	var serverError *ServerError
	return errors.As(err, &serverError) && serverError.Code == code
}

func newServerError(kind Kind, code string, message string, cause error) *ServerError {
	return &ServerError{
		Kind:    kind,
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
