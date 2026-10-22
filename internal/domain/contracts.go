// Package domain contains the stable business contracts shared by the API,
// coordinator, workers, persistence adapters, and validation components.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ID is an opaque server-generated identifier represented as a string on the wire.
type ID string

// UserID identifies a client user.
type UserID ID

// WorkerID identifies a contributor worker.
type WorkerID ID

// JobID identifies a compression job.
type JobID ID

// TaskID identifies a unit of processing work.
type TaskID ID

// AttemptID identifies one fenced execution of a task.
type AttemptID ID

// ResultID identifies one submitted or accepted task result.
type ResultID ID

// SettlementKey identifies one idempotent credit settlement.
type SettlementKey string

// JobState is the externally meaningful lifecycle state of a job.
type JobState string

const (
	JobCreated       JobState = "created"
	JobUploadPending JobState = "upload_pending"
	JobUploaded      JobState = "uploaded"
	JobInspecting    JobState = "inspecting"
	JobPlanned       JobState = "planned"
	JobProcessing    JobState = "processing"
	JobAssembling    JobState = "assembling"
	JobValidating    JobState = "validating"
	JobCompleted     JobState = "completed"
	JobRejected      JobState = "rejected"
	JobFailed        JobState = "failed"
	JobCancelled     JobState = "cancelled"
)

// TaskState is the lifecycle state of a schedulable task.
type TaskState string

const (
	TaskPending    TaskState = "pending"
	TaskClaimed    TaskState = "claimed"
	TaskValidating TaskState = "validating"
	TaskAccepted   TaskState = "accepted"
	TaskRetryable  TaskState = "retryable"
	TaskFailed     TaskState = "failed"
)

// AttemptState describes one worker execution and its lease outcome.
type AttemptState string

const (
	AttemptActive    AttemptState = "active"
	AttemptSubmitted AttemptState = "submitted"
	AttemptAccepted  AttemptState = "accepted"
	AttemptRejected  AttemptState = "rejected"
	AttemptExpired   AttemptState = "expired"
	AttemptStale     AttemptState = "stale"
)

// ResultState describes validation and authority for a submitted result.
type ResultState string

const (
	ResultSubmitted  ResultState = "submitted"
	ResultValidating ResultState = "validating"
	ResultAccepted   ResultState = "accepted"
	ResultRejected   ResultState = "rejected"
)

// ProcessingMode identifies how a task consumes its input media.
type ProcessingMode string

const (
	ProcessingWholeFile ProcessingMode = "whole_file"
	ProcessingSegment   ProcessingMode = "segment"
)

// ObjectPurpose identifies why an object exists and who may reference it.
type ObjectPurpose string

const (
	ObjectInput       ObjectPurpose = "input"
	ObjectTaskOutput  ObjectPurpose = "task_output"
	ObjectFinalOutput ObjectPurpose = "final_output"
)

// EncodeProfile contains server-controlled media output settings.
type EncodeProfile struct {
	Container  string `json:"container"`
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
}

// MediaInfo contains metadata extracted from an input or output media object.
type MediaInfo struct {
	Duration     time.Duration `json:"duration"`
	Container    string        `json:"container"`
	VideoCodec   string        `json:"video_codec"`
	AudioCodec   string        `json:"audio_codec"`
	VideoStreams int           `json:"video_streams"`
	AudioStreams int           `json:"audio_streams"`
}

// ObjectRef identifies an object without granting access to it.
type ObjectRef struct {
	Purpose   ObjectPurpose `json:"purpose"`
	Bucket    string        `json:"bucket"`
	Key       string        `json:"key"`
	JobID     JobID         `json:"job_id"`
	TaskID    TaskID        `json:"task_id,omitempty"`
	AttemptID AttemptID     `json:"attempt_id,omitempty"`
}

