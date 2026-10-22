package domain

import "testing"

// TestJobTransitions verifies the documented job state machine accepts only legal moves.
func TestJobTransitions(t *testing.T) {
	tests := []struct {
		name string
		from JobState
		to   JobState
		want bool
	}{
		{name: "upload starts", from: JobCreated, to: JobUploadPending, want: true},
		{name: "processing starts", from: JobPlanned, to: JobProcessing, want: true},
		{name: "completion is terminal", from: JobCompleted, to: JobProcessing, want: false},
		{name: "cannot skip upload", from: JobCreated, to: JobProcessing, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidateJobTransition(test.from, test.to) == nil; got != test.want {
				t.Fatalf("transition %q to %q valid = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

// TestAttemptAndResultTransitions verifies stale executions cannot become authoritative.
func TestAttemptAndResultTransitions(t *testing.T) {
	if err := ValidateAttemptTransition(AttemptActive, AttemptSubmitted); err != nil {
		t.Fatal(err)
	}
	if err := ValidateAttemptTransition(AttemptStale, AttemptAccepted); err == nil {
		t.Fatal("expected stale attempt to remain terminal")
	}
	if err := ValidateResultTransition(ResultValidating, ResultAccepted); err != nil {
		t.Fatal(err)
	}
	if err := ValidateResultTransition(ResultAccepted, ResultRejected); err == nil {
		t.Fatal("expected accepted result to remain terminal")
	}
}

// TestObjectReferenceValidation verifies ownership fields and traversal rejection.
func TestObjectReferenceValidation(t *testing.T) {
	valid := ObjectRef{Purpose: ObjectInput, Bucket: "bergen", Key: "jobs/abc/input.mp4", JobID: "job-1"}
	if err := ValidateObjectRef(valid); err != nil {
		t.Fatal(err)
	}
	invalid := valid
	invalid.Key = "jobs/../secret"
	if err := ValidateObjectRef(invalid); err == nil {
		t.Fatal("expected traversal key to be rejected")
	}
}

// TestResultMetadataValidation verifies checksum shape and required ownership metadata.
func TestResultMetadataValidation(t *testing.T) {
	result := Result{
		AttemptID: "attempt-1", TaskID: "task-1", WorkerID: "worker-1", Size: 10,
		SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Object: ObjectRef{Purpose: ObjectTaskOutput, Bucket: "bergen", Key: "tasks/task-1/attempt-1/out.mp4", JobID: "job-1", TaskID: "task-1", AttemptID: "attempt-1"},
	}
	if err := ValidateResultMetadata(result); err != nil {
		t.Fatal(err)
	}
	result.SHA256 = "not-a-checksum"
	if err := ValidateResultMetadata(result); err == nil {
		t.Fatal("expected invalid checksum to be rejected")
	}
}
