# MVP Implementation Plan

## Objective

Deliver the documented MVP: an authenticated client submits a video, uploads it directly to S3:compatible storage, and receives a validated compressed output produced by one contributor worker. PostgreSQL is authoritative for application and processing state. Credits are non:monetary and recorded through an idempotent ledger after accepted work; reward issuance remains gated by the unresolved credit policy.

The MVP uses the repository's existing Go 1.23 toolchain, standard:library `net/http`, pgx, custom SQL migration runner, MinIO:compatible storage adapter, Docker Compose, Makefile, and CI checks. Phase 0 is complete. This plan does not introduce Chi, sqlc, a migration library, testify, or testcontainers.

## Phase execution model

Phase numbers identify the order of integration, not an exclusive work schedule. Each phase lists its implementation prerequisites and the work that can begin concurrently. After Phase 1 contracts are agreed, independent phases may be assigned and developed in parallel against fakes, fixtures, and those contracts. A listed integration prerequisite is required before end:to:end use, but does not prevent isolated implementation. Shared schemas, interfaces, and protocol changes require coordinated ownership to avoid conflicting edits.

Each phase's acceptance criteria are its review boundary. Tests for that phase are included in its acceptance criteria; testing is not deferred to one final test phase.

## MVP phases

### Phase 0 : Repository foundation

**Scope:** Go module and CI, Docker Compose services, configuration and logging, SQL migration runner, baseline ports and adapters, media fixture support, worker protocol sketch.

**Acceptance:** Existing repository foundation is present. Current checks are defined in CI and the Makefile.

### Phase 1 : Shared domain and wire contracts

**Scope:** Confirm IDs, job/task/attempt/result states, object:reference ownership, client and worker request/response payloads, and storage operations required by MVP. Align `docs/worker_protocol.md` with these contracts. Assign ownership for schema and shared:interface changes.

**Prerequisite:** Phase 0.

**Acceptance:** The contracts identify required fields and state transitions without choosing unresolved product policies. Contract tests or fixtures capture worker request/response shapes.

### Phase 2 : Client authentication and authorization

**Scope:** Client credential verification, JWT validation, and authorization middleware for user:owned resources. Record local token issuance/bootstrap behavior before exposing a login flow.

**Prerequisite:** Phase 1. Middleware can be developed and tested with fixed test tokens while token issuance policy is recorded.

**Acceptance:** Tests cover missing, invalid, expired, and valid credentials and cross:user resource access.

### Phase 3 : Job creation and idempotency

**Scope:** Persist a job and its requested server:supported output profile; support safe repeated creation using an idempotency key. Do not implement upload handling in this phase.

**Prerequisite:** Phase 1. Service and HTTP behavior can be built against a fake repository; PostgreSQL integration requires the agreed job schema.

**Acceptance:** Tests cover valid creation, invalid settings, duplicate idempotency keys, and user ownership.

### Phase 4 : Input object storage operations

**Scope:** Implement the storage adapter operations needed to create scoped upload permissions and inspect/read objects. Keep interfaces consumer:owned and limited to required operations.

**Prerequisite:** Phase 1. Adapter work can proceed independently of API and job code.

**Acceptance:** Local S3:compatible integration tests cover scoped upload access, object inspection/read, and error handling.

### Phase 5 : Upload URL issuance

**Scope:** Create a server:generated input object key and return a short:lived upload URL for an authorized job owner.

**Prerequisite:** Phases 3 and 4 contracts. Handler tests can use fake job and storage adapters before integration.

**Acceptance:** Tests verify authorization, key ownership, URL scope/expiry behavior, and that the API does not proxy video bytes.

### Phase 6 : Upload finalization and input integrity

**Scope:** Accept upload finalization metadata, verify the server:assigned object exists, check actual size and SHA:256 against the client claim, and transition the job to uploaded. Make repeated finalization safe.

**Prerequisite:** Phase 5 integration; implementation can use the Phase 1 object:reference contract.

**Acceptance:** Tests cover missing object, wrong key, size/checksum mismatch, duplicate finalize, and successful state transition.

### Phase 7 : FFprobe execution and metadata parsing

