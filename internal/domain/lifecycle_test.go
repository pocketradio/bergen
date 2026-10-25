package domain

import "testing"

// TestJobTransitions verifies every documented job transition and terminal state.
func TestJobTransitions(t *testing.T) {
	allowed := map[JobState][]JobState{
		JobCreated:           {JobUploadPending, JobRejected, JobCancelled},
		JobUploadPending:     {JobUploaded, JobRejected, JobCancelled},
		JobUploaded:          {JobInspectionPending, JobRejected, JobCancelled},
		JobInspectionPending: {JobInspecting, JobRejected, JobInspectionFailed},
		JobInspecting:        {JobTaskPending, JobRejected, JobInspectionFailed},
		JobTaskPending:       {JobPlanned, JobFailed},
		JobPlanned:           {JobProcessing, JobFailed, JobCancelled},
		JobProcessing:        {JobPromotionPending, JobFailed, JobCancelled},
		JobPromotionPending:  {JobPromoting, JobPromotionFailed},
		JobPromoting:         {JobCompleted, JobPromotionFailed},
	}
	assertTransitions(t, "job", allowed, ValidateJobTransition, []JobState{JobRejected, JobInspectionFailed, JobPromotionFailed, JobFailed, JobCancelled, JobCompleted})
}

// TestTaskTransitions verifies every documented task transition and terminal state.
func TestTaskTransitions(t *testing.T) {
	allowed := map[TaskState][]TaskState{
		TaskPending:    {TaskLeased, TaskFailed},
		TaskLeased:     {TaskValidating, TaskRetryable, TaskFailed},
		TaskValidating: {TaskAccepted, TaskRetryable, TaskFailed},
		TaskRetryable:  {TaskLeased, TaskFailed},
	}
	assertTransitions(t, "task", allowed, ValidateTaskTransition, []TaskState{TaskAccepted, TaskFailed})
}

// TestAttemptTransitions verifies every documented attempt transition and terminal state.
func TestAttemptTransitions(t *testing.T) {
	allowed := map[AttemptState][]AttemptState{
		AttemptLeased:     {AttemptSubmitted, AttemptRejected, AttemptExpired, AttemptStale},
		AttemptSubmitted:  {AttemptValidating, AttemptRejected, AttemptStale},
		AttemptValidating: {AttemptAccepted, AttemptRejected, AttemptStale},
	}
	assertTransitions(t, "attempt", allowed, ValidateAttemptTransition, []AttemptState{AttemptAccepted, AttemptRejected, AttemptExpired, AttemptStale})
}

// TestResultTransitions verifies every documented result transition and terminal state.
func TestResultTransitions(t *testing.T) {
	allowed := map[ResultState][]ResultState{
		ResultSubmitted:  {ResultValidating, ResultRejected},
		ResultValidating: {ResultAccepted, ResultRejected},
	}
	assertTransitions(t, "result", allowed, ValidateResultTransition, []ResultState{ResultAccepted, ResultRejected})
}

// TestObjectTransitions verifies every documented object transition and terminal state.
func TestObjectTransitions(t *testing.T) {
	allowed := map[ObjectState][]ObjectState{
		ObjectReserved:         {ObjectUploaded, ObjectDeletionPending},
		ObjectUploaded:         {ObjectVerified, ObjectDeletionPending},
		ObjectVerified:         {ObjectPromotionPending, ObjectDeletionPending},
		ObjectPromotionPending: {ObjectPromoted, ObjectDeletionPending},
		ObjectPromoted:         {ObjectDeletionPending},
		ObjectDeletionPending:  {ObjectDeleted, ObjectDeletionFailed},
		ObjectDeletionFailed:   {ObjectDeletionPending},
	}
	assertTransitions(t, "object", allowed, ValidateObjectTransition, []ObjectState{ObjectDeleted})
}

// assertTransitions checks every declared transition and verifies terminal states have no outgoing transitions.
func assertTransitions[T ~string](t *testing.T, kind string, allowed map[T][]T, validate func(T, T) error, terminal []T) {
	t.Helper()

	// iterating over the set of all state transitions
	// and checking for invalid transitions
	for from, destinations := range allowed {
		for _, to := range destinations {
			if err := validate(from, to); err != nil {
				t.Errorf("%s transition %q to %q rejected: %v", kind, from, to, err)
			}
		}
	}

	// checking the terminal states have no further transitions
	for _, state := range terminal {
		if err := validate(state, state); err == nil {
			t.Errorf("terminal %s state %q accepted an outgoing transition", kind, state)
		}
	}
}
