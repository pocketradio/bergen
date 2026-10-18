# Workflows

## Job-processing workflow

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

## Lease and stale-attempt flow

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

Initial timing values are heartbeat intervals of 15–30 seconds, a lease near twice the estimated task duration plus network grace, and about three attempts. A result from an attempt replaced by a newer owner is stale and is rejected.

## Credit settlement

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

The settlement key can use `(task_id, accepted_result_id, reward_type)`. Repeated requests resolve to the existing ledger outcome rather than creating another reward.

## Cleanup and retention

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

URL expiration and object deletion are separate operations. A presigned URL expiring does not delete an object.
