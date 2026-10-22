package protocol

import (
	"testing"
	"time"

	"github.com/pocketradio/bergen/internal/domain"
)

// TestWorkerRequests verifies required fields in the worker request contracts.
func TestWorkerRequests(t *testing.T) {
	if err := ValidatePollRequest(PollRequest{Capabilities: Capability{VideoCodec: "h264", AudioCodec: "aac"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHeartbeatRequest(HeartbeatRequest{AttemptID: "attempt-1"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSubmitRequest(SubmitRequest{AttemptID: "attempt-1", ObjectKey: "output.mp4", Size: 1, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}); err != nil {
		t.Fatal(err)
	}
}

// TestTaskAssignmentValidation verifies assignments contain lease and profile data.
func TestTaskAssignmentValidation(t *testing.T) {
	assignment := TaskAssignment{
		TaskID: "task-1", AttemptID: "attempt-1", InputURL: "https://storage/input",
		OutputURL: "https://storage/output", LeaseExpiresAt: time.Now().Add(time.Minute),
		Profile: domain.EncodeProfile{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"},
	}
	if err := ValidateTaskAssignment(assignment); err != nil {
		t.Fatal(err)
	}
	assignment.OutputURL = ""
	if err := ValidateTaskAssignment(assignment); err == nil {
		t.Fatal("expected missing output URL to be rejected")
	}
}
