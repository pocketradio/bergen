package domain

import "testing"

// TestRetryDisposition verifies retry classification does not select an unresolved retry limit.
func TestRetryDisposition(t *testing.T) {
	tests := []struct {
		class RetryClass
		want  RetryDisposition
	}{
		{class: RetryExecutionLeaseExpired, want: RetryAllowed},
		{class: RetryValidationTimeout, want: RetryAllowed},
		{class: RetryInvalidMedia, want: RetryTerminal},
		{class: RetryObjectIntegrityFailure, want: RetryTerminal},
	}
	// check that only specific retry classes are allowed to retry
	for _, test := range tests {
		got, err := RetryDispositionFor(test.class)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("retry class %q disposition = %q, want %q", test.class, got, test.want)
		}
	}
}
