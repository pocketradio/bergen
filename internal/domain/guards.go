package domain

import (
	"errors"
	"fmt"
	"time"
)

// WorkerAttemptGuard contains the values a worker-side transition must compare atomically.
type WorkerAttemptGuard struct {
	CurrentAttemptID      AttemptID
	AttemptID             AttemptID
	TaskID                TaskID
	AttemptState          AttemptState
	AttemptWorkerID       WorkerID
	AuthenticatedWorkerID WorkerID
	LeaseExpiresAt        time.Time
}

// SubmissionAttemptGuard contains the assigned output and submitted key a result submission must compare atomically.
type SubmissionAttemptGuard struct {
	WorkerGuard        WorkerAttemptGuard
	AssignedOutput     ObjectRef
	SubmittedObjectKey string
}

// ValidationAttemptGuard contains the values a coordinator acceptance transition must compare atomically.
type ValidationAttemptGuard struct {
	CurrentAttemptID AttemptID
	AttemptID        AttemptID
	CoordinatorID    CoordinatorID
	ValidationLease  ValidationLease
	AcceptedResultID *ResultID
	Result           Result
}

// RetryTransitionGuard contains the classified failure and resulting task state.
type RetryTransitionGuard struct {
	Class     RetryClass
	NextState TaskState
}

// ValidationRecoveryGuard contains the values required to fence an expired validating attempt before retry.
type ValidationRecoveryGuard struct {
	CurrentAttemptID    AttemptID
	AttemptID           AttemptID
	AttemptState        AttemptState
	ValidationLease     ValidationLease
	RecoveryCoordinator CoordinatorID
	FencedAttemptState  AttemptState
	Retry               RetryTransitionGuard
}

// ValidateValidationLease validates the coordinator ownership and finite duration of a validation lease.
func ValidateValidationLease(lease ValidationLease) error {
	if lease.OwnerID == "" || lease.ExpiresAt.IsZero() {
		return errors.New("validation lease requires coordinator owner and expiry")
	}
	return nil
}

// ValidateWorkerAttemptGuard validates a worker transition against the current worker lease at now.
func ValidateWorkerAttemptGuard(guard WorkerAttemptGuard, now time.Time) error {
	if guard.CurrentAttemptID == "" || guard.AttemptID == "" || guard.CurrentAttemptID != guard.AttemptID {
		return errors.New("worker transition requires the current attempt")
	}
	if guard.TaskID == "" || guard.AttemptState != AttemptLeased {
		return errors.New("worker transition requires a leased task attempt")
	}
	if guard.AttemptWorkerID == "" || guard.AuthenticatedWorkerID == "" || guard.AttemptWorkerID != guard.AuthenticatedWorkerID {
		return errors.New("worker transition requires the assigned authenticated worker")
	}
	if guard.LeaseExpiresAt.IsZero() || !guard.LeaseExpiresAt.After(now) {
		return errors.New("worker transition requires an unexpired lease")
	}
	return nil
}

// ValidateSubmissionAttemptGuard validates that a current leased attempt submits only its assigned output key.
func ValidateSubmissionAttemptGuard(guard SubmissionAttemptGuard, now time.Time) error {
	if err := ValidateWorkerAttemptGuard(guard.WorkerGuard, now); err != nil {
		return err
	}
	if err := ValidateObjectRef(guard.AssignedOutput); err != nil {
		return fmt.Errorf("assigned output: %w", err)
	}
	if guard.AssignedOutput.Purpose != ObjectTaskOutput || guard.AssignedOutput.TaskID != guard.WorkerGuard.TaskID || guard.AssignedOutput.AttemptID != guard.WorkerGuard.AttemptID {
		return errors.New("assigned output must belong to the current task attempt")
	}
	if guard.SubmittedObjectKey != guard.AssignedOutput.Key {
		return errors.New("submitted object key must match assigned output key")
	}
	return nil
}

// ValidateValidationAttemptGuard validates coordinator acceptance inputs at now before the persistence compare-and-set.
func ValidateValidationAttemptGuard(guard ValidationAttemptGuard, now time.Time) error {
	if guard.CurrentAttemptID == "" || guard.AttemptID == "" || guard.CurrentAttemptID != guard.AttemptID {
		return errors.New("validation transition requires the current attempt")
	}
	if err := ValidateValidationLease(guard.ValidationLease); err != nil {
		return err
	}
	if guard.CoordinatorID == "" || guard.ValidationLease.OwnerID != guard.CoordinatorID {
		return errors.New("validation transition requires the lease-owning coordinator")
	}
	if !guard.ValidationLease.ExpiresAt.After(now) {
		return errors.New("validation transition requires an unexpired validation lease")
	}
	if guard.AcceptedResultID != nil {
		return errors.New("validation transition requires no accepted task result")
	}
	if guard.Result.AttemptID != guard.AttemptID || guard.Result.State != ResultValidating {
		return errors.New("validation transition requires the guarded validating result")
	}
	if err := ValidateResultMetadata(guard.Result); err != nil {
		return fmt.Errorf("validate result: %w", err)
	}
	if err := ValidateBoundObjectRef(guard.Result.Object); err != nil {
		return fmt.Errorf("validate result object: %w", err)
	}
	return nil
}

// ValidateValidationRecoveryGuard validates that an expired validation lease fences its current attempt before retry.
func ValidateValidationRecoveryGuard(guard ValidationRecoveryGuard, now time.Time) error {
	if guard.CurrentAttemptID == "" || guard.AttemptID == "" || guard.CurrentAttemptID != guard.AttemptID {
		return errors.New("validation recovery requires the current attempt")
	}
	if guard.AttemptState != AttemptValidating || guard.FencedAttemptState != AttemptStale {
		return errors.New("validation recovery must fence a validating attempt as stale")
	}
	if guard.RecoveryCoordinator == "" {
		return errors.New("validation recovery requires coordinator identity")
	}
	if err := ValidateValidationLease(guard.ValidationLease); err != nil {
		return err
	}
	if guard.ValidationLease.ExpiresAt.After(now) {
		return errors.New("validation recovery requires an expired validation lease")
	}
	if guard.Retry.Class != RetryValidationLeaseExpired {
		return errors.New("validation recovery requires validation lease expiry classification")
	}
	return ValidateRetryTransitionGuard(guard.Retry)
}

// ValidateRetryTransitionGuard validates that the retry classification selects its matching task outcome.
func ValidateRetryTransitionGuard(guard RetryTransitionGuard) error {
	disposition, err := RetryDispositionFor(guard.Class)
	if err != nil {
		return err
	}
	if disposition == RetryAllowed && guard.NextState == TaskRetryable {
		return nil
	}
	if disposition == RetryTerminal && guard.NextState == TaskFailed {
		return nil
	}
	return fmt.Errorf("retry class %q cannot transition task to %q", guard.Class, guard.NextState)
}