**Scope:** Run FFprobe as a bounded subprocess and parse its output into domain media metadata. This phase does not decide whether metadata is acceptable.

**Prerequisite:** Phase 0 fixture support and Phase 1 media contract. Independent of API/database implementation.

**Acceptance:** Tests cover valid output, malformed output, process errors, timeout, cancellation, and fixture media.

### Phase 8 : Input media acceptance policy

**Scope:** Apply the MVP container, codec, and stream:count requirements: MP4, H.264, AAC, one video stream, and one audio stream. Apply configured size and duration limits only after their values are recorded as policy.

**Prerequisite:** Phase 7 metadata contract. The pure policy function can be implemented independently of upload finalization.

**Acceptance:** Table:driven tests cover accepted profile, unsupported codecs/containers, stream:count mismatches, and configured bounds.

### Phase 9 : Whole:file task planning

**Scope:** For an accepted input, produce exactly one pending whole:file task. No segmentation or multi:worker planning is included.

**Prerequisite:** Phase 1 task contract. Planner behavior can be tested with synthetic metadata; persistence integration follows Phase 10.

**Acceptance:** Tests prove one task is created for eligible media and no task is created for rejected media.

### Phase 10 : Job, media, and task persistence

**Scope:** Persist job state transitions, inspected media metadata, and the whole:file task using the existing migration runner and pgx.

**Prerequisite:** Phases 3, 7, 8, and 9 contracts. Migration ownership and numbering must be coordinated; repository tests can use the existing PostgreSQL service.

**Acceptance:** PostgreSQL integration tests cover transaction behavior, constraints, and valid/invalid state transitions.

### Phase 11 : Worker enrollment and authentication

**Scope:** Enroll a worker using an enrollment token and issue a separate persistent worker credential stored in non:recoverable form. Worker identity is derived from authentication, not request fields.

**Prerequisite:** Phase 1 worker identity contract. Can proceed in parallel with client authentication and job intake.

**Acceptance:** Tests cover enrollment, credential verification, invalid/revoked credentials, and separation from client credentials.

### Phase 12 : Worker polling client and protocol handling

**Scope:** Implement worker:side authenticated HTTP calls and decoding for polling, no:work responses, task payloads, and API errors. Use standard `net/http`.

**Prerequisite:** Phase 1 protocol contract. Can be tested against a fake server before coordinator endpoints exist.

**Acceptance:** Protocol tests cover valid payloads, malformed responses, authentication failure, cancellation, and retryable errors.

### Phase 13 : Atomic task claim and attempt creation

**Scope:** Claim one eligible task transactionally, create a fresh attempt ID and lease, and set a unique server:generated output key for that attempt. Return task settings and task:scoped storage permissions.

**Prerequisite:** Phases 1, 10, and 11 contracts. Can be implemented and concurrency:tested before worker polling is integrated.

**Acceptance:** PostgreSQL tests prove concurrent pollers cannot claim the same task and each assignment has a unique attempt and output key.

### Phase 14 : Worker input download and output upload

**Scope:** Download task input and upload the produced object directly through scoped object:storage URLs. Submit metadata:not video bytes:to the API.

**Prerequisite:** Phases 4 and 12 contracts. Can use a fake coordinator and test objects while Phase 13 is in progress.

**Acceptance:** Worker tests cover transfer errors, retry behavior, cleanup of local temporary files, and use of only assigned URLs and keys.

### Phase 15 : Server:defined FFmpeg encoding

**Scope:** Map typed server:defined settings into FFmpeg arguments and run the encoder with process, time, disk, and resource controls. Do not accept arbitrary worker:supplied commands.

**Prerequisite:** Phase 1 task/profile contract and Phase 0 media fixture support. Independent of coordinator implementation.

**Acceptance:** Tests cover argument construction, successful encoding, invalid input, timeout, cancellation, and process failure.

### Phase 16 : Worker heartbeat and lease renewal

**Scope:** Renew a lease only for the authenticated worker's current, unexpired attempt. The API determines renewal duration; the worker cannot submit timestamps or lease values.

Initial trial configuration: heartbeat about every 20 seconds; lease duration near twice the estimated task duration plus two minutes of network grace; maximum three attempts. Keep these values configurable and provisional until measured and recorded in `docs/decisions.md`.

