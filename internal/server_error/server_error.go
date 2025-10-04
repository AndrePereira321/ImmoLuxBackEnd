package server_error

import (
	"errors"
	"fmt"
)

type ServerError struct {
	Code    string
	Message string
	Cause   error
}

func New(code, message string) *ServerError {
	return newServerError(code, message, nil)
}

func Wrap(code, message string, cause error) *ServerError {
	return newServerError(code, message, cause)
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

func (e *ServerError) String() string {
	return e.Error()
}

func IsServerError(err error, code string) bool {
	var serverError *ServerError
	return errors.As(err, &serverError) && serverError.Code == code
}

func newServerError(code string, message string, cause error) *ServerError {
	return &ServerError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
