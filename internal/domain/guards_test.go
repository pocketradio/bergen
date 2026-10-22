package domain

import (
	"testing"
	"time"
)

// TestValidationLeaseGuards verifies coordinator lease ownership and expiry are required for validation.
func TestValidationLeaseGuards(t *testing.T) {
	// setting up dummy validation attempt data
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	result := testResult()
	result.State = ResultValidating
	result.Object.Version = "version-1"
	guard := ValidationAttemptGuard{
		CurrentAttemptID: "attempt-1",
		AttemptID:        "attempt-1",
		CoordinatorID:    "coordinator-1",
		ValidationLease:  ValidationLease{OwnerID: "coordinator-1", ExpiresAt: now.Add(time.Minute)},
		Result:           result,
	}

	// Running our validations
	if err := ValidateValidationAttemptGuard(guard, now); err != nil {
		t.Fatal(err)
	}

	// try changing the validation coordinator to an invalid one
	guard.ValidationLease.OwnerID = "coordinator-2"
	if err := ValidateValidationAttemptGuard(guard, now); err == nil {
		t.Fatal("expected wrong coordinator to be rejected")
	}

	// reverting to valid coordinator
	// explicitly setting lease expiry to current time to test rejection
	guard.ValidationLease.OwnerID = "coordinator-1"
	guard.ValidationLease.ExpiresAt = now
	if err := ValidateValidationAttemptGuard(guard, now); err == nil {
		t.Fatal("expected expired validation lease to be rejected")
	}

	guard.ValidationLease.ExpiresAt = now.Add(time.Minute)
	guard.Result.AttemptID = "attempt-2"
	if err := ValidateValidationAttemptGuard(guard, now); err == nil {
		t.Fatal("expected result for another attempt to be rejected")
	}
}

// TestWorkerAttemptGuard verifies worker transitions require the current assigned leased attempt.
func TestWorkerAttemptGuard(t *testing.T) {
	// setting up dummy worker attempt data
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	guard := WorkerAttemptGuard{
		CurrentAttemptID: "attempt-1", AttemptID: "attempt-1", TaskID: "task-1", AttemptState: AttemptLeased,
		AttemptWorkerID: "worker-1", AuthenticatedWorkerID: "worker-1", LeaseExpiresAt: now.Add(time.Minute),
	}

	// testing the transitions with respect to current time
	if err := ValidateWorkerAttemptGuard(guard, now); err != nil {
		t.Fatal(err)
	}

	// manually incrementing currentAttempt to make the worker's attemptId stale
	guard.CurrentAttemptID = "attempt-2"
	if err := ValidateWorkerAttemptGuard(guard, now); err == nil {
		t.Fatal("expected stale attempt to be rejected")
	}

	// testing that a submitted attempt is rejected
	guard.CurrentAttemptID = "attempt-1"
	guard.AttemptState = AttemptSubmitted
	if err := ValidateWorkerAttemptGuard(guard, now); err == nil {
		t.Fatal("expected submitted attempt to be rejected")
	}
}

// TestSubmissionAttemptGuard verifies a worker may submit only its assigned task-output key.
func TestSubmissionAttemptGuard(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	guard := SubmissionAttemptGuard{
		WorkerGuard: WorkerAttemptGuard{
			CurrentAttemptID: "attempt-1", AttemptID: "attempt-1", TaskID: "task-1", AttemptState: AttemptLeased,
			AttemptWorkerID: "worker-1", AuthenticatedWorkerID: "worker-1", LeaseExpiresAt: now.Add(time.Minute),
		},
		AssignedOutput:     ObjectRef{Purpose: ObjectTaskOutput, Bucket: "bergen", Key: "tasks/task-1/attempt-1/out.mp4", JobID: "job-1", TaskID: "task-1", AttemptID: "attempt-1"},
		SubmittedObjectKey: "tasks/task-1/attempt-1/out.mp4",
	}
	if err := ValidateSubmissionAttemptGuard(guard, now); err != nil {
		t.Fatal(err)
	}

	// checking that submitted key matches the assigned key
	guard.SubmittedObjectKey = "tasks/task-1/attempt-1/other.mp4"
	if err := ValidateSubmissionAttemptGuard(guard, now); err == nil {
		t.Fatal("expected unassigned output key to be rejected")
	}
}

// TestValidationRecoveryGuard verifies expired validation work is fenced before it can be retried.
func TestValidationRecoveryGuard(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	guard := ValidationRecoveryGuard{
		CurrentAttemptID: "attempt-1", AttemptID: "attempt-1", AttemptState: AttemptValidating,
		ValidationLease: ValidationLease{OwnerID: "coordinator-1", ExpiresAt: now}, RecoveryCoordinator: "coordinator-2",
		FencedAttemptState: AttemptStale, Retry: RetryTransitionGuard{Class: RetryValidationLeaseExpired, NextState: TaskRetryable},
	}
	if err := ValidateValidationRecoveryGuard(guard, now); err != nil {
		t.Fatal(err)
	}

	// the current attempt can only be declared stale, not rejected before retrying
	guard.FencedAttemptState = AttemptRejected
	if err := ValidateValidationRecoveryGuard(guard, now); err == nil {
		t.Fatal("expected unfenced validation recovery to be rejected")
	}
}

// TestRetryTransitionGuard verifies classifications cannot select an incompatible task outcome.
func TestRetryTransitionGuard(t *testing.T) {
	if err := ValidateRetryTransitionGuard(RetryTransitionGuard{Class: RetryValidationTimeout, NextState: TaskRetryable}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRetryTransitionGuard(RetryTransitionGuard{Class: RetryInvalidMedia, NextState: TaskRetryable}); err == nil {
		t.Fatal("expected terminal retry class to reject a retryable task state")
	}
}
