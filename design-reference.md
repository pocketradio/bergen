# Bergen Distributed Video Compressor — Design Reference

This document captures the recommended architecture, workflows, invariants, and component tradeoffs for Bergen. The attached image was used only as a readability reference.

## 1. High-level architecture

```mermaid
flowchart LR
    C[Client]
    API[Go API / Coordinator]
    DB[(PostgreSQL<br/>Authoritative State)]
    OBJ[(S3-Compatible<br/>Object Storage)]
    PLAN[Media Inspector<br/>and Segment Planner]
    SCH[Task Scheduler<br/>DB-backed Claiming]
    W[Contributor Worker]
    FFMPEG[Worker-local<br/>FFmpeg / FFprobe]
    VAL[Result Validator]
    ASM[Final Assembler]
    LEDGER[Credit Ledger<br/>and Settlement]
    CLEAN[Cleanup and<br/>Reconciliation]
    OBS[Metrics, Logs<br/>and Tracing]
    Q[Optional Queue Adapter]

    C -->|HTTPS API| API
    API --> DB
    API -->|Presigned upload/download URLs| OBJ
    API --> PLAN
    PLAN --> DB
    PLAN --> OBJ
    DB --> SCH
    SCH -->|Authenticated polling / long-polling| W
    W -->|Task-scoped media access| OBJ
    W --> FFMPEG
    W -->|Heartbeat, progress, result| API
    API --> VAL
    VAL --> OBJ
    VAL --> DB
    DB --> ASM
    ASM --> OBJ
    ASM --> VAL
    ASM --> DB
    DB --> LEDGER
    API --> CLEAN
    CLEAN --> DB
    CLEAN --> OBJ
    API -.-> Q
    API --> OBS
    W --> OBS
    VAL --> OBS
    CLEAN --> OBS
```

PostgreSQL decides what is true: task ownership, current attempts, accepted results, job completion, and credit settlement. Object storage holds large data; workers process it; an optional queue only improves dispatch.

## 2. Job-processing workflow

```mermaid
flowchart TD
    A[Client submits job] --> B[Validate settings]
    B --> C[Create job and input object record]
    C --> D[Client uploads original to object storage]
    D --> E[Finalize upload]
    E --> F[FFprobe input]
    F --> G{Worth distributing?}
    G -->|No| H[Create one whole-file task]
    G -->|Yes| I[Create keyframe-aware timeline segments]
    I --> J[Add limited overlap where needed]
    H --> K[Persist tasks in PostgreSQL]
    J --> K
    K --> L[Worker claims task]
    L --> M[Create attempt and lease]
    M --> N[Worker downloads task input]
    N --> O[Worker runs FFmpeg]
    O --> P[Worker uploads output]
    P --> Q[Worker submits result]
    Q --> R[Validate result]
    R -->|Invalid| S[Reject and retry if allowed]
    R -->|Valid| T[Accept one authoritative result]
    S --> L
    T --> U{All tasks accepted?}
    U -->|No| L
    U -->|Yes| V[Assemble segments]
    V --> W[Validate final output]
    W -->|Invalid| X[Retry assembly or fail]
    W -->|Valid| Y[Settle eligible credits]
    Y --> Z[Mark job complete]
```

## 3. Lease and stale-attempt flow

```mermaid
sequenceDiagram
    participant W as Worker
    participant API as Coordinator
    participant DB as PostgreSQL
    participant OBJ as Object Storage
    participant V as Validator
    W->>API: Claim available task
    API->>DB: Atomically create attempt and lease
    DB-->>API: attempt_id and lease_expiry
    API-->>W: Task specification and scoped URLs
    OBJ-->>W: Download input
    loop Every 15–30 seconds
        W->>API: Heartbeat(attempt_id)
        API->>DB: Renew only if attempt is current
        DB-->>API: New lease expiry
        API-->>W: Renewal accepted or rejected
    end
    W->>OBJ: Upload output
    W->>API: Submit result(attempt_id, checksum, metadata)
    API->>DB: Verify current attempt
    API->>V: Validate uploaded output
    V-->>API: Valid or invalid
    alt Valid current attempt
        API->>DB: Accept result exactly once
        API->>DB: Record credit eligibility
    else Invalid result
        API->>DB: Reject and schedule bounded retry
    else Expired or stale attempt
        API->>DB: Reject stale submission
    end
```

Provisional defaults: heartbeat every 15–30 seconds; lease around twice the estimated task duration plus network grace; about three attempts; late results rejected if a newer attempt owns or completed the task.

## 4. Credit settlement

```mermaid
flowchart TD
    A[Task result submitted] --> B{Result accepted?}
    B -->|No| C[No normal reward]
    B -->|Yes| D[Create stable settlement key]
    D --> E{Settlement already exists?}
    E -->|Yes| F[Return existing outcome]
    E -->|No| G[Append ledger entry transactionally]
    G --> H[Update or derive balance]
    H --> I[Mark settlement complete]
    I --> J[Audit record]
    C --> J
    F --> J
```

Use a uniqueness rule such as `(task_id, accepted_result_id, reward_type)`. Repeated requests cannot create another reward.

## 5. Cleanup and retention

```mermaid
flowchart TD
    A[Scheduled cleanup scan] --> B[Find expired object records]
    B --> C{Still needed?}
    C -->|Yes| D[Defer and retry later]
    C -->|No| E[Delete object idempotently]
    E --> F{Deletion successful?}
    F -->|Yes| G[Mark object deleted]
    F -->|No| H[Record failure and retry with backoff]
    G --> I[Reconcile orphaned objects]
    H --> I
    D --> I
    I --> J[Metrics and operator alerts]
```

