# Phase 1.5 contract hardening

This document is the implementation contract for phases that depend on Phase 1.5. It defines data and transactional rules only; it does not introduce a persistence adapter, HTTP handler, object-storage operation, or coordinator workflow. Go representations are in `internal/domain` and `internal/protocol`.

## Versioning and errors

Public client and worker endpoints use the `/v1` prefix. A compatible change may add optional response fields; a required field, semantic, or error-code change requires a new version prefix.

All API failures use:

```json
{ "code": "machine_readable_code", "message": "safe explanation" }
```

| Code | HTTP status | Meaning |
| --- | --- | --- |
| `invalid_request` | 400 | Structurally invalid request or unsupported server policy input. |
| `unauthorized` | 401 | Missing, invalid, expired, or revoked credential. |
| `forbidden` | 403 | Authenticated principal does not own the resource. |
| `not_found` | 404 | Resource is absent or must not be disclosed. |
| `conflict` | 409 | Conflicting idempotency replay or incompatible current state. |
| `stale_attempt` | 409 | Attempt is not the task's current fenced attempt. |
| `lease_expired` | 409 | Current worker lease expired before the requested operation. |
| `internal` | 500 | Unexpected server failure; internal details are not exposed. |

Messages must not contain credentials, presigned URLs, object contents, or untrusted raw storage errors.

## Client and worker wire contracts

`POST /v1/jobs` accepts `CreateJobRequest`, which carries typed requested container/video/audio codecs, and requires the `Idempotency-Key` header. The service selects a matching, server-defined `EncodeProfile`; it never accepts FFmpeg arguments. A new request returns `201 Created` with `JobResponse`. An identical replay returns the stored `201` response body; the same key with a different canonical fingerprint returns `409 conflict`. `JobResponse` always returns the persisted immutable profile revision. The exact idempotency-key length and character policy remains unresolved until Phase 3, but the header must be nonempty.

The worker contract remains in [worker_protocol.md](worker_protocol.md). Worker identity comes exclusively from its credential. A worker receives only a task assignment's scoped URLs and the full immutable profile; it does not receive client identity, database credentials, or unrestricted storage access.

## Encoding profile

A persisted profile contains an immutable `id` and `version`, plus container, video codec, dimensions, pixel format, frame rate and VFR allowance, H.264 profile/level, video bitrate, audio codec, sample rate, channels, and bitrate. These fields are structural requirements, not selected values: supported values and quality limits remain server policy. Tasks persist the full selected profile revision, rather than re-resolving a profile by mutable name.

## Ownership and object identity

- A `Job` is owned by its `OwnerID`; only that client principal may access its upload, status, or final download.
- A `Task` belongs to exactly one job. Its input object belongs to that same job.
- An `Attempt` belongs to exactly one task and authenticated worker. `Task.CurrentAttempt` is the only worker-authoritative attempt.
- An input object belongs to one job and has no task or attempt owner. A task output object belongs to exactly one `(job_id, task_id, attempt_id)` tuple. A final-output object belongs to one job and has no task or attempt owner. A worker may submit only the output key assigned to its current attempt.
- A result records its job, attempt, task, worker, and exact task-output reference; every result ownership ID must equal the corresponding object ownership ID. It can become authoritative only while its attempt is current and its coordinator validation lease is current.
- An object reference consists of purpose, bucket, server-generated key, ownership IDs, and an optional provider version. A blank version means a reserved assignment that has not yet been bound to observed bytes. Validation, acceptance, promotion, download, and reconciliation must require a nonblank bound version. Cleanup may delete an unbound `reserved` assignment only after confirming storage never observed the assigned object; this is a recorded no-op cleanup outcome rather than deletion of a bound object.

The provider version may be an S3 version ID, a service-defined immutable-copy identity, or another provider value that changes on overwrite. An ETag alone is not sufficient unless the selected provider contract guarantees it as a version identity for the operation. A checksum verifies observed bytes but does not make a key immutable.

## State machines and transactional guards

All transitions below occur in a single database transaction. A successful transition writes the new state and all stated side effects atomically. Implementations use compare-and-set predicates and must treat zero affected rows as a current-state conflict rather than overwriting a newer state. `WorkerAttemptGuard`, `SubmissionAttemptGuard`, `CoordinatorWorkGuard`, `ValidationAttemptGuard`, `ValidationRecoveryGuard`, and `RetryTransitionGuard` define the values later persistence operations must validate before issuing their compare-and-set updates; they do not replace those transactional predicates.

