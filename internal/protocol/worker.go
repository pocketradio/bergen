// Package protocol contains versioned HTTP wire contracts for contributor workers.
package protocol

import (
	"errors"
	"time"

	"github.com/pocketradio/bergen/internal/domain"
)

// APIVersion is the current worker API version.
const APIVersion = "v1"

// Capability describes media a worker can process.
type Capability struct {
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
}

// PollRequest asks the coordinator for one compatible task.
type PollRequest struct {
	Capabilities Capability `json:"capabilities"`
}

// TaskAssignment contains the server-authorized work and scoped object URLs.
type TaskAssignment struct {
	TaskID         domain.TaskID        `json:"task_id"`
	AttemptID      domain.AttemptID     `json:"attempt_id"`
	LeaseExpiresAt time.Time            `json:"lease_expires_at"`
	InputURL       string               `json:"input_url"`
	OutputURL      string               `json:"output_url"`
	Profile        domain.EncodeProfile `json:"profile"`
}

// HeartbeatRequest renews the current attempt lease.
type HeartbeatRequest struct {
	AttemptID domain.AttemptID `json:"attempt_id"`
}

// HeartbeatResponse returns the lease duration selected by the coordinator.
type HeartbeatResponse struct {
	LeaseExpiresAt time.Time `json:"lease_expires_at"`
}

// SubmitRequest submits result metadata after the worker uploads its output.
type SubmitRequest struct {
	AttemptID domain.AttemptID `json:"attempt_id"`
	ObjectKey string           `json:"object_key"`
	Size      int64            `json:"size"`
	SHA256    string           `json:"sha256"`
}

// SubmitResponse confirms that a current submission is awaiting validation.
type SubmitResponse struct {
	ResultID domain.ResultID    `json:"result_id"`
	State    domain.ResultState `json:"state"`
}

// ErrorResponse is the stable, secret-safe API error shape.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ValidatePollRequest checks the worker capabilities required for scheduling.
func ValidatePollRequest(request PollRequest) error {
	if request.Capabilities.VideoCodec == "" || request.Capabilities.AudioCodec == "" {
		return errors.New("worker capabilities require video and audio codecs")
	}
	return nil
}

// ValidateTaskAssignment checks that a claimed assignment is usable by a worker.
func ValidateTaskAssignment(assignment TaskAssignment) error {
	if assignment.TaskID == "" || assignment.AttemptID == "" || assignment.InputURL == "" || assignment.OutputURL == "" {
		return errors.New("task assignment requires task, attempt, input URL, and output URL")
	}
	if assignment.LeaseExpiresAt.IsZero() {
		return errors.New("task assignment requires lease expiry")
	}
	return domain.ValidateEncodeProfile(assignment.Profile)
}

// ValidateHeartbeatRequest checks that a heartbeat identifies an attempt.
func ValidateHeartbeatRequest(request HeartbeatRequest) error {
	if request.AttemptID == "" {
		return errors.New("heartbeat requires attempt ID")
	}
	return nil
}

// ValidateSubmitRequest checks result metadata before coordinator authorization.
func ValidateSubmitRequest(request SubmitRequest) error {
	if request.AttemptID == "" || request.ObjectKey == "" {
		return errors.New("result submission requires attempt ID and object key")
	}
	if request.Size < 0 || len(request.SHA256) != 64 {
		return errors.New("result submission requires non-negative size and SHA-256 checksum")
	}
	return nil
}