**Prerequisite:** Phase 13 attempt contract. Worker and coordinator portions can be developed separately against the protocol.

**Acceptance:** Tests cover valid renewal, wrong worker, stale attempt, expired attempt, and concurrent renewal/reassignment.

### Phase 17 : Lease expiry and bounded retry

**Scope:** Identify expired attempts, fence them from later writes, requeue eligible tasks, count attempts, and mark tasks terminal after the configured retry limit.

**Prerequisite:** Phases 13 and 16 state:transition contract. Reaper logic can be tested with an injected clock independently of worker execution.

**Acceptance:** Deterministic PostgreSQL tests cover expiry, reassignment, retry bounds, and coordinator restart recovery.

### Phase 18 : Result submission endpoint

**Scope:** Accept attempt ID and result metadata from an authenticated worker, record submission idempotently, and enqueue/invoke validation. Submission alone does not accept work or award credits.

**Prerequisite:** Phases 1, 11, and 13 contracts. Handler implementation can use a fake validator and repository.

**Acceptance:** Tests cover malformed metadata, duplicate identical submission, conflicting duplicate submission, wrong worker, and stale attempt.

### Phase 19 : Result attempt and object:key authorization

**Scope:** Verify the submitted attempt is current and belongs to the worker/task, and that the submitted key exactly matches the server:assigned key for that attempt.

**Prerequisite:** Phases 13 and 18 contracts. Independent of media:tool validation.

**Acceptance:** Tests reject stale attempts, wrong worker/task, unassigned keys, and attempts to overwrite another result.

### Phase 20 : Result object integrity verification

**Scope:** Read the assigned result object from storage, verify existence and actual size, recompute SHA:256, and compare with worker:reported metadata.

**Prerequisite:** Phases 4, 18, and 19 contracts. Can be tested with fake objects before full worker execution exists.

**Acceptance:** Tests cover missing, empty, truncated, wrong:size, and checksum:mismatch objects. Checksums are treated as byte:integrity checks, not proof of correct computation.

### Phase 21 : Result metadata and profile validation

**Scope:** Use FFprobe to validate output container, codec, stream counts, encoding profile, duration bounds, and timestamp sanity against the task contract.

**Prerequisite:** Phases 7, 8, and 20 contracts. Can be implemented using generated and malformed fixtures.

**Acceptance:** Tests cover each rejected media property and valid output metadata.

### Phase 22 : Full decode validation

**Scope:** Run a bounded decode:only FFmpeg pass over the exact uploaded result object. Discard decoded output; do not re:encode or create a replacement file.

**Prerequisite:** Phase 20 object access and Phase 21 result profile contract. Can be developed independently of the submission handler.

**Acceptance:** Tests cover successful full decode, corruption/truncation, timeout, resource limits, and cancellation.

### Phase 23 : Atomic result acceptance and rejection handling

**Scope:** Orchestrate result checks and atomically accept one valid result only if its attempt remains current. Rejected results follow the configured retry state transition; stale results cannot become authoritative.

**Prerequisite:** Phases 17:22 contracts. Validator components and transaction logic can be developed concurrently; integration requires all checks.

**Acceptance:** PostgreSQL tests cover concurrent submissions, duplicate requests, lease expiry during validation, competing attempts, and at:most:one accepted result.

### Phase 24 : Final output promotion and validation

**Scope:** For the single:task MVP, make the accepted worker output the job's final output through a recorded promotion/copy operation as required by storage semantics, then validate final:output availability and media before completion.

**Prerequisite:** Phase 23 acceptance contract. Can be implemented against fake accepted results and storage.

**Acceptance:** Tests prevent completion before accepted result and successful final:output validation; copy/promotion retries are idempotent.

### Phase 25 : Job completion state transition

**Scope:** Mark a job complete only after its required task result is accepted and the final output passes validation.

**Prerequisite:** Phase 24 contract. State:transition tests can run independently of download API work.

**Acceptance:** PostgreSQL tests reject premature completion and allow safe repeated completion requests.

### Phase 26 : Credit ledger persistence and idempotency