### Job

| From | To | Guard and atomic side effect |
| --- | --- | --- |
| `created` | `upload_pending` | Job owner and selected profile are persisted. |
| `upload_pending` | `uploaded` | Assigned input object is present and version-bound. |
| `uploaded` | `inspection_pending` | Durable inspection work is recorded; no FFprobe runs in the transaction. |
| `inspection_pending` | `inspecting` | Coordinator claims pending inspection work. |
| `inspecting` | `task_pending` | Inspection result is accepted for the current work claim. |
| `task_pending` | `planned` | Exactly the planned task set is persisted. |
| `planned` | `processing` | Required task processing starts. |
| `processing` | `promotion_pending` | Every required task has one accepted result. |
| `promotion_pending` | `promoting` | Coordinator claims recoverable promotion work. |
| `promoting` | `completed` | Exact final-object version is bound and final validation succeeded. |
| `created`, `upload_pending`, or `uploaded` | `rejected` or `cancelled` | Client cancellation or intake rejection records a safe terminal reason. |
| `inspection_pending` or `inspecting` | `rejected` or `inspection_failed` | Inspection rejection or bounded terminal inspection failure records a safe terminal reason. |
| `task_pending`, `planned`, or `processing` | `failed` | Required planning or processing work reaches a terminal failure. |
| `planned` or `processing` | `cancelled` | Authorized client cancellation records a safe terminal reason before completion. |
| `promotion_pending` or `promoting` | `promotion_failed` | Promotion reaches a terminal failure after its bounded recovery policy. |

`rejected`, `inspection_failed`, `promotion_failed`, `failed`, `cancelled`, and `completed` are terminal. A transition to a terminal error state records a safe user-visible reason and never removes provenance.

Inspection and planning share one job-owned `intake` claim; promotion uses a `promotion` claim. Each claim records its coordinator, unique fencing token, and expiry. The intake claim is held from `inspection_pending` through `task_pending` until planning completes; the promotion claim is held from `promotion_pending` until completion. Renewal and completion require the current token, matching work kind and coordinator, and an unexpired claim. Recovery atomically replaces an expired claim's token before continuing; a completion using the old token must fail. Clear the claim when its work completes or reaches a terminal outcome. Later persistence phases enforce these checks in the same transaction as each state change.

### Task, attempt, and result

| Entity | From | To | Guard and atomic side effect |
| --- | --- | --- | --- |
| Task | `pending` or `retryable` | `leased` | Eligible worker claim atomically sets `current_attempt_id`, increments attempt count, creates a fresh attempt and unique output assignment. |
| Task | `pending` | `failed` | Planning makes the task terminal before it can be leased. |
| Attempt | `leased` | `submitted` | Authenticated worker matches attempt worker; task current attempt matches; worker lease has not expired; submitted key exactly matches the assigned task-output key. |
| Result | absent | `submitted` | Submission metadata is recorded exactly once for the current attempt. |
| Result | `submitted` | `rejected` | Submission metadata or authorization is terminally invalid before validation starts. |
| Attempt | `submitted` | `validating` | Worker authority ends and a coordinator ID plus bounded validation lease are written. |
| Result | `submitted` | `validating` | The same coordinator transaction starts validation for the submitted result. |
| Task | `leased` | `validating` | The same coordinator transaction transfers task ownership from the worker to validation. |
| Attempt | `validating` | `accepted` or `rejected` | Coordinator owns an unexpired validation lease and task current attempt still matches. |
| Result | `validating` | `accepted` or `rejected` | Result attempt equals the guarded current attempt, and exact object version, integrity, media, and decode checks have completed. |
| Task | `validating` | `accepted`, `retryable`, or `failed` | `accepted_result_id` is null before acceptance; otherwise the recorded retry class and bounded-retry decision select `retryable` or `failed`. |
| Task | `leased` | `retryable` or `failed` | Execution expiry or worker-side failure records a retry class before reassignment. |
| Task | `retryable` | `failed` | The bounded retry policy is exhausted or requeue becomes terminal. |
| Attempt | `leased` | `rejected`, `expired`, or `stale` | Execution expiry or worker-side failure fences the attempt; stale attempts cannot change task or accepted-result state. |
| Attempt | `submitted` or `validating` | `rejected` or `stale` | Validation failure or coordinator recovery fences the attempt without treating worker execution as expired. |

