// Package domain contains the stable business contracts shared by the API,
// coordinator, workers, persistence adapters, and validation components.
package domain

// ID is an opaque server-generated identifier represented as a string on the wire.
type ID string

// UserID identifies a client user.
type UserID ID

// WorkerID identifies a contributor worker.
type WorkerID ID

// CoordinatorID identifies the coordinator instance holding a validation lease.
type CoordinatorID ID

// JobID identifies a compression job.
type JobID ID

// TaskID identifies a unit of processing work.
type TaskID ID

// AttemptID identifies one fenced execution of a task.
type AttemptID ID

// ResultID identifies one submitted or accepted task result.
type ResultID ID

// SettlementKey identifies one idempotent credit settlement.
type SettlementKey string
