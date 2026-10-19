# Phased Implementation Plan

## Guiding approach

**Build vertical slices, not layers.** The first goal is the whole 37-step happy path working end to end for a single worker and a whole-file job (the "single-worker fallback" from decisions.md). Real segmentation and multi-worker distribution come after that. This gets a demo-able system early, and everything after it is incremental.

**Put the strictest TDD where the invariants live.** The correctness invariants in decisions.md (stale attempts can't overwrite newer ones, one accepted result per task, one credit per contribution) are the hard part of this project. Test those first and hardest. Handlers, CLI glue, and config can be covered by end-to-end tests.

**Stay a modular monolith.** The happy path shows Coordinator, Validator, Assembler, Ledger, and Cleanup as separate boxes. In code they are packages behind interfaces inside one `server` binary, plus a separate `worker` binary. No microservices, no Redis/Asynq, and no hardware acceleration until the core works.

### Suggested stack

| Concern | Choice |
|---|---|
| HTTP | `net/http` + `chi` |
| DB access | `pgx` + `sqlc` (type-safe SQL that AI tools generate reliably) |
| Migrations | `goose` or `golang-migrate` |
| Object storage | S3-compatible emulator locally via docker-compose; AWS SDK v2 or `minio-go` behind a `Storage` interface |
| Auth | `golang-jwt` for clients; hashed enrollment and worker tokens |
| Testing | `testing` + `testify`, `testcontainers-go` (Postgres, LocalStack), golden files |
| Media | `os/exec` wrapping `ffmpeg`/`ffprobe`, behind `Prober` and `Encoder` interfaces |
| Logs | `log/slog` with correlation IDs |

### Repo layout

```
cmd/server      cmd/worker      cmd/vcctl (CLI client)
internal/
  api  auth  jobs  scheduler  attempts  planner
  validator  assembler  ledger  cleanup
  media (ffprobe/ffmpeg wrappers)  storage  store  config  obs
migrations/     testdata/ (tiny lavfi-generated clips)
```

### Test layers

1. **Unit tests:** pure state transitions, planner boundaries, FFmpeg argument builder (golden tests), validator checks.
2. **Integration tests:** real Postgres and MinIO via testcontainers, so no mocking of the database that owns the invariants.
3. **E2E tests:** the real `server` and `worker` binaries with real FFmpeg on 2–5 second generated clips.
4. **Fault-injection tests:** added in Phase 8.

### Working with AI assistance

- Write the test or spec first, then have the AI implement against it.
- Paste the relevant invariants from decisions.md into prompts for state-machine code.
- Treat AI-generated SQL and transaction code as the highest-review-priority code. Both of you should read every line of lease, claim, accept, and ledger logic.
- Let the AI generate boilerplate (handlers, sqlc queries, table-driven test cases) freely.

### Suggested split for two people

- **Person A (control plane):** schema, API, scheduler, attempts, ledger, cleanup.
- **Person B (media plane):** ffprobe/ffmpeg wrappers, worker binary, validator, planner, assembler.
- **Shared:** the interface contracts in Phase 0, the worker protocol in Phase 3, and review of each other's core-invariant code.

---

## Phase 0: Foundations

**Steps covered:** none directly.

**Build**
- Repo, Go module, CI (`go vet`, `golangci-lint`, `go test`), docker-compose (Postgres, MinIO), and a Makefile.
- Config loading, slog setup, and the migration runner.
- Interface definitions: `Storage`, `Prober`, `Encoder`, `Clock`, `IDGen`. Injectable clock and IDs make lease-expiry tests deterministic.
- A test-fixture helper that generates tiny MP4s (H.264/AAC) via ffmpeg `lavfi` sources.
- A one-page **worker protocol sketch** (endpoints and payloads for poll, heartbeat, submit) so both of you can work in parallel later.

**Done when:** `make test` runs green in CI with a real Postgres container and a trivial storage round-trip test against the local S3-compatible emulator.

---

## Phase 1: Auth, job creation, upload (steps 1–7)

**Build**
- Schema: `users`, `jobs`, with an explicit job state enum (e.g. `created → uploaded → inspecting → ...`).
- Client login and JWT middleware, then `POST /jobs`, which returns a job ID and a presigned upload URL (steps 1–3).
- Finalize endpoint: reads job and object details, verifies the object exists, and checks size and checksum (steps 5–7).
- Idempotency keys on job creation and finalize.
- `vcctl` CLI that logs in, creates a job, uploads, and finalizes. This is your main manual-testing tool from here on.

**TDD focus**
- Job state transitions as a pure table-driven test (valid and invalid transitions).
- Idempotent finalize: calling it twice yields one outcome.
- Checksum mismatch is rejected.
- Authorization: user A cannot touch user B's job.

**Done when:** `vcctl` can upload a real file and the job reaches `uploaded` with a verified object.

---

## Phase 2: Media inspection and whole-file task creation (steps 8–11)

**Build**
- `media.Prober`: runs ffprobe with a timeout and process isolation, then parses its JSON into a domain `MediaInfo`.
- MVP policy checks: MP4, H.264, AAC, exactly one video and one audio stream. Reject anything else with a clear error. Max size and duration are config values.
- Metadata persisted to the DB, plus the `tasks` table.
- **Planner v0:** always emits one whole-file task. Keep the planner behind an interface so Phase 7 swaps in the real one.
- The inspector is invoked synchronously from finalize at first. Make it a background step later if inspection proves slow.

**TDD focus**
- Parser tests against recorded ffprobe JSON fixtures (including malformed, multi-stream, and wrong-codec files).
- Timeout and hang handling using a fake `Prober`.

**Done when:** finalize on a valid file creates one `pending` task; an invalid file fails the job with a useful reason.

---

## Phase 3: Task claiming and attempt lifecycle (steps 12–14, 17, 19, 23)

**This is the core of the project. Spend disproportionate time and test effort here.**

**Build**
- Schema: `workers`, `attempts`, `leases`, `results`.
- Worker enrollment (enrollment token → persistent credential, stored hashed) and worker auth middleware.
- `POST /worker/poll`: claims a task using `SELECT ... FOR UPDATE SKIP LOCKED`, creates an attempt with a lease, and returns the task plus scoped presigned URLs (steps 12–14). Start with plain polling, since long-polling is an easy later upgrade.
- `POST /worker/heartbeat`, which requires an attempt ID and extends the lease (step 17).
- Lease-expiry reaper (a goroutine ticker) that returns expired tasks to `pending`, with bounded retries and a `failed` terminal state.
- `POST /worker/submit`, which requires an attempt ID and records the result as `submitted` (step 19).
- `Accept` operation (compare-and-set): only the current attempt of a non-accepted task can win (step 23).

**TDD focus (write these first)**
- Two concurrent pollers never claim the same task (run with `-race` and many goroutines).
- A stale attempt's heartbeat and submission are rejected after the task is re-leased.
- At most one accepted result per task, even with duplicate or concurrent submits.
- Lease expiry increments attempts and respects the retry bound.
- Use the injected `Clock` so no test sleeps.

**Done when:** a scripted fake worker can poll, heartbeat, and submit, and the invariant tests pass repeatedly under `-race -count=50`.

---

## Phase 4: Real worker (steps 15–19)

**Build**
- `cmd/worker`: enroll, poll loop with backoff and jitter, download input, run FFmpeg, heartbeat in a background goroutine, upload output, submit.
- `media.Encoder`: builds FFmpeg args from typed, server-defined settings only, with a CPU/time limit, a scratch directory per task, and kill-on-hang. Never accept raw args from the server payload.
- Graceful handling of cancellation, lost lease (heartbeat returns "stale", so abort), and cleanup of local temp files.

**TDD focus**
- Golden tests for argument construction.
- Worker loop tested against a fake coordinator and fake FFmpeg (a shell script that sleeps, fails, or hangs).
- Lost-lease behaviour: the worker stops work and doesn't submit.

**Done when:** a real worker process claims a task, encodes a real clip, and uploads output, leaving the result `submitted`.

---

## Phase 5: Result validator (steps 20–22, reused in 28–30)

**Build**
- Layered validator as a pipeline of checks, cheap ones first: object exists, checksum matches, ffprobe parses, stream layout, duration within tolerance, expected codec, timestamps sane, then a decode pass (`ffmpeg -v error -i out -f null -`).
- Wire it into the submit flow: submit → validate → accept or reject (a rejected result counts as a failed attempt).
- Make validation depth configurable. The redundant-validation percentage is an open decision, so expose it as a config value, default 0.

**TDD focus**
- A fixture set of "bad outputs": truncated file, wrong codec, wrong duration, silent-audio-removed, corrupt stream, zero bytes. Each should be rejected for the right reason.
- Checks stay independent and individually testable.

**Done when:** each fixture is accepted or rejected as expected, and a submitted result flows to `accepted` or a retry.

---

## Phase 6: Completion, credits, download, cleanup (steps 24–37). Milestone M1

**Build**
- **Assembler v0:** with one segment it is a verify-and-promote step (read the accepted output, write the final object, run final validation). Keep the interface general for Phase 7.
- Job completion: confirm all tasks accepted, then validate, then mark completed (steps 24–31).
- **Ledger:** immutable entries, a stable settlement key (e.g. `task_id + reward_type`) and a DB unique constraint for at-most-once awards. The reward comes from a simple server-side policy. Pick a placeholder formula such as credits proportional to the output seconds processed, since the exact formula is an open decision (steps 32–33).
- Download endpoint returns a short-lived presigned URL (steps 34–36).
- **Cleanup worker v0:** delete expired temp objects, with reference checks so it never deletes objects needed by active work. Make it idempotent and retry failed deletes (step 37).

**TDD focus**
- The double-award attempt fails at the DB constraint.
- Cleanup never touches objects referenced by non-terminal tasks.
- Job completion is blocked until all tasks are accepted and the final validation passes.

**Milestone M1:** all 37 happy-path steps run end to end with the real server, a real worker, and `vcctl`, as an automated E2E test. Tag a release.

---

## Phase 7: Segmentation and true distribution

**Build**
- **Planner v1:** pure function from `MediaInfo` to segments, using keyframe-aware boundaries, limited overlap where needed, and a whole-file fallback for short or unsuitable videos. Use ffprobe packet/keyframe data and unit-test it against many synthetic keyframe layouts.
- Segment-aware worker: the encoder takes a time range plus overlap trim.
- **Assembler v1:** concat accepted segments in order, then verify timestamps, stream continuity, duration, and A/V sync before final validation.
- Run multiple workers concurrently in the E2E test and in docker-compose.

**Risk to spike first (a half-day experiment before committing):** audio across segment boundaries. AAC encoder priming can cause glitches or drift when segments are concatenated. Options: (a) encode audio in a separate whole-file task and mux in assembly, or (b) trim priming samples per segment. Option (a) is simpler, and I'd start there. This is a design choice not covered in your docs, so decide it deliberately.

**TDD focus**
- Planner property tests: segments are contiguous, cover the full duration, and start on keyframes.
- Assembly of N segments equals the single-pass duration within tolerance.
- A segment finishing out of order doesn't break assembly.

**Done when:** a longer clip split across 3+ workers assembles into a video that passes final validation, and a benchmark shows wall-clock speedup versus a single worker.

---

## Phase 8: Resilience and hardening-lite

Not production grade, but enough to trust the demo.

**Build**
- **Fault-injection suite:** worker crashes mid-encode, worker hangs, worker submits garbage, duplicate submission, slow worker, network partition on heartbeat, server restart mid-job.
- Resumable uploads or multipart handling, plus abandoned-multipart cleanup.
- Reconciliation job for storage/DB drift (orphan objects, missing objects).
- Basic rate limiting, size limits, and request timeouts.
- Observability: Prometheus metrics for job latency, lease expiries, invalid results, retries, and cleanup lag. Make sure logs exclude presigned URLs and credentials.
- Use benchmarks to settle the open decisions that need data: lease multiplier, retry count, and redundant-validation percentage.

**Done when:** the fault-injection suite is green and a chaos-style run of dozens of jobs with randomly killed workers completes with no invariant violations. Add a DB-level invariant checker query (e.g. "no task with more than one accepted result", "no duplicate ledger keys") and run it at the end of those tests.

---

## Phase 9: Stretch (only if time allows)

- Simple web UI or richer CLI (progress, credits display).
- Long-polling for workers.
- Queue adapter (Redis/Asynq) as a dispatch optimization only, never authoritative.
- Docs, architecture diagrams, and a demo script.
- Explicitly out of scope unless priorities change: hardware acceleration, public internet launch, real-money or crypto payments, custom codecs, and splitting into services.

---

## Rough sizing

These are indicative only, assuming part-time work and heavy AI assistance.

| Phase | Size | Rough effort |
|---|---|---|
| 0 Foundations | S | 2–4 days |
| 1 Auth/upload | M | ~1 week |
| 2 Inspection | S–M | 3–5 days |
| 3 Claim/attempts | L | 1–2 weeks |
| 4 Real worker | M | ~1 week |
| 5 Validator | M | ~1 week |
| 6 Completion/credits/cleanup | M–L | 1–2 weeks |
| 7 Segmentation | L | 1–2 weeks |
| 8 Hardening | M | 1–2 weeks |

**Roughly 8–12 weeks to the end of Phase 8.** The Phase 6 milestone (M1) lands around the halfway mark.

## When to resolve the open decisions

| Open decision | Resolve by |
|---|---|
| Max file size/duration | Phase 2 (config default, e.g. 500 MB / 30 min) |
| Credit formula | Phase 6 (placeholder), revisit after Phase 7 |
| Lease multiplier, retry count | Phase 8 (after benchmarks) |
| Redundant-validation % | Phase 5 (configurable, default 0), tune in Phase 8 |
| Retention duration | Phase 6 (config) |
| Users processing their own jobs | Phase 6 (a one-line policy check in the ledger) |
| Worker OS support, HW accel, public launch | Defer past this plan |

## Top risks

1. **Concurrency bugs in claim/lease/accept.** Mitigated by Phase 3 being test-first, using `-race`, and a DB-level invariant checker.
2. **Segment boundary and A/V sync problems.** Mitigated by getting M1 first, spiking the audio approach, and keeping the whole-file fallback always available.
3. **AI-generated code that looks right but violates an invariant.** Mitigated by human review of all state-transition and ledger code, plus invariant tests that don't depend on implementation details.
4. **Scope creep toward production concerns.** Mitigated by decisions.md: when in doubt, keep it a monolith, keep it configurable, and defer it.
