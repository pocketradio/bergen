package domain

import "fmt"

// RetryClass identifies the cause of a task retry or terminal failure.
type RetryClass string

const (
	RetryExecutionLeaseExpired   RetryClass = "execution_lease_expired"
	RetryValidationLeaseExpired  RetryClass = "validation_lease_expired"
	RetryValidationTimeout       RetryClass = "validation_timeout"
	RetryStorageTransient        RetryClass = "storage_transient"
	RetryInfrastructureTransient RetryClass = "infrastructure_transient"
	RetryMalformedSubmission     RetryClass = "malformed_submission"
	RetryObjectIntegrityFailure  RetryClass = "object_integrity_failure"
	RetryInvalidMedia            RetryClass = "invalid_media"
)

// RetryDisposition states whether a classified failure can consume a bounded retry.
type RetryDisposition string

const (
	RetryAllowed  RetryDisposition = "retry_allowed"
	RetryTerminal RetryDisposition = "terminal"
)

// RetryDispositionFor returns the contract-defined retry behavior without choosing a retry limit.
func RetryDispositionFor(class RetryClass) (RetryDisposition, error) {
	switch class {
	case RetryExecutionLeaseExpired, RetryValidationLeaseExpired, RetryValidationTimeout, RetryStorageTransient, RetryInfrastructureTransient:
		return RetryAllowed, nil
	case RetryMalformedSubmission, RetryObjectIntegrityFailure, RetryInvalidMedia:
		return RetryTerminal, nil
	default:
		return "", fmt.Errorf("unknown retry class %q", class)
	}
}
