# Development Guide

## Project context

This project is a crowdsourced distributed video-compression platform written in Go. A client submits a video and output settings. The service stores the input in S3-compatible object storage, inspects it with FFprobe, and selects either whole-file processing or timeline-based distributed processing. Contributor workers run controlled FFmpeg jobs, upload results, and receive non-monetary credits only after the result is accepted.

The system accounts for worker failures, delayed connections, duplicate execution, stale submissions, invalid media, malicious workers, storage failures, and cleanup races.

This project uses conventional FFmpeg-based transcoding. It does not implement a custom codec, blockchain, cryptocurrency, or general-purpose peer-to-peer network.

## System architecture

### Components

- **Client**: authenticates, creates jobs, uploads media, watches progress, views credits, and downloads completed outputs.
- **HTTPS API**: authenticates requests, authorizes operations, validates input, and exposes client and worker endpoints.
- **Coordinator**: manages job and task transitions, media planning, leases, retries, result acceptance, assembly, and credit eligibility.
- **PostgreSQL**: authoritative storage for users, workers, jobs, tasks, attempts, leases, results, credits, and cleanup records.
- **Object storage**: stores original videos, task inputs, worker outputs, assembled results, and final downloads.
- **Media inspector**: uses FFprobe to read containers, streams, codecs, duration, frame rate, resolution, and timestamps.
- **Segment planner**: selects whole-file or distributed processing and creates keyframe-aware timeline boundaries.
- **Task scheduler**: matches eligible tasks with compatible workers and manages claims, leases, retries, and concurrency.
- **Contributor worker**: maintains an outbound authenticated connection, claims tasks, runs local tools, sends heartbeats, and uploads results.
- **FFmpeg and FFprobe runner**: executes media tools with server-defined settings and resource limits.
- **Result validator**: checks uploaded objects, metadata, checksums, decoding, settings, timestamps, and stream compatibility.
- **Final assembler**: combines accepted outputs and validates the completed video.
- **Credit ledger**: records rewards, spending, reversals, and adjustments as auditable transactions.
- **Cleanup and reconciliation**: removes expired objects and repairs differences between database records and object storage.
- **Observability**: records structured logs, metrics, traces, and operational state without exposing sensitive data.

### Component relationships

- The client communicates with the HTTPS API over TLS.
- The client uploads large media directly to object storage using short-lived, object-specific URLs.
- The API uses PostgreSQL for durable application state.
- Workers initiate outbound communication with the API and do not require inbound connections.
- Workers do not receive PostgreSQL, Redis, or broad cloud credentials.
- PostgreSQL is authoritative for task ownership, attempt fencing, accepted results, job completion, and credit settlement.
- A queue may improve dispatch and delayed work, but queue delivery does not establish correctness.

## Processing lifecycle

1. The client authenticates and creates a job with output settings and an idempotency key.
2. The API creates the job record and returns a short-lived upload URL.
3. The client uploads the original video to object storage and finalizes the upload with the object key, size, and checksum.
4. The API verifies the object and records the upload state in PostgreSQL.
5. FFprobe inspects the media.
6. The coordinator selects whole-file processing or distributed timeline tasks.
7. Tasks are persisted with their processing settings and boundaries.
8. A compatible worker claims a task and receives an attempt ID, lease, and scoped storage URLs.
9. The worker downloads its input, runs FFmpeg, sends heartbeats, uploads its output, and submits result metadata.
10. The validator checks the output. A valid current attempt becomes the task's authoritative result.
11. The assembler combines accepted results and validates the final output.
12. The ledger settles eligible credits using an idempotent transaction.
13. The client receives a temporary download URL.
14. Cleanup removes temporary objects after their retention period and after active references have cleared.

## Reliability and correctness

- Every task execution has a unique attempt ID.
- Lease renewal and result submission include the current attempt ID.
- A stale or expired attempt cannot replace a newer attempt or an accepted result.
- Heartbeats renew only the currently fenced attempt.
- Lease expiration makes a task eligible for bounded retry; it does not prove worker fraud.
- Client, worker, validation, settlement, and cleanup operations handle safe retries.
- A task has at most one accepted authoritative result.
- A normal reward has at most one settlement entry for its accepted contribution.
- Database transactions and uniqueness constraints protect acceptance and settlement races.
- Coordinator restart recovery comes from PostgreSQL state rather than in-memory state.
- Queue messages are treated as delivery hints; missing or duplicate messages are recoverable from durable task state.
- A job reaches completed state only after all required task results and final-output validation succeed.
- Cleanup checks active tasks, retries, validation, assembly, and retention records before deletion.

## Media boundaries

The initial media profile is:

- MP4 container.
- H.264 video.
- AAC audio.
- One video stream and one audio stream.
- Server-defined encoding profiles.
- No arbitrary worker-supplied FFmpeg commands.

