package queryanalyzer

import (
	"errors"
	"fmt"
)

type Code string

const (
	ErrMeasurementNotFound  Code = "measurement-not-found"
	ErrFieldNotFound        Code = "field-not-found"
	ErrUnsupportedFunction  Code = "unsupported-function"
	ErrFunctionTypeMismatch Code = "function-type-mismatch"
	ErrInvalidWindow        Code = "invalid-window"
	ErrInvalidPagination    Code = "invalid-pagination"
	ErrInvalidGroup         Code = "invalid-group"
)

type Error struct {
	Code    Code
	Message string
}

func (e Error) Error() string {
	return fmt.Sprintf("queryanalyzer %s: %s", e.Code, e.Message)
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
