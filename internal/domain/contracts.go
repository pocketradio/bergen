package domain

import "time"

// ProcessingMode identifies how a task consumes its input media.
type ProcessingMode string

const (
	ProcessingWholeFile ProcessingMode = "whole_file"
	ProcessingSegment   ProcessingMode = "segment"
)

// MediaInfo contains metadata extracted from an input or output media object.
type MediaInfo struct {
	Duration     time.Duration `json:"duration"`
	Container    string        `json:"container"`
	VideoCodec   string        `json:"video_codec"`
	AudioCodec   string        `json:"audio_codec"`
	VideoStreams int           `json:"video_streams"`
	AudioStreams int           `json:"audio_streams"`
}

// ValidationLease identifies coordinator-only ownership after worker submission.
type ValidationLease struct {
	OwnerID   CoordinatorID `json:"owner_id"`
	ExpiresAt time.Time     `json:"expires_at"`
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
	CurrentAttempt *AttemptID     `json:"current_attempt_id,omitempty"`
	AttemptCount   int            `json:"attempt_count"`
}

// Attempt is one worker-owned, lease-fenced execution of a task.
type Attempt struct {
	ID              AttemptID        `json:"id"`
	TaskID          TaskID           `json:"task_id"`
	WorkerID        WorkerID         `json:"worker_id"`
	State           AttemptState     `json:"state"`
	Output          ObjectRef        `json:"output"`
	LeaseExpiresAt  time.Time        `json:"lease_expires_at"`
	ValidationLease *ValidationLease `json:"validation_lease,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

// Result is metadata submitted for one attempt and does not itself authorize acceptance.
type Result struct {
	ID          ResultID    `json:"id"`
	JobID       JobID       `json:"job_id"`
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
