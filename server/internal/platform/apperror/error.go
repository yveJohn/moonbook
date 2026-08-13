package apperror

import (
	"errors"
	"net/http"
)

type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeRateLimited     Code = "RATE_LIMITED"
	CodeUnavailable     Code = "UNAVAILABLE"
	CodeInternal        Code = "INTERNAL"
)

type Error struct {
	code          Code
	httpStatus    int
	publicMessage string
	cause         error
}

type Public struct {
	Code       Code
	HTTPStatus int
	Message    string
}

func New(code Code, httpStatus int, publicMessage string) *Error {
	return &Error{code: code, httpStatus: httpStatus, publicMessage: publicMessage}
}

func Wrap(cause error, code Code, httpStatus int, publicMessage string) *Error {
	return &Error{code: code, httpStatus: httpStatus, publicMessage: publicMessage, cause: cause}
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.cause != nil {
		return err.cause.Error()
	}
	return err.publicMessage
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.cause
}

func (err *Error) Code() Code {
	return err.code
}

func (err *Error) HTTPStatus() int {
	return err.httpStatus
}

func (err *Error) PublicMessage() string {
	return err.publicMessage
}

func Expose(err error) Public {
	var appErr *Error
	if errors.As(err, &appErr) {
		return Public{Code: appErr.code, HTTPStatus: appErr.httpStatus, Message: appErr.publicMessage}
	}
	return Public{Code: CodeInternal, HTTPStatus: http.StatusInternalServerError, Message: "internal service error"}
}
