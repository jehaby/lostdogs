Goal: repost selected posts to a VK target wall (our group/community) using a durable DB outbox to decouple scanning/storage from delivery. Ship text-only first; add media upload in a follow-up. No code here — just an implementable plan.

High‑Level Architecture

- Scanner stores posts in `posts` (already implemented).
- Enqueue phase: for matching posts, insert an item into a VK-specific outbox (`outbox_vk`).
- Worker: a goroutine periodically claims pending rows, posts to the configured VK wall, and updates status with idempotency and retries.

DB Schema (outbox_vk)

- Table: `outbox_vk`
  - `id INTEGER PRIMARY KEY AUTOINCREMENT`
  - `owner_id INTEGER NOT NULL` — origin VK owner_id (from scanned post)
  - `post_id INTEGER NOT NULL` — origin VK post_id (from scanned post)
  - `status TEXT NOT NULL CHECK (status IN ('pending','sending','sent','failed')) DEFAULT 'pending'`
  - `retries INTEGER NOT NULL DEFAULT 0`
  - `last_error TEXT`
  - `vk_post_id INTEGER` — ID of the created post on the destination wall (from `wall.post`)
  - `leased_until INTEGER` — unix seconds, lease for in‑flight processing
  - `created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `UNIQUE(owner_id, post_id)` — idempotent enqueue per origin post
- Indexes:
  - `idx_outbox_vk_status_created_at(status, created_at)`
  - `idx_outbox_vk_lease(status, leased_until)`

sqlc Queries (resources/db/queries.sql)

- `-- name: EnqueueOutboxVK :exec`
  - Insert or ignore on unique conflict: `(owner_id, post_id)`
- `-- name: ClaimPendingMarkVK :exec`
  - Mark up to `:limit` oldest as `sending`, set `leased_until=@lease`
- `-- name: ListSendingByLeaseVK :many`
  - Return claimed rows for current lease
- `-- name: MarkSentVK :exec`
  - `UPDATE outbox_vk SET status='sent', vk_post_id=@vk_post_id, updated_at=CURRENT_TIMESTAMP WHERE id=@id`
- `-- name: MarkFailedVK :exec`
  - Bump `retries`, reset lease, transition to `pending` or `failed` by `@max_retries`
- `-- name: ReapStaleVK :exec`
  - Return expired `sending` rows to `pending`

Config (env via caarlos0/env)

- `VK_OUT_ENABLED` (bool, default false)
- `VK_OUT_TOKEN` (string, optional; falls back to `VK_TOKEN` if empty). Needs `wall` scope and access to the destination group.
- `VK_OUT_OWNER_ID` (int64, required if enabled) — destination wall owner id; negative for groups (e.g., `-123456`).
- `VK_OUT_RATE_PER_SEC` (float, default 1.0) — post rate.
- `VK_OUT_HTTP_TIMEOUT` (duration, default 10s)
- `VK_OUT_FROM_GROUP` (bool, default true) — post as the group (`from_group=1`).
- Future: `VK_OUT_ATTACH_MEDIA` (bool) to gate media uploads.

Module Layout

- `internal/vk`
  - `client.go` — init `vkapi.VK` with token + timeout, hold `DestOwnerID` and `FromGroup` flags.
  - `formatter.go` — build message text (normalize, truncate, add source link).
  - `worker.go` — worker loop: claim → build payload → `wall.post` → mark sent/failed.
  - `enqueue.go` — `EnqueueIfMatchVK` (same filter as TG initially: only `TypeLost`).
- `cmd/lostdogs`
  - Wire-up: if `VK_OUT_ENABLED`, start the VK worker.

Enqueue Logic

- Location: after successful `UpsertPost` in `SaveMessage`.
- Filter: start with `TypeLost` only; ignore others for now.
- Call `EnqueueOutboxVK(owner_id, post_id)`; ignore unique conflicts.
- Do not block saving path on enqueue failures; log and continue.

Worker Loop

- Every `~1s / VK_OUT_RATE_PER_SEC`:
  1. `ReapStaleVK`.
  2. `ClaimPendingMarkVK(limit=BATCH, lease=now+LeaseTTL)`.
  3. For each row: load post via `GetPost`, build text, call `wall.post` with:
     - `owner_id=VK_OUT_OWNER_ID` (negative for group)
     - `from_group=1` if configured
     - `message` built by formatter
     - Phase 1: no attachments
  4. On success: `MarkSentVK(vk_post_id)`.
  5. On failure: map VK API errors → retries/backoff, `MarkFailedVK`.

Rate Limiting

- Default `1.0 msg/s` to be conservative; allow tuning via `VK_OUT_RATE_PER_SEC`.
- Add small jitter between posts to avoid bursts.

Message Formatting

- Similar to TG: title line based on type (🔎/✅/👀/🏠/💳), normalized/truncated body (~3000–3500 chars), and source link `https://vk.com/wall{owner_id}_{post_id}` at the end.
- No HTML/Markdown; plain text for `wall.post`.

Media Support (Phase 2)

- Upload flow per photo:
  1. `photos.getWallUploadServer(owner_id)`
  2. HTTP upload to returned URL
  3. `photos.saveWallPhoto(...)` → returns `photo{owner_id}_{id}`
  4. Pass attachments to `wall.post`
- Cap total photos to a small number (e.g., `VK_OUT_MAX_MEDIA`, default 5–10).

Error Handling & Idempotency

- Unique key prevents duplicate enqueue for a given origin post.
- Lease prevents double-processing; `ReapStaleVK` returns expired claims.
- Map common VK errors:
  - `6` (Too many requests): backoff and retry
  - `5/15/214` (auth/access): mark failed and surface error
  - `14` (Captcha): mark failed or park for manual handling

Observability

- Logs: claims, sends, failures; include origin `(owner_id,post_id)` and dest `VK_OUT_OWNER_ID`.
- Optional stats: pending/sent/failed counts; oldest pending age.

Testing Plan

- Unit: formatter (truncate, link), enqueue filter.
- Integration (hermetic): inject custom `http.RoundTripper` into VK client to mock `wall.post` and photo upload endpoints; simulate `error_code 6/14/5xx` and assert transitions.
- Outbox transitions and reaping.

Rollout Steps

1. Add `outbox_vk` table + indexes (new migration).
2. Extend `resources/db/queries.sql` with VK outbox queries and regenerate sqlc.
3. Add env/config; init VK client for output behind `VK_OUT_ENABLED`.
4. Implement enqueue after `UpsertPost`.
5. Implement worker with claim/send/mark/retry; ship text-only first.
6. Add tests (formatter, worker with mocked VK API).
7. Enable in staging with low rate; then prod.

Notes / Future

- Multi-destination routing: if/when needed, consider consolidating into a generic `deliveries` table with a `destination` column instead of parallel outboxes.
- Support edits/deletes (optional follow-ups).
- Per-rule routing (multiple target walls) — add `dst_owner_id` per row.
