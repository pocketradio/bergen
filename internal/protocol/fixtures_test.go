package protocol

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/pocketradio/bergen/internal/domain"
)

// TestRequestJSONFixtures verifies the stable serialized request shapes for version one.
func TestRequestJSONFixtures(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "create job",
			value: CreateJobRequest{Output: OutputRequest{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac"}},
			want:  `{"output":{"container":"mp4","video_codec":"h264","audio_codec":"aac"}}`,
		},
		{
			name:  "worker poll",
			value: PollRequest{Capabilities: Capability{VideoCodec: "h264", AudioCodec: "aac"}},
			want:  `{"capabilities":{"video_codec":"h264","audio_codec":"aac"}}`,
		},
		{
			name:  "worker heartbeat",
			value: HeartbeatRequest{AttemptID: "attempt-1"},
			want:  `{"attempt_id":"attempt-1"}`,
		},
		{
			name:  "worker submission",
			value: SubmitRequest{AttemptID: "attempt-1", ObjectKey: "outputs/attempt-1.mp4", Size: 10, SHA256: workerSHA256},
			want:  `{"attempt_id":"attempt-1","object_key":"outputs/attempt-1.mp4","size":10,"sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}`,
		},
	}

	// iterating over dummy data to verify request JSON serialization
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("JSON = %s, want %s", got, test.want)
			}
		})
	}
}

// TestResponseJSONFixtures verifies the stable serialized response and error shapes for version one.
func TestResponseJSONFixtures(t *testing.T) {
	leaseExpiry := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{
			name:  "job response",
			value: JobResponse{JobID: "job-1", State: domain.JobCreated, Profile: testProfile()},
			want:  `{"job_id":"job-1","state":"created","profile":{"id":"h264-aac","version":"1","container":"mp4","video_codec":"h264","width":1920,"height":1080,"pixel_format":"yuv420p","frame_rate":"30/1","allow_variable_frame_rate":false,"h264_profile":"high","h264_level":"4.1","video_bitrate":4000000,"audio_codec":"aac","audio_sample_rate":48000,"audio_channels":2,"audio_bitrate":128000}}`,
		},
		{
			name:  "heartbeat response",
			value: HeartbeatResponse{LeaseExpiresAt: leaseExpiry},
			want:  `{"lease_expires_at":"2026-10-22T12:00:00Z"}`,
		},
		{
			name:  "submission response",
			value: SubmitResponse{ResultID: "result-1", State: domain.ResultSubmitted},
			want:  `{"result_id":"result-1","state":"submitted"}`,
		},
		{
			name:  "error response",
			value: ErrorResponse{Code: ErrorConflict, Message: "idempotency key conflicts"},
			want:  `{"code":"conflict","message":"idempotency key conflicts"}`,
		},
	}

	// iterating over dummy data to verify response JSON serialization
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != test.want {
				t.Fatalf("JSON = %s, want %s", got, test.want)
			}
		})
	}
}
