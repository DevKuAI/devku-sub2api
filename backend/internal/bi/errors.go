package bi

import (
	"errors"
	"net/http"
)

type FieldError struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type Error struct {
	Status    int          `json:"-"`
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id"`
	Retryable bool         `json:"retryable"`
	Details   []FieldError `json:"details"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func apiError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message, Retryable: status >= 500, Details: []FieldError{}}
}

var (
	ErrUnauthenticated = apiError(http.StatusUnauthorized, "UNAUTHENTICATED", "Session is invalid or expired")
	ErrForbidden       = apiError(http.StatusForbidden, "FORBIDDEN", "Permission denied")
	ErrNotFound        = apiError(http.StatusNotFound, "NOT_FOUND", "Resource not found")
	ErrBindingConflict = apiError(http.StatusConflict, "BINDING_CONFLICT", "The WeChat identity is already bound")
	ErrBindingExpired  = apiError(http.StatusGone, "BINDING_EXPIRED", "Binding challenge is expired or revoked")
	ErrPendingBinding  = errors.New("binding awaits account approval")
)

func invalid(field, reason string) *Error {
	err := apiError(http.StatusBadRequest, "INVALID_ARGUMENT", "Invalid request")
	err.Details = []FieldError{{Field: field, Reason: reason}}
	return err
}
