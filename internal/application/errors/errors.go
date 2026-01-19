package errors

import (
	"errors"
	"fmt"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrServerMisconfigured = errors.New("server_misconfigured")
)

type WrappedError struct {
	Err error
	Msg string
}

func (e *WrappedError) Error() string {
	if e.Msg != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s: %v", e.Msg, e.Err)
		}
		return e.Msg
	}
	return fmt.Sprintf("%v", e.Err)
}

func (e *WrappedError) Unwrap() error {
	return e.Err
}

func NewError(err error, msg string) error {
	return &WrappedError{Err: err, Msg: msg}
}