Compressed video cannot be safely partitioned by arbitrary byte ranges because frames may depend on earlier frames. Distributed tasks use timeline boundaries aligned with suitable keyframes and may include limited decode overlap. Short or unsuitable videos use whole-file processing. Final validation covers segment order, duration, timestamps, streams, synchronization, decoding, and output settings.

## Security and trust model

- Workers are untrusted and may be unreliable or malicious.
- User credentials, worker credentials, and task-attempt identities are separate.
- Object-storage permissions are short-lived and scoped to the assigned task.
- Client-provided IDs, paths, metadata, capabilities, and timing claims are validated server-side.
- FFmpeg arguments come from typed server-controlled settings rather than worker-supplied shell commands.
- Media files and metadata are treated as hostile input.
- Media processes use CPU, memory, disk, process, and timeout limits.
- Paths, object keys, uploads, and output metadata are checked for traversal, size, and authorization violations.
- Logs exclude video contents, credentials, tokens, and presigned URLs.
- Contributors can see plaintext media assigned to their machines; object authorization does not provide confidentiality from the processing worker.

## Credits and accounting

- Credits are non-monetary units in the initial system.
- Reward amounts come from server-side policy.
- Uploading an object, reporting elapsed time, or submitting metadata does not create a reward.
- Only an accepted result becomes reward-eligible.
- The ledger records awards, spending, reversals, and adjustments.
- Settlement keys and database uniqueness prevent duplicate awards.
- Workers cannot choose reward amounts.
- Cryptocurrency and real-money settlement are outside the initial system.

## Go coding standards

### Documentation and readability

- Run `gofmt` on every changed Go file.
- Keep imports organized and let tooling remove unused imports.
- Place an explanatory comment immediately above every new or modified function and method, including unexported functions.
- Start function and method comments with the exact function or method name.
- Describe behavior, inputs, outputs, side effects, and important failure conditions when relevant.
- Prefer clear names, small functions, focused packages, and explicit control flow.
- Keep comments useful; do not restate obvious syntax.

### Types, interfaces, and dependencies

- Use structs for domain data and concrete behavior.
- Define small interfaces at the consumer boundary when substitution or testing requires them.
- Avoid large interfaces that combine unrelated responsibilities.
- Avoid interfaces introduced only to hide a single concrete implementation without a testing or architectural reason.
- Use composition and delegation instead of inheritance-style designs.
- Inject infrastructure dependencies through constructors or explicit fields.
- Keep domain logic independent from HTTP, PostgreSQL, object storage, and FFmpeg adapters.
- Apply SOLID principles pragmatically; do not create speculative layers or abstractions.
- Keep serialization types separate from domain types when their contracts differ.

### Errors, context, and concurrency

- Return errors explicitly and add context with wrapping where useful.
- Use `errors.Is` and `errors.As` for classification.
- Reserve `panic` for unrecoverable programmer or startup invariants, not normal runtime failures.
- Pass `context.Context` as the first parameter when an operation can block, cancel, or access external systems.
- Do not store request contexts in long-lived structs.
- Give every goroutine a clear owner and shutdown path.
- Protect shared mutable state with the smallest appropriate synchronization boundary.
- Make channel ownership and closure explicit.
- Avoid hidden global mutable state.

## Testing standards

- Use table-driven tests when multiple inputs share the same behavior.
- Use subtests to name distinct scenarios.
- Keep tests deterministic; avoid arbitrary sleeps and timing-based assertions.
- Use `httptest` for HTTP handlers and client behavior.
- Use fakes at application boundaries and integration tests for real PostgreSQL or object-storage adapters.
- Test idempotency, duplicate requests, lost responses, and retry behavior.
- Test lease expiry, heartbeat races, stale attempts, duplicate submissions, and concurrent claims.
- Test invalid media, malformed metadata, invalid worker output, and final assembly failures.
- Test duplicate credit settlement and cleanup protection for referenced objects.
- Run the race detector for concurrent code.

Expected checks for Go changes:

```text
gofmt -w <changed-go-files>
go vet ./...
go test ./...
go test -race ./...
```

## Change and pull-request rules

- Keep each change focused on its requested behavior.
- Avoid unrelated refactors and generated files.
- Preserve existing behavior unless the task explicitly changes it.
- Update documentation when behavior, interfaces, or architecture changes.
- Use meaningful conventional commit prefixes such as `docs:`, `chore:`, `fix:`, and `feat:`.
- Do not add `Co-authored-by` trailers.
- Review the complete diff before committing.
- Do not merge pull requests unless explicitly requested.
- Documentation-only changes do not include application code.

## Open decisions

The following remain policy or product decisions rather than implementation assumptions:

- Maximum input size and duration.
- Exact lease multiplier and retry count.
- Final-output retention duration and extension policy.
- Redundant-validation percentage.
- Supported worker operating systems.
- Hardware acceleration.
- Credit formula.
- Public internet launch timing.
- Whether users can process their own jobs for credits.
- PostgreSQL-only scheduling versus a later queue adapter.