// Job is the durable domain representation of a compression request.
type Job struct {
	ID          JobID         `json:"id"`
	OwnerID     UserID        `json:"owner_id"`
	State       JobState      `json:"state"`
	Profile     EncodeProfile `json:"profile"`
	Input       ObjectRef     `json:"input"`
	FinalOutput *ObjectRef    `json:"final_output,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Task is a durable unit of processing work belonging to a job.
type Task struct {
	ID             TaskID         `json:"id"`
	JobID          JobID          `json:"job_id"`
	State          TaskState      `json:"state"`
	Mode           ProcessingMode `json:"mode"`
	Profile        EncodeProfile  `json:"profile"`
	Input          ObjectRef      `json:"input"`
	AcceptedResult *ResultID      `json:"accepted_result_id,omitempty"`
	AttemptCount   int            `json:"attempt_count"`
}

// Attempt is one worker-owned, lease-fenced execution of a task.
type Attempt struct {
	ID             AttemptID    `json:"id"`
	TaskID         TaskID       `json:"task_id"`
	WorkerID       WorkerID     `json:"worker_id"`
	State          AttemptState `json:"state"`
	Output         ObjectRef    `json:"output"`
	LeaseExpiresAt time.Time    `json:"lease_expires_at"`
	CreatedAt      time.Time    `json:"created_at"`
}

// Result is metadata submitted for one attempt and does not itself authorize acceptance.
type Result struct {
	ID          ResultID    `json:"id"`
	AttemptID   AttemptID   `json:"attempt_id"`
	TaskID      TaskID      `json:"task_id"`
	WorkerID    WorkerID    `json:"worker_id"`
	Object      ObjectRef   `json:"object"`
	Size        int64       `json:"size"`
	SHA256      string      `json:"sha256"`
	Media       MediaInfo   `json:"media"`
	State       ResultState `json:"state"`
	SubmittedAt time.Time   `json:"submitted_at"`
}

// ValidateEncodeProfile validates the structural profile contract without deciding policy.
func ValidateEncodeProfile(profile EncodeProfile) error {
	if strings.TrimSpace(profile.Container) == "" || strings.TrimSpace(profile.VideoCodec) == "" || strings.TrimSpace(profile.AudioCodec) == "" {
		return errors.New("encoding profile requires container and audio/video codecs")
	}
	return nil
}

// ValidateObjectRef validates object identity and ownership fields.
func ValidateObjectRef(ref ObjectRef) error {
	if ref.Purpose == "" || ref.Bucket == "" || ref.Key == "" || ref.JobID == "" {
		return errors.New("object reference requires purpose, bucket, key, and job")
	}
	if ref.Purpose != ObjectInput && ref.Purpose != ObjectTaskOutput && ref.Purpose != ObjectFinalOutput {
		return fmt.Errorf("unknown object purpose %q", ref.Purpose)
	}
	if strings.HasPrefix(ref.Key, "/") || strings.Contains(ref.Key, "..") {
		return errors.New("object key must be opaque and traversal-free")
	}
	if ref.Purpose == ObjectTaskOutput && (ref.TaskID == "" || ref.AttemptID == "") {
		return errors.New("task output requires task and attempt ownership")
	}
	return nil
}

// ValidateResultMetadata validates integrity metadata before media validation runs.
func ValidateResultMetadata(result Result) error {
	if result.AttemptID == "" || result.TaskID == "" || result.WorkerID == "" {
		return errors.New("result requires attempt, task, and worker identity")
	}
	if result.Size < 0 || len(result.SHA256) != 64 {
		return errors.New("result requires a non-negative size and hexadecimal SHA-256 checksum")
	}
	for _, r := range result.SHA256 {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return errors.New("result checksum must be hexadecimal")
		}
	}
	return ValidateObjectRef(result.Object)
}

// ValidateJobTransition reports whether a job may move between two states.
func ValidateJobTransition(from, to JobState) error {
	allowed := map[JobState][]JobState{
		JobCreated:       {JobUploadPending, JobRejected, JobCancelled},
		JobUploadPending: {JobUploaded, JobRejected, JobCancelled},
		JobUploaded:      {JobInspecting, JobRejected, JobCancelled},
		JobInspecting:    {JobPlanned, JobRejected, JobFailed},
		JobPlanned:       {JobProcessing, JobFailed, JobCancelled},
		JobProcessing:    {JobAssembling, JobValidating, JobFailed, JobCancelled},
		JobAssembling:    {JobValidating, JobFailed},
		JobValidating:    {JobCompleted, JobFailed},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("invalid job transition %q to %q", from, to)
}

// ValidateTaskTransition reports whether a task may move between two states.
func ValidateTaskTransition(from, to TaskState) error {
	allowed := map[TaskState][]TaskState{
		TaskPending:    {TaskClaimed, TaskFailed},
		TaskClaimed:    {TaskValidating, TaskRetryable, TaskFailed},
		TaskValidating: {TaskAccepted, TaskRetryable, TaskFailed},
		TaskRetryable:  {TaskClaimed, TaskFailed},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("invalid task transition %q to %q", from, to)
}

// ValidateAttemptTransition reports whether an attempt may move between lease and result states.
func ValidateAttemptTransition(from, to AttemptState) error {
	allowed := map[AttemptState][]AttemptState{
		AttemptActive:    {AttemptSubmitted, AttemptRejected, AttemptExpired, AttemptStale},
		AttemptSubmitted: {AttemptAccepted, AttemptRejected, AttemptStale},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("invalid attempt transition %q to %q", from, to)
}

// ValidateResultTransition reports whether a submitted result may change validation state.
func ValidateResultTransition(from, to ResultState) error {
	allowed := map[ResultState][]ResultState{
		ResultSubmitted:  {ResultValidating, ResultRejected},
		ResultValidating: {ResultAccepted, ResultRejected},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("invalid result transition %q to %q", from, to)
}
