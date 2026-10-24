package protocol

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pocketradio/bergen/internal/domain"
)

// ClientAPIVersion is the current version prefix for client endpoints.
const ClientAPIVersion = APIVersion

const (
	// CreateJobPath is the versioned endpoint for creating a compression job.
	CreateJobPath = "/" + ClientAPIVersion + "/jobs"

	// IdempotencyKeyHeader carries a caller-selected idempotency key for retryable client operations.
	IdempotencyKeyHeader = "Idempotency-Key"

	// CreateJobStatus is returned for a new or identical replayed job creation.
	CreateJobStatus = http.StatusCreated
)

// CreateJobRequest is the versioned client payload for requesting a server-selected output profile.
type CreateJobRequest struct {
	Output OutputRequest `json:"output"`
}

// OutputRequest expresses typed output requirements without carrying encoder arguments.
type OutputRequest struct {
	Container  string `json:"container"`
	VideoCodec string `json:"video_codec"`
	AudioCodec string `json:"audio_codec"`
}

// JobResponse is the versioned client representation of a job.
type JobResponse struct {
	JobID   domain.JobID         `json:"job_id"`
	State   domain.JobState      `json:"state"`
	Profile domain.EncodeProfile `json:"profile"`
}

// ValidateIdempotencyKey validates that a requester supplied a key without selecting the unresolved key policy.
func ValidateIdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("idempotency key is required")
	}
	return nil
}

// ValidateCreateJobRequest checks the structural request shape while server policy selects a matching profile.
func ValidateCreateJobRequest(request CreateJobRequest) error {
	if request.Output.Container == "" || request.Output.VideoCodec == "" || request.Output.AudioCodec == "" {
		return errors.New("output request requires container and audio/video codecs")
	}
	return nil
}

// ValidateJobResponse checks the stable client response contains an immutable profile revision.
func ValidateJobResponse(response JobResponse) error {
	if response.JobID == "" || response.State == "" {
		return errors.New("job response requires job ID and state")
	}
	return domain.ValidateEncodeProfile(response.Profile)
}
