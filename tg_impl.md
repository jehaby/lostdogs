Goal: repost selected VK wall posts to a Telegram group, using a durable DB outbox to decouple scanning/storage from delivery. Start with a simple filter (type=lost), clean message formatting, and conservative rate limiting. No code here — just an implementable plan.

High‑Level Architecture
- Scanner parses and stores posts (already implemented).
- Enqueue phase: for matching posts, insert an item into an Outbox table (unique per `(owner_id, post_id)`).
- Worker: a goroutine periodically claims pending outbox rows, sends to Telegram, and updates status with idempotency and retries.

DB Schema (Outbox)
- Table: `outbox`
  - `id INTEGER PRIMARY KEY AUTOINCREMENT`
  - `owner_id INTEGER NOT NULL`
  - `post_id INTEGER NOT NULL`
  - `status TEXT NOT NULL CHECK (status IN ('pending','sending','sent','failed')) DEFAULT 'pending'`
  - `retries INTEGER NOT NULL DEFAULT 0`
  - `last_error TEXT`
  - `tg_message_id INTEGER` — Telegram message id after success
  - `leased_until INTEGER` — unix seconds, lease for in‑flight processing
  - `created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP`
  - `UNIQUE(owner_id, post_id)` — ensures idempotent enqueue
- Indexes:
  - `idx_outbox_status_created_at(status, created_at)`
  - `idx_outbox_lease(status, leased_until)`

sqlc Queries (resources/db/queries.sql)
- `-- name: EnqueueOutbox :exec` — insert or ignore on unique conflict
  - Inputs: `owner_id, post_id`
- `-- name: ClaimPending :many`
  - Atomically claim up to `:limit` oldest rows: update `status='sending', leased_until=now+lease`, return claimed rows
  - For SQLite, emulate claim with a transaction:
    - `UPDATE outbox SET status='sending', leased_until=@lease, updated_at=CURRENT_TIMESTAMP
       WHERE id IN (SELECT id FROM outbox WHERE status='pending' AND (leased_until IS NULL OR leased_until < strftime('%s','now')) ORDER BY created_at LIMIT @limit);`
    - `SELECT id, owner_id, post_id FROM outbox WHERE status='sending' AND leased_until=@lease;`
- `-- name: MarkSent :exec` — `UPDATE outbox SET status='sent', tg_message_id=@tg_id, updated_at=CURRENT_TIMESTAMP WHERE id=@id;`
- `-- name: MarkFailed :exec` — `UPDATE outbox SET status=CASE WHEN retries+1>=@max THEN 'failed' ELSE 'pending' END, retries=retries+1, last_error=@err, leased_until=NULL, updated_at=CURRENT_TIMESTAMP WHERE id=@id;`
- `-- name: ReapStale :exec` — return stuck rows to pending: `UPDATE outbox SET status='pending', leased_until=NULL WHERE status='sending' AND leased_until < strftime('%s','now');`
- `-- name: PendingStats :one` — counts for metrics (optional now).

Config (env via caarlos0/env)
- `TG_ENABLED` (bool, default false)
- `TG_TOKEN` (required if enabled)
- `TG_CHAT` (int64, required if enabled)
- `TG_PARSE_MODE` (string: `HTML` or `MarkdownV2`, default `HTML`)
- `TG_RATE_PER_SEC` (float, default 1.0)
- `TG_MAX_MEDIA` (int, default 10)
- `TG_HTTP_TIMEOUT` (duration, default 10s)

Module Layout
- `internal/telegram` (new package)
  - `client.go` — init `github.com/go-telegram/bot` client from env
  - `formatter.go` — build text/caption with proper escaping, truncation, link
  - `worker.go` — worker loop, rate limiting, retries, outbox transitions
  - `enqueue.go` — enqueue API called from `service.SaveMessage` (or similar)
