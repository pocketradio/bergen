package protocol

import (
	"net/http"
	"testing"
)

// TestErrorCodeStatus verifies public protocol errors keep their documented HTTP mappings.
func TestErrorCodeStatus(t *testing.T) {
	tests := []struct {
		code ErrorCode
		want int
	}{
		{code: ErrorInvalidRequest, want: http.StatusBadRequest},
		{code: ErrorUnauthorized, want: http.StatusUnauthorized},
		{code: ErrorForbidden, want: http.StatusForbidden},
		{code: ErrorNotFound, want: http.StatusNotFound},
		{code: ErrorConflict, want: http.StatusConflict},
		{code: ErrorStaleAttempt, want: http.StatusConflict},
		{code: ErrorLeaseExpired, want: http.StatusConflict},
		{code: ErrorInternal, want: http.StatusInternalServerError},
	}

	// iterating over dummy error codes to
	// test that custom errors map to correct http status codes
	for _, test := range tests {
		if got := test.code.HTTPStatus(); got != test.want {
			t.Errorf("error code %q status = %d, want %d", test.code, got, test.want)
		}
	}
}

// TestErrorResponseValidation verifies only documented, nonempty public errors are emitted.
func TestErrorResponseValidation(t *testing.T) {
	if err := ValidateErrorResponse(ErrorResponse{Code: ErrorConflict, Message: "idempotency key conflicts"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateErrorResponse(ErrorResponse{Code: "unknown", Message: "unexpected"}); err == nil {
		t.Fatal("expected undocumented error code to be rejected")
	}
}
