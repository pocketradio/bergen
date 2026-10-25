package protocol

import (
	"errors"
	"net/http"
	"strings"
)

// ErrorCode is a stable, machine-readable API failure classification.
type ErrorCode string

const (
	ErrorInvalidRequest ErrorCode = "invalid_request"
	ErrorUnauthorized   ErrorCode = "unauthorized"
	ErrorForbidden      ErrorCode = "forbidden"
	ErrorNotFound       ErrorCode = "not_found"
	ErrorConflict       ErrorCode = "conflict"
	ErrorStaleAttempt   ErrorCode = "stale_attempt"
	ErrorLeaseExpired   ErrorCode = "lease_expired"
	ErrorInternal       ErrorCode = "internal"
)

// ErrorResponse is the stable, secret-safe API error shape.
type ErrorResponse struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// IsKnown reports whether code is a documented public error code.
func (code ErrorCode) IsKnown() bool {
	switch code {
	case ErrorInvalidRequest, ErrorUnauthorized, ErrorForbidden, ErrorNotFound, ErrorConflict, ErrorStaleAttempt, ErrorLeaseExpired, ErrorInternal:
		return true
	default:
		return false
	}
}

// ValidateErrorResponse validates a documented public error response.
func ValidateErrorResponse(response ErrorResponse) error {
	if !response.Code.IsKnown() {
		return errors.New("error response requires a documented error code")
	}
	if strings.TrimSpace(response.Message) == "" {
		return errors.New("error response requires message")
	}
	return nil
}

// HTTPStatus returns the fixed HTTP status for a public error code.
func (code ErrorCode) HTTPStatus() int {
	// mapping our custom errors to http status codes
	switch code {
	case ErrorInvalidRequest:
		return http.StatusBadRequest
	case ErrorUnauthorized:
		return http.StatusUnauthorized
	case ErrorForbidden:
		return http.StatusForbidden
	case ErrorNotFound:
		return http.StatusNotFound
	case ErrorConflict, ErrorStaleAttempt, ErrorLeaseExpired:
		return http.StatusConflict
	case ErrorInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
