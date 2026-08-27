package domain

import "fmt"

type ErrorCode string

const (
	CodeValidation       ErrorCode = "validation_error"
	CodeNotFound         ErrorCode = "not_found"
	CodeConflict         ErrorCode = "version_conflict"
	CodeInvalidState     ErrorCode = "invalid_state"
	CodeDuplicateRelease ErrorCode = "duplicate_release"
	CodeUnresolved       ErrorCode = "unresolved_disputes"
)

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Field   string    `json:"field,omitempty"`
}

func (e *Error) Error() string { return e.Message }

func NewError(code ErrorCode, message, field string) error {
	return &Error{Code: code, Message: message, Field: field}
}

func Required(field string) error {
	return NewError(CodeValidation, fmt.Sprintf("%s 不能为空", field), field)
}

func Invalid(field, reason string) error {
	return NewError(CodeValidation, fmt.Sprintf("%s %s", field, reason), field)
}

func InvalidState(current SurveyStatus, operation string) error {
	return NewError(CodeInvalidState, fmt.Sprintf("调查处于 %s 状态，不能执行%s", current, operation), "")
}