**Scope:** Persist immutable ledger entries with a stable unique settlement key and transactional balance/ledger behavior. Support audit, reversal, and adjustment records as defined by the ledger contract.

**Prerequisite:** Phase 1 ledger contract. Can proceed in parallel with job processing using synthetic accepted contributions.

**Acceptance:** PostgreSQL tests prove duplicate settlement cannot create a second normal award and ledger history remains auditable. Do not select or infer the reward formula in this phase.

### Phase 27 : Credit policy and reward activation

**Scope:** Record the product:approved formula and own:job reward policy, then connect eligible accepted contributions to the ledger. If policy remains undecided, keep reward issuance disabled while retaining ledger mechanics.

**Prerequisite:** Phase 26 and explicit decisions recorded in `docs/decisions.md`.

**Acceptance:** Tests prove workers cannot choose amounts, only accepted work is eligible, policy is applied server:side, and repeated settlement remains idempotent.

### Phase 28 : Client job status API

**Scope:** Return authorized job state and progress without exposing worker credentials, internal object paths, or another user's data.

**Prerequisite:** Phases 2, 3, and 25 contracts. Can be implemented against a fake job repository while completion is in progress.

**Acceptance:** Handler tests cover ownership, each externally visible job state, and missing jobs.

### Phase 29 : Final download URL

**Scope:** Return a short:lived download URL only for the authorized owner of a completed job.

**Prerequisite:** Phases 4, 25, and 28 contracts. Can be tested with fake storage and job repositories.

**Acceptance:** Tests reject incomplete jobs and unauthorized users and verify URL scope and expiry behavior.

### Phase 30 : Cleanup eligibility and reference protection

**Scope:** Select expired temporary:object records and prove they are not referenced by active tasks, attempts, validation, retries, assembly, or retention requirements.

**Prerequisite:** Phase 1 object lifecycle contract. Can proceed independently against fake repositories.

**Acceptance:** Tests prove active and retained objects are excluded from cleanup candidates.

### Phase 31 : Object deletion and cleanup retry

**Scope:** Delete eligible objects idempotently, persist deletion outcomes, and retry transient failures. Object deletion and URL expiration remain distinct operations.

**Prerequisite:** Phases 4 and 30 contracts; final:output retention policy must be recorded before final outputs are eligible.

**Acceptance:** Tests cover repeated deletion, storage errors, retry, and protection of referenced objects.

### Phase 32 : Storage/database reconciliation

**Scope:** Detect missing referenced objects and unreferenced stored objects; record recoverable discrepancies for safe repair or operator review.

**Prerequisite:** Phases 10, 20, 24, and 30 object/state contracts. Scanner logic can be developed against fakes before cleanup integration.

**Acceptance:** Tests identify each discrepancy type without deleting active or unclassified objects.

### Phase 33 : HTTP request deadlines and upload limits

**Scope:** Apply configured HTTP request deadlines and upload size limits. Do not introduce unsupported limits or deployment policy; values must be recorded as configuration or decisions.

**Prerequisite:** Relevant API and storage contracts. This phase can use test handlers and a fake storage adapter.

**Acceptance:** Tests cover configured request deadlines, upload limits, and cancellation.

### Phase 34 : Media process resource limits

**Scope:** Apply CPU/time, memory, disk, and process limits to FFprobe/FFmpeg execution, including termination on timeout or cancellation.

**Prerequisite:** Phases 7, 15, 21, and 22 process contracts. Limit enforcement can be tested with fixture processes before workflow integration.

**Acceptance:** Tests cover configured limits, timeout, cancellation, and child:process cleanup.

### Phase 35 : Secret:safe logging

**Scope:** Ensure application logs do not expose credentials, tokens, media contents, or presigned URLs. Include correlation identifiers where available.

**Prerequisite:** Logging fields and request/worker contracts from Phase 1.

**Acceptance:** Tests or reviewable log:capture checks prove sensitive values are redacted or omitted.

### Phase 36 : Coordinator restart recovery

**Scope:** Verify that in:flight jobs, task ownership, attempts, and cleanup work recover from PostgreSQL state after coordinator restart; no correctness:critical state depends on memory.

**Prerequisite:** Phases 10, 13, 17, 23, 25, and 30 state contracts. Recovery tests can be developed alongside those packages.

