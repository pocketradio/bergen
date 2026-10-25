package protocol

import (
	"net/http"
	"testing"

	"github.com/pocketradio/bergen/internal/domain"
)

// TestClientContracts verifies versioned client requests and responses use a complete profile.
func TestClientContracts(t *testing.T) {
	if CreateJobPath != "/v1/jobs" {
		t.Fatalf("create job path = %q, want /v1/jobs", CreateJobPath)
	}
	if CreateJobStatus != http.StatusCreated {
		t.Fatalf("create job status = %d, want 201", CreateJobStatus)
	}
	if err := ValidateIdempotencyKey("create-job-1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIdempotencyKey(" "); err == nil {
		t.Fatal("expected empty idempotency key to be rejected")
	}

	request := CreateJobRequest{Output: OutputRequest{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}}
	if err := ValidateCreateJobRequest(request); err != nil {
		t.Fatal(err)
	}
	response := JobResponse{JobID: "job-1", State: domain.JobCreated, Profile: testProfile()}
	if err := ValidateJobResponse(response); err != nil {
		t.Fatal(err)
	}
}
