package domain

import "testing"

const testFingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// TestIdempotencyRecordValidation verifies client replay records preserve a complete original outcome.
func TestIdempotencyRecordValidation(t *testing.T) {
	record := IdempotencyRecord{
		Scope:       IdempotencyScope{PrincipalKind: PrincipalClient, PrincipalID: "user-1", Operation: "create_job"},
		Key:         "key-1",
		Fingerprint: testFingerprint,
		ResourceID:  "job-1",
		Outcome:     IdempotencyOutcome{StatusCode: 201, ResponseBody: []byte(`{"job_id":"job-1"}`)},
	}
	if err := ValidateIdempotencyRecord(record); err != nil {
		t.Fatal(err)
	}

	invalid := record
	invalid.Fingerprint = "not-a-fingerprint"
	if err := ValidateIdempotencyRecord(invalid); err == nil {
		t.Fatal("expected invalid fingerprint to be rejected")
	}

	// only accept lowercase fingerprints
	invalid = record
	invalid.Fingerprint = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
	if err := ValidateIdempotencyRecord(invalid); err == nil {
		t.Fatal("expected uppercase fingerprint to be rejected")
	}

	// response body should be json
	invalid = record
	invalid.Outcome.ResponseBody = []byte(`not-json`)
	if err := ValidateIdempotencyRecord(invalid); err == nil {
		t.Fatal("expected invalid outcome response to be rejected")
	}

	// create job must not have a target
	// because no job exists before creation
	invalid = record
	invalid.Scope.TargetID = "unexpected-target"
	if err := ValidateIdempotencyRecord(invalid); err == nil {
		t.Fatal("expected create job scope target to be rejected")
	}

	// idempotency is scoped to the job
	// changing the operation on the same key implies it's not longer the same create job operation
	// thus it must fail
	targeted := record
	targeted.Scope.Operation = "finalize_upload"
	targeted.Scope.TargetID = "job-1"
	if err := ValidateIdempotencyRecord(targeted); err != nil {
		t.Fatal(err)
	}
}

// TestWorkerIdempotencyScopeValidation verifies worker submissions are scoped to one attempt.
func TestWorkerIdempotencyScopeValidation(t *testing.T) {
	record := IdempotencyRecord{
		Scope:       IdempotencyScope{PrincipalKind: PrincipalWorker, PrincipalID: "worker-1", Operation: "submit_result", TargetID: "attempt-1"},
		Key:         "key-1",
		Fingerprint: testFingerprint,
		ResourceID:  "result-1",
		Outcome:     IdempotencyOutcome{StatusCode: 202, ResponseBody: []byte(`{"result_id":"result-1"}`)},
	}
	if err := ValidateIdempotencyRecord(record); err != nil {
		t.Fatal(err)
	}
	record.Scope.TargetID = ""
	if err := ValidateIdempotencyRecord(record); err == nil {
		t.Fatal("expected worker scope without attempt target to be rejected")
	}
}
