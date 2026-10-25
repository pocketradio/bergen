package domain

import (
	"errors"
	"fmt"
	"strings"
)

// ObjectPurpose identifies why an object exists and who may reference it.
type ObjectPurpose string

const (
	ObjectInput       ObjectPurpose = "input"
	ObjectTaskOutput  ObjectPurpose = "task_output"
	ObjectFinalOutput ObjectPurpose = "final_output"
)

// ObjectRef identifies an object without granting access to it.
// Version remains empty while an assigned key is reserved but not yet bound to observed bytes.
type ObjectRef struct {
	Purpose   ObjectPurpose `json:"purpose"`
	Bucket    string        `json:"bucket"`
	Key       string        `json:"key"`
	Version   string        `json:"version,omitempty"`
	JobID     JobID         `json:"job_id"`
	TaskID    TaskID        `json:"task_id,omitempty"`
	AttemptID AttemptID     `json:"attempt_id,omitempty"`
}

// ManagedObject is the durable lifecycle record for a server-owned object reference.
type ManagedObject struct {
	Ref   ObjectRef   `json:"ref"`
	State ObjectState `json:"state"`
}

// ValidateObjectRef validates object identity and ownership fields.
func ValidateObjectRef(ref ObjectRef) error {
	if ref.Purpose == "" || ref.Bucket == "" || ref.Key == "" || ref.JobID == "" {
		return errors.New("object reference requires purpose, bucket, key, and job")
	}
	if ref.Purpose != ObjectInput && ref.Purpose != ObjectTaskOutput && ref.Purpose != ObjectFinalOutput {
		return fmt.Errorf("unknown object purpose %q", ref.Purpose)
	}
	if err := ValidateObjectKey(ref.Key); err != nil {
		return err
	}
	switch ref.Purpose {
	case ObjectInput:
		if ref.TaskID != "" || ref.AttemptID != "" {
			return errors.New("input object must not have task or attempt ownership")
		}
	case ObjectTaskOutput:
		if ref.TaskID == "" || ref.AttemptID == "" {
			return errors.New("task output requires task and attempt ownership")
		}
	case ObjectFinalOutput:
		if ref.TaskID != "" || ref.AttemptID != "" {
			return errors.New("final output must not have task or attempt ownership")
		}
	}
	return nil
}

// ValidateObjectKey validates an opaque, traversal-free object key.
func ValidateObjectKey(key string) error {
	if strings.TrimSpace(key) == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		return errors.New("object key must be opaque and traversal-free")
	}
	return nil
}

// ValidateBoundObjectRef validates an object reference that has been bound to one observed storage version.
func ValidateBoundObjectRef(ref ObjectRef) error {
	if err := ValidateObjectRef(ref); err != nil {
		return err
	}
	if strings.TrimSpace(ref.Version) == "" {
		return errors.New("bound object reference requires storage version")
	}
	return nil
}

// ValidateResultMetadata validates integrity metadata before media validation runs.
func ValidateResultMetadata(result Result) error {
	if result.JobID == "" || result.AttemptID == "" || result.TaskID == "" || result.WorkerID == "" {
		return errors.New("result requires job, attempt, task, and worker identity")
	}
	if result.Object.Purpose != ObjectTaskOutput || result.Object.JobID != result.JobID || result.Object.TaskID != result.TaskID || result.Object.AttemptID != result.AttemptID {
		return errors.New("result object must match the result job, task, and attempt")
	}
	if result.Size < 0 {
		return errors.New("result requires a non-negative size")
	}
	if err := ValidateSHA256(result.SHA256); err != nil {
		return fmt.Errorf("result checksum: %w", err)
	}
	return ValidateObjectRef(result.Object)
}

// ValidateAcceptedResult validates the version-bound object required for an authoritative result.
func ValidateAcceptedResult(result Result) error {
	if err := ValidateResultMetadata(result); err != nil {
		return err
	}
	if result.State != ResultAccepted {
		return errors.New("accepted result must have accepted state")
	}
	return ValidateBoundObjectRef(result.Object)
}
