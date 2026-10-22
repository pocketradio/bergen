package domain

import (
	"fmt"
	"slices"
)

// JobState is the externally meaningful lifecycle state of a job.
type JobState string

const (
	JobCreated           JobState = "created"
	JobUploadPending     JobState = "upload_pending"
	JobUploaded          JobState = "uploaded"
	JobInspectionPending JobState = "inspection_pending"
	JobInspecting        JobState = "inspecting"
	JobTaskPending       JobState = "task_pending"
	JobPlanned           JobState = "planned"
	JobProcessing        JobState = "processing"
	JobPromotionPending  JobState = "promotion_pending"
	JobPromoting         JobState = "promoting"
	JobCompleted         JobState = "completed"
	JobRejected          JobState = "rejected"
	JobInspectionFailed  JobState = "inspection_failed"
	JobPromotionFailed   JobState = "promotion_failed"
	JobFailed            JobState = "failed"
	JobCancelled         JobState = "cancelled"
)

// TaskState is the lifecycle state of a schedulable task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskLeased     TaskState = "leased"
	TaskValidating TaskState = "validating"
	TaskAccepted   TaskState = "accepted"
	TaskRetryable  TaskState = "retryable"
	TaskFailed     TaskState = "failed"

	// TaskClaimed is retained as a source-compatible alias for TaskLeased.
	TaskClaimed = TaskLeased
)

// AttemptState describes one worker execution and its validation outcome.
type AttemptState string

const (
	AttemptLeased     AttemptState = "leased"
	AttemptSubmitted  AttemptState = "submitted"
	AttemptValidating AttemptState = "validating"
	AttemptAccepted   AttemptState = "accepted"
	AttemptRejected   AttemptState = "rejected"
	AttemptExpired    AttemptState = "expired"
	AttemptStale      AttemptState = "stale"

	// AttemptActive is retained as a source-compatible alias for AttemptLeased.
	AttemptActive = AttemptLeased
)

// ResultState describes validation and authority for submitted result metadata.
type ResultState string

const (
	ResultSubmitted  ResultState = "submitted"
	ResultValidating ResultState = "validating"
	ResultAccepted   ResultState = "accepted"
	ResultRejected   ResultState = "rejected"
)

// ObjectState describes the lifecycle of a server-owned object record.
type ObjectState string

const (
	ObjectReserved         ObjectState = "reserved"
	ObjectUploaded         ObjectState = "uploaded"
	ObjectVerified         ObjectState = "verified"
	ObjectPromotionPending ObjectState = "promotion_pending"
	ObjectPromoted         ObjectState = "promoted"
	ObjectDeletionPending  ObjectState = "deletion_pending"
	ObjectDeleted          ObjectState = "deleted"
	ObjectDeletionFailed   ObjectState = "deletion_failed"
)

// ValidateJobTransition reports whether a job may move between two states.
func ValidateJobTransition(from, to JobState) error {
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
	return validateTransition("job", from, to, allowed)
}

// ValidateTaskTransition reports whether a task may move between two states.
func ValidateTaskTransition(from, to TaskState) error {
	allowed := map[TaskState][]TaskState{
		TaskPending:    {TaskLeased, TaskFailed},
		TaskLeased:     {TaskValidating, TaskRetryable, TaskFailed},
		TaskValidating: {TaskAccepted, TaskRetryable, TaskFailed},
		TaskRetryable:  {TaskLeased, TaskFailed},
	}
	return validateTransition("task", from, to, allowed)
}

// ValidateAttemptTransition reports whether an attempt may move between lease and result states.
func ValidateAttemptTransition(from, to AttemptState) error {
	allowed := map[AttemptState][]AttemptState{
		AttemptLeased:     {AttemptSubmitted, AttemptRejected, AttemptExpired, AttemptStale},
		AttemptSubmitted:  {AttemptValidating, AttemptRejected, AttemptStale},
		AttemptValidating: {AttemptAccepted, AttemptRejected, AttemptStale},
	}
	return validateTransition("attempt", from, to, allowed)
}

// ValidateResultTransition reports whether submitted result metadata may change validation state.
func ValidateResultTransition(from, to ResultState) error {
	allowed := map[ResultState][]ResultState{
		ResultSubmitted:  {ResultValidating, ResultRejected},
		ResultValidating: {ResultAccepted, ResultRejected},
	}
	return validateTransition("result", from, to, allowed)
}

// ValidateObjectTransition reports whether an object lifecycle record may move between states.
func ValidateObjectTransition(from, to ObjectState) error {
	allowed := map[ObjectState][]ObjectState{
		ObjectReserved:         {ObjectUploaded, ObjectDeletionPending},
		ObjectUploaded:         {ObjectVerified, ObjectDeletionPending},
		ObjectVerified:         {ObjectPromotionPending, ObjectDeletionPending},
		ObjectPromotionPending: {ObjectPromoted, ObjectDeletionPending},
		ObjectPromoted:         {ObjectDeletionPending},
		ObjectDeletionPending:  {ObjectDeleted, ObjectDeletionFailed},
		ObjectDeletionFailed:   {ObjectDeletionPending},
	}
	return validateTransition("object", from, to, allowed)
}

// validateTransition checks a lifecycle transition and treats states without outgoing edges as terminal.
func validateTransition[T ~string](kind string, from, to T, allowed map[T][]T) error {
	// check if the next state is an allowed state
	if slices.Contains(allowed[from], to) {
		return nil
	}

	return fmt.Errorf("invalid %s transition %q to %q", kind, from, to)
}