- `cmd/lostdogs`
  - wire-up only: read env, construct `telegram.Client` and start `telegram.Worker` when `TG_ENABLED` is true

Enqueue Logic
- Where: inside `SaveMessage` (after `UpsertPost`) or right after save in `processPosts`.
  - Prefer `SaveMessage` to centralize logic for any future ingestion paths.
- Filter: `p.Type == TypeLost` initially; make it configurable later.
- Call `EnqueueOutbox(owner_id, post_id)`; ignore unique conflict.
- Do not block save path on enqueue failures; log and continue.

Worker Loop
- Startup: if `TG_ENABLED`, initialize Telegram bot and start a goroutine on service creation.
- Loop steps (every ~250ms or on demand):
  1) `ReapStale` to unlock orphaned leases.
  2) `ClaimPending(limit=BATCH)` with lease (e.g., 30s).
  3) For each claimed row:
     - Build message payload from DB record of the post (join by owner_id, post_id).
     - Send to Telegram respecting rate limit.
     - On success: `MarkSent(id, tg_message_id)`.
     - On failure:
       - If 429: parse `Retry-After` (if available), sleep that duration; `MarkFailed` with backoff info.
       - If 5xx/network: exponential backoff via `retries` and requeue to `pending`.
       - If 4xx permanent (e.g., chat forbidden): `MarkFailed` and stop worker with error.

Rate Limiting
- Token bucket: `TG_RATE_PER_SEC` messages per second (start with 1.0).
- Add jitter (±100ms) to avoid bursts.

Message Formatting
- Parse mode: `HTML` (simpler escaping). Provide `escapeHTML` util.
- Structure:
  - First line: icon + type, e.g., `🔎 Пропал питомец`
  - Body: normalized/truncated text (<= 3500 chars to be safe)
  - Phones and location (if available)
  - VK link: `https://vk.com/wall{owner_id}_{post_id}`
- Media (phase 1): text‑only posts to ship quickly.
- Media (phase 2, optional):
  - If/when attachments stored, send `sendMediaGroup` with up to `TG_MAX_MEDIA` photos.
  - Caption only on the first media (<=1024 chars). If too long, fallback to a plain message + album.
  - Use full VK CDN URLs including query string.

Error Handling & Idempotency
- Outbox unique key prevents duplicate enqueue.
- Worker marks rows `sending` with a `leased_until` to avoid double processing.
- On crash, `ReapStale` returns expired `sending` rows to `pending`.
- Store `tg_message_id` and skip re‑send if already set.

Observability
- Logs: enqueue decisions, claims, sends, failures (message truncated to avoid PII), counts per tick.
- Stats (optional): pending/sent/failed counts; oldest pending age.

Testing Plan
- Unit tests:
  - Formatter: escaping, truncation, link inclusion, caption building.
  - Enqueue filter: only lost posts queued.
- Integration tests (hermetic):
  - Mock Telegram API using a custom `http.RoundTripper` injected into the bot client.
  - Simulate 429/5xx; assert retries, backoff, and final statuses (`sent`/`failed`).
  - Outbox transitions: `pending -> sending -> sent`, reaping stale leases, idempotent enqueue.

Rollout Steps
1) Add `outbox` table + indexes to `resources/db/schema.sql`.
2) Extend `resources/db/queries.sql` with queries above and run `sqlc generate`.
3) Add config/env fields; init Telegram client behind `TG_ENABLED`.
4) Implement enqueue after `UpsertPost` for `TypeLost`.
5) Implement worker with claim/send/mark/retry.
6) Add tests (formatter, worker with mocked Telegram).
7) Ship with `TG_ENABLED=false` by default; enable in staging, then prod.

Notes / Future Work
- Extend filters (animal, regex, has phones, etc.).
- Support edits/deletes: optional follow‑up messages or ignore.
- Support multiple chats/rules: add `chat_id` to outbox rows; route per rule.
- Metrics endpoint (Prometheus) if needed.
