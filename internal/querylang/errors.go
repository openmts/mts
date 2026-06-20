package querylang

import (
	"errors"
	"fmt"
)

type Code string

const (
	ErrInvalidMeasurement Code = "invalid-measurement"
	ErrInvalidTimeRange   Code = "invalid-time-range"
	ErrInvalidPagination  Code = "invalid-pagination"
	ErrSyntax             Code = "syntax"
)

type Error struct {
	Code     Code
	Message  string
	Position int
}

func (e Error) Error() string {
	if e.Position > 0 {
		return fmt.Sprintf("querylang %s at %d: %s", e.Code, e.Position, e.Message)
	}
	return fmt.Sprintf("querylang %s: %s", e.Code, e.Message)
}

func (e Error) Is(target error) bool {
	code, ok := target.(Code)
	return ok && e.Code == code
}

func (c Code) Error() string {
	return string(c)
}

func IsCode(err error, code Code) bool {
	var queryErr Error
	if errors.As(err, &queryErr) {
		return queryErr.Code == code
	}
	return false
}

func newError(code Code, message string) error {
	return Error{Code: code, Message: message}
}
