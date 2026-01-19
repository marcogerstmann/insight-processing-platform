package apperr

import (
	"errors"
)

var (
	ErrUnauthorized        = errors.New("unauthorized")
	ErrServerMisconfigured = errors.New("server_misconfigured")
	ErrInvalidPayload      = errors.New("invalid_payload")
)

type Error struct {
	kind error
	err  error
}

func (e *Error) Unwrap() error { return e.err }

func (e *Error) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	if e.kind != nil {
		return e.kind.Error()
	}
	return "unknown error"
}

func (e *Error) Is(target error) bool {
	return target == e.kind
}

func E(kind error, detail error) error {
	return &Error{kind: kind, err: detail}
}
