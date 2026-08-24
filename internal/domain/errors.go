package domain

import "fmt"

type ErrorCode string

const (
	CodeInvalid   ErrorCode = "invalid"
	CodeNotFound  ErrorCode = "not_found"
	CodeConflict  ErrorCode = "conflict"
	CodeForbidden ErrorCode = "forbidden"
)

type RuleError struct {
	Code    ErrorCode
	Message string
}

func (e *RuleError) Error() string { return e.Message }

func NewRuleError(code ErrorCode, format string, args ...any) error {
	return &RuleError{Code: code, Message: fmt.Sprintf(format, args...)}
}
