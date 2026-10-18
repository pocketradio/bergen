# Decisions and Defaults

## MVP defaults

- MP4 container, H.264 video, AAC audio.
- Initially one video stream and one audio stream.
- Server-defined encoding parameters; worker-supplied FFmpeg commands are not part of the interface.
- Worker enrollment token followed by persistent worker credentials.
- Outbound authenticated polling or long-polling.
- Short-lived, task-scoped object-storage permissions.
- Keyframe-aware segments with limited overlap where needed.
- Single-worker fallback for short or unsuitable videos.
- PostgreSQL-backed task claiming first; a queue adapter remains a later dispatch option.
- Attempt IDs required for heartbeats and result submission.
- At-most-once accepted result per task.
- At-most-once normal credit settlement per accepted task.
- Bounded retries and idempotent cleanup.

## Correctness invariants

- PostgreSQL is authoritative for task acceptance and credit settlement.
- A stale attempt cannot overwrite a newer attempt.
- A task has at most one accepted authoritative result.
- A contribution produces at most one normal credit award.
- Queue delivery is not treated as exactly-once.
- Reward and processing parameters come from server-side policy rather than worker input.
- Job completion follows acceptance of all required task results and final validation.
- Cleanup excludes objects referenced by active work.

## Open decisions

- Maximum file size and duration.
- Exact lease multiplier and retry count after benchmarking.
- Final-output retention duration and extension policy.
- Redundant-validation percentage.
- Supported worker operating systems.
- Hardware acceleration.
- Exact credit formula.
- Public internet launch timing.
- Whether users may process their own jobs for credits.

These remain configuration and policy choices rather than fixed properties of the core state model.