`accepted`, `rejected`, `expired`, and `stale` attempts are terminal. `accepted` and `rejected` results are terminal. `accepted` and `failed` tasks are terminal. A worker heartbeat may extend only its current `leased` attempt before the worker lease expires. It must reject a submitted or validating attempt. The coordinator validation lease is not renewable by a worker.

### Object

| From | To | Guard |
| --- | --- | --- |
| `reserved` | `uploaded` | Server-assigned key was observed in storage. |
| `uploaded` | `verified` | Actual size/checksum and exact storage version were recorded. |
| `verified` | `promotion_pending` | Source is an accepted, bound result. |
| `promotion_pending` | `promoted` | Conditional/version-bound promotion wrote and bound the final object. |
| a bound live state | `deletion_pending` | Retention expired and no active reference, retry, validation, promotion, or retention requirement remains. |
| `reserved` and unbound | `deletion_pending` | Storage inspection confirmed the assigned object was never observed; deletion is recorded as a no-op outcome. |
| `deletion_pending` | `deleted` / `deletion_failed` | Deletion outcome is durably recorded. |

`deleted` is terminal. A promotion must condition its copy/read on the recorded source version; an overwrite at the same key cannot be promoted. `deletion_failed` may return only to `deletion_pending` for a later retry.

## Idempotency

An idempotent operation is uniquely identified by `(principal_kind, authenticated_principal_id, operation, target_id, idempotency_key)`. `target_id` is empty for client job creation, identifies the target job for client job-scoped operations such as upload finalization, and is the attempt ID for worker result submission. The durable record stores a lowercase hexadecimal SHA-256 request fingerprint, the created resource ID, and the original HTTP status plus JSON response body.

- The fingerprint is SHA-256 over the UTF-8 JSON Canonicalization Scheme (RFC 8785) serialization of the validated request body. Authentication, endpoint path, operation, target ID, and idempotency key are excluded because they participate in the durable uniqueness tuple; HTTP headers are not included. Field ordering, whitespace, JSON formatting, equivalent escaping, and numeric rendering cannot change it.
- The same scope/key/fingerprint is an identical replay: return the recorded status and response body without creating another resource.
- The same scope/key with a different fingerprint is a `conflict`.
- Different authenticated principals have independent key namespaces. Client operations use a `client` principal; worker submission uses a `worker` principal with its attempt ID as `target_id`.
- Worker result submission requires the `Idempotency-Key` header and uses the same behavior scoped to the authenticated worker and attempt: identical metadata returns the stored submission; different metadata conflicts.

The exact key length/character policy remains an API input-validation decision. It must be recorded before implementation and must not silently change replay identity.

## Validation leases, fencing, and retries

Attempt creation generates a new `attempt_id`; it is the fencing token. Every worker heartbeat, submission, validation completion, rejection, and acceptance compares its attempt ID to the task's current attempt inside the transaction. Reassignment marks the previous attempt `stale` or `expired` before creating a new one. A stale write returns `stale_attempt` and cannot alter task state, accepted result, final output, or credits.

Validation acceptance compares the result's job, task, worker, and attempt IDs with the authoritative job, task, and current attempt. The result object's job, task, and attempt IDs must match the result. The later acceptance transaction enforces these ownership comparisons with the lease and current-attempt predicates.

When a current worker submits an output, the worker lease is relinquished and the coordinator creates a bounded validation lease. If that lease expires, only coordinator recovery may classify/requeue the task. `ValidationRecoveryGuard` requires the current attempt to be `validating`, the validation lease to be expired, and the attempt to be fenced as `stale` in the same transaction before a retryable transition. It must not immediately reassign a `submitted` or `validating` attempt without fencing it in the same transaction.

| Retry class | Disposition |
| --- | --- |
| Execution lease expired | Retry allowed |
| Validation lease expired | Retry allowed |
| Validation timeout | Retry allowed |
| Transient storage failure | Retry allowed |
| Transient infrastructure failure | Retry allowed |
| Malformed submission | Terminal for that attempt/task path |
| Object integrity failure | Terminal for that attempt/task path |
| Invalid media/profile/decode result | Terminal for that attempt/task path |

“Retry allowed” means the configured bounded retry policy may requeue the task; it does not select a retry count. The retry limit, lease duration, and validation timeout remain open policy decisions.

## Ledger contract

The Phase 26 prerequisite is a minimal ledger contract: a normal settlement has a stable unique key derived from the accepted contribution and reward type; entries are immutable; reversals and adjustments reference the entry they affect; and only an accepted result may be reward-eligible. Credit amount and own-job eligibility remain unresolved policy and are not represented by a default amount in this phase.
