# Worker protocol sketch (not implemented in Phase 0)

All endpoints require TLS outside local development and an authenticated worker credential (separate from client JWTs). The API derives worker identity from that credential, never from a request field. All JSON is versioned by `/v1`. Identifiers and object keys are opaque; never accept arbitrary FFmpeg flags.

- `POST /v1/worker/poll` with `{ "capabilities": { "video_codec": "h264", "audio_codec": "aac" } }`. Returns `204` if no eligible work, otherwise `200` with `{ "task_id": "...", "attempt_id": "...", "lease_expires_at": "RFC3339", "input_url": "short-lived scoped GET URL", "output_url": "short-lived scoped PUT URL", "profile": { "container": "mp4", "video_codec": "h264", "audio_codec": "aac" } }`. Claim and attempt creation must be one DB transaction.
- `POST /v1/worker/heartbeat` with `{ "attempt_id": "..." }`. Returns `200` with `{ "lease_expires_at": "RFC3339" }` only for the current, unexpired attempt; `409` for stale/expired attempts. No worker-supplied timestamps or lease duration.
- `POST /v1/worker/submit` with `{ "attempt_id": "...", "object_key": "...", "size": 123, "sha256": "hex checksum" }`. Returns `202` for a current attempt awaiting validation, `409` for stale/expired attempts; repeated identical submissions are idempotent. Server verifies the object belongs to this attempt and validates bytes before accepting one authoritative result. Upload alone never awards credits.

Errors use `{ "code": "stale_attempt", "message": "..." }` without leaking credentials or presigned URLs. Exact lease, retry, size, and policy parameters remain open decisions in `docs/decisions.md`.