## Recommended MVP defaults

- MP4 container, H.264 video, AAC audio.
- Initially one video stream and one audio stream.
- Server-defined encoding parameters; no arbitrary worker FFmpeg commands.
- Worker enrollment token followed by persistent worker credentials.
- Outbound authenticated polling or long-polling.
- Short-lived, task-scoped object-storage permissions.
- Keyframe-aware segments with limited overlap where needed.
- Single-worker fallback for short or unsuitable videos.
- PostgreSQL-backed task claiming first; queue adapter later if measurements justify it.
- Attempt IDs required for heartbeats and result submission.
- At-most-once accepted result per task.
- At-most-once normal credit settlement per accepted task.
- Bounded retries and idempotent cleanup.

## Component analysis

### Client

Responsibility: submit jobs, upload videos, monitor status, download results, and view credits. It should use direct object-storage uploads for large files. It must support idempotency and resumable uploads. Its drawback is handling interruptions and delayed asynchronous work. It is recommended as a thin client because it keeps media transfer out of API handlers. Arbitrary FFmpeg command construction is out of scope.

### Go API/coordinator

Responsibility: authenticate requests, authorize operations, and coordinate state transitions. It exposes client and worker APIs and manages idempotency, leases, and policy. Its advantage is centralized correctness; its drawback is potential growth into a large service. It must never relay all video bytes. A single Go coordinator is recommended for the MVP; service splitting can wait for measured pressure.

### PostgreSQL

Responsibility: authoritative durable state for users, workers, jobs, tasks, attempts, leases, results, credits, and cleanup records. Transactions, uniqueness constraints, row locks, and compare-and-set updates enforce correctness. Its drawback is that very high-volume dispatch may eventually need support. It is recommended as the initial source of truth. Video bytes do not belong in database rows.

### Object storage

Responsibility: store originals, segments, worker outputs, assembled results, and final downloads. It provides scale and direct transfer, but introduces egress, orphan, multipart, and cleanup costs. Use short-lived object-specific permissions and never give workers broad credentials. S3-compatible storage is recommended; lifecycle rules can supplement application reconciliation later.

### Media inspector

Responsibility: use FFprobe to inspect containers, streams, codecs, duration, frame rate, and timestamps. It provides cheap early rejection, but metadata cannot prove honest computation. Run it with timeouts and isolation because media is hostile input. FFprobe behind a controlled interface is recommended.

### Segment planner

Responsibility: decide whether distribution is worthwhile and produce safe timeline boundaries. It should use keyframe-aware boundaries, limited overlap, and a whole-file fallback. Its main drawback is boundary complexity: gaps, duplication, and audio drift are possible. This hybrid strategy is recommended because distribution is not beneficial for every video. Arbitrary editing timelines are out of scope.

### Task scheduler

Responsibility: make eligible work claimable by compatible workers while enforcing concurrency, fairness, retries, and capability matching. Its drawback is inaccurate estimates and slow-worker tail latency. PostgreSQL-backed claiming is recommended first; Redis/Asynq can later improve dispatch without becoming authoritative.

### Contributor worker

Responsibility: authenticate, claim, process, upload, heartbeat, and report tasks. Outbound polling works behind NAT and tolerates intermittent connectivity. Workers are untrusted and may crash, lie, or be slow. Use persistent worker credentials, scoped URLs, local resource limits, and graceful shutdown. Workers must not access PostgreSQL, Redis, or broad cloud credentials.

### FFmpeg and FFprobe execution

Responsibility: perform encoding and inspection. FFmpeg is mature and practical but complex and security-sensitive. Construct arguments from server-defined typed settings, isolate processes, enforce CPU/memory/disk/time limits, and kill hangs. Custom codecs and arbitrary shell commands are out of scope.

### Result validator

Responsibility: check object existence, checksum, parsing, streams, duration, codec, settings, timestamps, and decoding. Layered validation keeps cheap checks early and deeper checks selective. It cannot detect every colluding or dishonest worker, but it greatly reduces obvious fraud and corruption. Rejected results receive no normal reward.

### Final assembler

Responsibility: combine accepted segments and verify order, timestamps, streams, duration, synchronization, and decoding. It introduces a serial phase and possible bottleneck, but centralizes final correctness. Only accepted, authorized objects may be assembled. A job cannot succeed before final validation.

### Credit ledger and settlement

Responsibility: record rewards, spending, reversals, and adjustments. Immutable ledger entries preserve audit history. Stable settlement keys and database uniqueness prevent duplicate awards. Workers cannot choose reward amounts, and failed work receives no normal reward. Cryptocurrency and real-money payments are out of scope.

### Cleanup and reconciliation

Responsibility: delete expired objects, retry failed deletions, remove abandoned multipart uploads, and reconcile storage/database drift. It must not delete objects still needed by active tasks, validation, retry, or assembly. URL expiry is not deletion. Scheduled cleanup plus explicit object lifecycle records is recommended.

### Observability

Responsibility: expose job latency, queue delay, retries, lease expiry, invalid results, credit settlements, storage usage, transfer volume, cleanup lag, and orphan counts. Structured logs should include correlation IDs but never media contents, credentials, or presigned URLs. This is essential for diagnosing stuck jobs and repeated failures.

## Explicitly unresolved decisions

- Maximum file size and duration.
- Exact lease multiplier and retry count after benchmarking.
- Final-output retention duration and extension policy.
- Redundant-validation percentage.
- Supported worker operating systems.
- Hardware acceleration.
- Exact credit formula.
- Public internet launch timing.
- Whether users may process their own jobs for credits.

These should be configuration or policy decisions, not hidden assumptions in the core correctness model.

