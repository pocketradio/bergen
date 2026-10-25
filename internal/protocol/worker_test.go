package protocol

import (
	"testing"
	"time"

	"github.com/pocketradio/bergen/internal/domain"
)

const workerSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestWorkerRequests verifies required fields in the worker request contracts.
func TestWorkerRequests(t *testing.T) {
	if WorkerSubmitIdempotencyKeyHeader != IdempotencyKeyHeader {
		t.Fatalf("worker idempotency header = %q, want %q", WorkerSubmitIdempotencyKeyHeader, IdempotencyKeyHeader)
	}
	if err := ValidateIdempotencyKey("submit-1"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePollRequest(PollRequest{Capabilities: Capability{VideoCodec: "h264", AudioCodec: "aac"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateHeartbeatRequest(HeartbeatRequest{AttemptID: "attempt-1"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSubmitRequest(SubmitRequest{AttemptID: "attempt-1", ObjectKey: "output.mp4", Size: 1, SHA256: workerSHA256}); err != nil {
		t.Fatal(err)
	}
}

// TestSubmitRequestValidation verifies checksum and object-key validation matches the domain contract.
func TestSubmitRequestValidation(t *testing.T) {
	request := SubmitRequest{AttemptID: "attempt-1", ObjectKey: "outputs/attempt-1.mp4", Size: 1, SHA256: workerSHA256}
	request.SHA256 = "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"
	if err := ValidateSubmitRequest(request); err == nil {
		t.Fatal("expected non-hex checksum to be rejected")
	}
	request.SHA256 = workerSHA256
	request.ObjectKey = "outputs/../other.mp4"
	if err := ValidateSubmitRequest(request); err == nil {
		t.Fatal("expected unsafe object key to be rejected")
	}
}

// TestTaskAssignmentValidation verifies assignments contain lease and profile data.
func TestTaskAssignmentValidation(t *testing.T) {
	assignment := TaskAssignment{
		TaskID: "task-1", AttemptID: "attempt-1", InputURL: "https://storage/input",
		OutputURL: "https://storage/output", LeaseExpiresAt: time.Now().Add(time.Minute),
		Profile: testProfile(),
	}
	if err := ValidateTaskAssignment(assignment); err != nil {
		t.Fatal(err)
	}

	// check missing output url
	assignment.OutputURL = ""
	if err := ValidateTaskAssignment(assignment); err == nil {
		t.Fatal("expected missing output URL to be rejected")
	}
}

// TestWorkerResponses verifies response contracts expose only an active validation submission and lease expiry.
func TestWorkerResponses(t *testing.T) {
	if err := ValidateHeartbeatResponse(HeartbeatResponse{LeaseExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSubmitResponse(SubmitResponse{ResultID: "result-1", State: domain.ResultSubmitted}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSubmitResponse(SubmitResponse{ResultID: "result-1", State: domain.ResultAccepted}); err == nil {
		t.Fatal("expected accepted result state to be rejected from submission response")
	}
}