**Acceptance:** Restart tests resume or safely retry in:flight work without losing ownership or accepting stale results.

### Phase 37 : Database invariant checks

**Scope:** Add test:time queries or assertions for at:most:one accepted result, unique settlement keys, and protection of referenced objects.

**Prerequisite:** Phases 10, 23, 26, and 30 schema/state contracts.

**Acceptance:** Invariant checks run against PostgreSQL test data and detect deliberately introduced violations.

### Phase 38 : End:to:end MVP verification

**Scope:** Exercise client authentication, job creation, upload/finalize, inspection, task claim, worker encode/upload/submit, validation, acceptance, finalization, status/download, ledger gate, and cleanup.

**Prerequisite:** Integrate completed MVP phases. Test harness work may begin earlier against the Phase 1 contracts and fixtures.

**Acceptance:** CI runs an end:to:end path with the repository's PostgreSQL and S3:compatible services and FFmpeg fixture. The test verifies failure and retry cases relevant to the MVP, as well as successful completion.

## Post:MVP phases

### Phase 39 : Segmentation feasibility experiment

**Scope:** Test keyframe:aware cut points, overlap, AAC boundary behavior, and whether distributed encoding provides useful speedup. Record results and the selected approach before production implementation.

**Prerequisite:** Phase 38 whole:file MVP path. Experimentation may use generated media and a standalone prototype.

**Acceptance:** Documented experiment results and an explicit decision to proceed, revise, or defer segmentation.

### Phase 40 : Segment planner

**Scope:** Convert inspected media into keyframe:aware segments with limited overlap and whole:file fallback.

**Prerequisite:** Phase 39 decision.

**Acceptance:** Property and fixture tests prove boundaries cover the timeline without gaps and meet the selected keyframe rules.

### Phase 41 : Segment:aware worker execution

**Scope:** Execute an assigned segment range and produce output conforming to the selected overlap/trim contract.

**Prerequisite:** Phases 15 and 37 contracts. Worker changes can use synthetic segment tasks before multi:worker scheduling is integrated.

**Acceptance:** Segment fixture tests validate requested ranges, overlap handling, and output metadata.

### Phase 42 : Multi:worker scheduling

**Scope:** Make multiple segment tasks independently claimable and processable while preserving PostgreSQL ownership and attempt fencing.

**Prerequisite:** Phases 13, 17, 23, and 37.

**Acceptance:** Concurrent:worker tests prove unique claims and correct retry/accept behavior across segments.

### Phase 43 : Ordered segment assembly

**Scope:** Assemble accepted segment outputs in timeline order and apply the Phase 39 audio:boundary decision.

**Prerequisite:** Phases 37:39 and the selected audio strategy.

**Acceptance:** Tests cover out:of:order completion, missing/duplicate segments, duration, timestamps, stream continuity, and A/V synchronization.

### Phase 44 : Distributed:processing performance validation

**Scope:** Compare multi:worker wall:clock performance with the whole:file path over representative inputs and worker counts.

**Prerequisite:** Phase 43.

**Acceptance:** Benchmark results are documented and used to decide supported thresholds and fallback behavior.

## Policy decisions and phase gates

The following values and policies remain open in `docs/decisions.md`. They are not implementation defaults unless recorded there:

* Client token issuance/bootstrap details: required before exposing client login beyond local development (Phase 2).
* Maximum input size and duration: required before enabling corresponding upload and media acceptance limits (Phases 5 and 8).
* Credit formula and own:job reward policy: required before enabling normal reward issuance (Phase 27).
* Final:output retention and extension policy: required before including final outputs in cleanup (Phase 31).
* Lease multiplier, heartbeat interval, and retry bound: required before replacing provisional trial configuration in Phases 16 and 17.
* Redundant:validation percentage: required before enabling redundant validation; MVP does not require it.
* Supported worker operating systems and hardware acceleration: required before expanding worker support.
* Public internet launch timing: required before public deployment.
* PostgreSQL:only scheduling versus a queue adapter: required before adding dispatch optimization.
* Upload interruption behavior: reconcile the single:upload MVP with the resumable:upload statement in `docs/architecture.md`.
