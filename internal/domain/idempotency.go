package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// IdempotencyKey identifies one retryable client operation within its scope.
type IdempotencyKey string

// RequestFingerprint identifies the canonical payload of an idempotent request.
type RequestFingerprint string

// PrincipalKind identifies the authenticated principal type owning an idempotency namespace.
type PrincipalKind string

const (
	PrincipalClient PrincipalKind = "client"
	PrincipalWorker PrincipalKind = "worker"
)

// IdempotencyScope identifies the authenticated principal and operation sharing a key namespace.
type IdempotencyScope struct {
	PrincipalKind PrincipalKind `json:"principal_kind"`
	PrincipalID   ID            `json:"principal_id"`
	Operation     string        `json:"operation"`
	TargetID      ID            `json:"target_id,omitempty"`
}

// IdempotencyOutcome stores the original response required for an identical replay.
type IdempotencyOutcome struct {
	StatusCode   int             `json:"status_code"`
	ResponseBody json.RawMessage `json:"response_body,omitempty"`
}

// IdempotencyRecord captures the durable identity and original outcome of one idempotent operation.
type IdempotencyRecord struct {
	Scope       IdempotencyScope   `json:"scope"`
	Key         IdempotencyKey     `json:"key"`
	Fingerprint RequestFingerprint `json:"fingerprint"`
	ResourceID  ID                 `json:"resource_id"`
	Outcome     IdempotencyOutcome `json:"outcome"`
}

// ValidateIdempotencyRecord validates the durable identity required to distinguish replays from conflicts.
func ValidateIdempotencyRecord(record IdempotencyRecord) error {
	if (record.Scope.PrincipalKind != PrincipalClient && record.Scope.PrincipalKind != PrincipalWorker) || record.Scope.PrincipalID == "" || strings.TrimSpace(record.Scope.Operation) == "" {
		return errors.New("idempotency scope requires client or worker principal and operation")
	}
	if record.Scope.PrincipalKind == PrincipalWorker && record.Scope.TargetID == "" {
		return errors.New("worker idempotency scope requires an attempt target")
	}
	if record.Scope.PrincipalKind == PrincipalClient && record.Scope.Operation == "create_job" && record.Scope.TargetID != "" {
		return errors.New("create job idempotency scope must not have a target")
	}
	if strings.TrimSpace(string(record.Key)) == "" || record.ResourceID == "" {
		return errors.New("idempotency record requires key and resource")
	}
	if err := ValidateLowercaseSHA256(string(record.Fingerprint)); err != nil {
		return fmt.Errorf("idempotency fingerprint: %w", err)
	}
	if record.Outcome.StatusCode < 100 || record.Outcome.StatusCode > 599 {
		return errors.New("idempotency outcome requires an HTTP status code")
	}
	if len(record.Outcome.ResponseBody) == 0 || !json.Valid(record.Outcome.ResponseBody) {
		return errors.New("idempotency outcome requires a valid JSON response")
	}
	return nil
}
