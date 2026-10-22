package domain

import "testing"

// TestObjectReferenceValidation verifies purpose-specific ownership, version binding, and traversal rejection.
func TestObjectReferenceValidation(t *testing.T) {
	tests := []struct {
		name string
		ref  ObjectRef
		want bool
	}{
		{name: "input", ref: ObjectRef{Purpose: ObjectInput, Bucket: "bergen", Key: "jobs/abc/input.mp4", JobID: "job-1"}, want: true},
		{name: "task output", ref: ObjectRef{Purpose: ObjectTaskOutput, Bucket: "bergen", Key: "tasks/task-1/attempt-1/out.mp4", JobID: "job-1", TaskID: "task-1", AttemptID: "attempt-1"}, want: true},
		{name: "final output", ref: ObjectRef{Purpose: ObjectFinalOutput, Bucket: "bergen", Key: "jobs/job-1/final.mp4", JobID: "job-1"}, want: true},
		{name: "input with attempt", ref: ObjectRef{Purpose: ObjectInput, Bucket: "bergen", Key: "jobs/abc/input.mp4", JobID: "job-1", AttemptID: "attempt-1"}, want: false},
		{name: "task output without task", ref: ObjectRef{Purpose: ObjectTaskOutput, Bucket: "bergen", Key: "out.mp4", JobID: "job-1", AttemptID: "attempt-1"}, want: false},
		{name: "final output with task", ref: ObjectRef{Purpose: ObjectFinalOutput, Bucket: "bergen", Key: "final.mp4", JobID: "job-1", TaskID: "task-1"}, want: false},
		{name: "traversal", ref: ObjectRef{Purpose: ObjectInput, Bucket: "bergen", Key: "jobs/../secret", JobID: "job-1"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ValidateObjectRef(test.ref) == nil; got != test.want {
				t.Fatalf("object validation = %v, want %v", got, test.want)
			}
		})
	}

	bound := tests[0].ref
	if err := ValidateBoundObjectRef(bound); err == nil {
		t.Fatal("expected unbound object reference to be rejected")
	}

	bound.Version = "version-1"
	if err := ValidateBoundObjectRef(bound); err != nil {
		t.Fatal(err)
	}
}

// TestResultMetadataValidation verifies results may reference only their exact task-output object.
func TestResultMetadataValidation(t *testing.T) {
	result := testResult()
	if err := ValidateResultMetadata(result); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "input object", mutate: func(result *Result) {
			result.Object.Purpose = ObjectInput
			result.Object.TaskID = ""
			result.Object.AttemptID = ""
		}},
		{name: "different job", mutate: func(result *Result) { result.Object.JobID = "job-2" }},
		{name: "different task", mutate: func(result *Result) { result.Object.TaskID = "task-2" }},
		{name: "different attempt", mutate: func(result *Result) { result.Object.AttemptID = "attempt-2" }},
		{name: "invalid checksum", mutate: func(result *Result) { result.SHA256 = "not-a-checksum" }},
	}

	// iterating over our dummy data
	// with different invalid object properties
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := testResult()
			test.mutate(&invalid)
			if err := ValidateResultMetadata(invalid); err == nil {
				t.Fatal("expected invalid result metadata to be rejected")
			}
		})
	}
}

// TestAcceptedResultValidation verifies authoritative results require an accepted state and bound object version.
func TestAcceptedResultValidation(t *testing.T) {
	result := testResult()
	result.State = ResultAccepted
	if err := ValidateAcceptedResult(result); err == nil {
		t.Fatal("expected accepted result without object version to be rejected")
	}
	result.Object.Version = "version-1"
	if err := ValidateAcceptedResult(result); err != nil {
		t.Fatal(err)
	}
}

// testResult returns valid submission metadata before an object version is observed.
func testResult() Result {
	return Result{
		JobID: "job-1", AttemptID: "attempt-1", TaskID: "task-1", WorkerID: "worker-1", Size: 10,
		SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Object: ObjectRef{Purpose: ObjectTaskOutput, Bucket: "bergen", Key: "tasks/task-1/attempt-1/out.mp4", JobID: "job-1", TaskID: "task-1", AttemptID: "attempt-1"},
	}
}
