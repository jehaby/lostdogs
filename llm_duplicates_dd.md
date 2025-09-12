**Context**
- Multiple VK source groups can publish the same real‑world case (lost pet). Our scanner saves each origin post separately (`posts` keyed by `(owner_id, post_id)`), and both Telegram and VK workers repost them. This creates destination duplicates (same case, different origin posts).
- Current pipeline: `SaveMessage(...)` → `UpsertPost` → enqueue to per‑destination outbox (TG: `outbox`, VK: `outbox_vk`) for `TypeLost` only → workers send in order.

**Goal**
- Prevent duplicate postings of the same case to a destination (TG chat and/or VK wall) while keeping the first occurrence. Keep changes minimal, robust, and explainable. Allow tuned time‑window to permit legitimate re‑announcements later.

**Key Signals Available**
- Parsed features from domain parser (`domain.go`): `Type`, `Animal`, `Sex`, `Phones` (E.164), `ContactNames`, `Location`, `When`, `VKAccounts`, normalized `Text`.
- Timestamps (`date`), per‑origin identifiers `(owner_id, post_id)`, and collected photo URLs (already stored).

**Baseline Proposal: Content Fingerprint + Sent Index (per destination)**

- Overview: derive a deterministic fingerprint for a post’s content; remember what the destination already sent by that fingerprint; gate enqueueing by consulting this “sent index” within a configurable TTL.

- Fingerprint (fp) design (phase 1: pragmatic heuristics)
  - Primary key part: sorted, deduped `Phones` joined with `,` (e.g., `+79991234567,+79001234567`). If ≥1 phone is present, phones dominate the fingerprint.
  - Secondary key part: compact text signature built from normalized `Text`:
    - Lowercase; strip VK mentions/URLs; collapse whitespace; drop emojis/punctuation except letters/digits/spaces.
    - Keep first N runes (e.g., 300–500) to reduce noise; split into tokens; sort unique tokens; join; SHA1 → 40‑hex.
  - Optional attributes appended to reduce collisions: `Animal`, `Sex`, and coarse `Location` (heuristic extractor already exists). Example raw fp input: `phones=+7999...,+7900...|animal=dog|sex=m|loc=ижевск|t=sha1:deadbeef…` → final `SHA1` of the whole string for storage.
  - Fallback when no phone numbers: rely on the text signature + `Animal|Sex|Location` only.

- Data model changes
  - Add `posts.fingerprint TEXT` with index `idx_posts_fingerprint(fingerprint)`.
  - Add per‑destination sent index to remember what was delivered recently:
    - Option A (unified): `deliveries (id, dest_kind TEXT, dest_id INTEGER, fingerprint TEXT, first_owner_id INTEGER, first_post_id INTEGER, message_id INTEGER NULL, first_sent_at TIMESTAMP, last_seen_at TIMESTAMP, count INTEGER DEFAULT 1, UNIQUE(dest_kind, dest_id, fingerprint))`.
    - Option B (simple split): `sent_tg_index(dest_chat_id, fingerprint, tg_message_id, first_owner_id, first_post_id, first_sent_at, last_seen_at, count)` and `sent_vk_index(dest_owner_id, fingerprint, vk_post_id, ...)` with `UNIQUE(dest_*, fingerprint)`.
  - Add indexes on `(dest_*, fingerprint)` and on `first_sent_at` for TTL cleanup.

- Enqueue gating (where to plug in)
  - Compute `fp` once when saving a post (inside `SaveMessage`) and persist to `posts.fingerprint`.
  - Before enqueuing to TG/VK (`internal/telegram.EnqueueIfMatch`, `internal/vk.EnqueueIfMatchVK`):
    1) If `DEDUP_ENABLED=true` and `p.Type == lost`, load `fp` for this `(owner_id, post_id)`.
    2) Check sent index for this destination: exists row where `fingerprint=fp` and `first_sent_at >= now - DEDUP_WINDOW` → skip enqueue.
    3) Also guard in‑flight duplicates: check an existing enqueued item with the same `fp` and status in (`pending`,`sending`) to avoid a race (via join to `outbox*/posts` or by storing `fingerprint` in outbox rows). If found → skip enqueue.
    4) If neither found → proceed with enqueue.
  - Skipped duplicates can increment `count` in the sent index (optional) or update a light `duplicates` counter table for observability.

- Worker integration
  - On successful send, upsert the sent index with `(dest_kind, dest_id, fingerprint)` and store message identifier (`tg_message_id` or `vk_post_id`) and `first_sent_at` if not set; update `last_seen_at`, bump `count`.
  - This lets later arrivals with the same `fp` be skipped without waiting for worker completion (because enqueue also checks in‑flight rows).

- TTL and cleanup
  - Add `DEDUP_WINDOW` (default: `72h`). Only suppress duplicates if the first send is within this window.
  - Periodic cleanup task (or during startup) removes sent index rows older than `DEDUP_WINDOW` to limit growth.

- Pros
  - Minimal invasive change; leverages existing parsing; deterministic and explainable; per‑destination behavior supported.
  - Works for both TG and VK with the same mechanism.

- Cons
  - Not true semantic dedupe; relies on phone/text heuristics. Edge cases without phones may slip through or over‑suppress if text is extremely generic.

**Alternative: Incident Clustering (Cases) – bigger, richer**
- Create `cases` table grouping origin posts into one case keyed by fingerprint; link `posts.case_id`.
- Destination delivery references `case_id` instead of `(owner_id, post_id)`. A case is delivered once per destination per window; later origin posts attach to the same case.
- Pros: natural home for future “updates” (e.g., “FOUND” follow‑up); enables editing/updating previous destination messages.
- Cons: more schema and logic, heavier rollout; not necessary for a first pass.

**Minimal Fast‑Track (if we want a very small patch first)**
- Skip new tables initially; only add `posts.fingerprint` and a quick EXISTS‑check at enqueue:
  - Before enqueue, check if there exists any `outbox`/`outbox_vk` row with status in (`pending`,`sending`,`sent`) whose referenced post has the same `fingerprint` and whose `updated_at/created_at` is within `DEDUP_WINDOW`. If yes → skip.
  - Pros: smallest change; no new tables.
  - Cons: Requires joins on `posts` every enqueue; “sent window” for `outbox` rows must be derived from timestamps there; visibility across restarts is fine but lacks the exact message id and longer‑term stats.

**Edge Cases & Rules**
- Different types: do not dedupe across fundamentally different `Type` (e.g., `lost` vs `found`). A later `found` with the same phones should likely be allowed (it’s an update), or handled by the “cases” approach.
- Zero phones: use text+attributes fp; raise threshold conservatism (e.g., require `Location` to be present) to avoid over‑suppressing generic reposts.
- Large edits: Post variants with minor edits will still match the fp; acceptable for near‑term goals.
- Time decay: allow reposting after `DEDUP_WINDOW` (e.g., 3–7 days) to keep visibility for long‑running cases.

**Configuration**
- `DEDUP_ENABLED` (bool, default `true`).
- `DEDUP_WINDOW` (duration, default `72h`).
- `DEDUP_STRICT_NO_PHONE` (bool, default `false`) — when no phones present, require both `Animal` and `Location` to include in fp.

**Implementation Outline (incremental)**
1) Fingerprint
   - Add `Fingerprint(p Post, normalizedText string) string` in `lostdogs` package; reuse existing helpers (`extractVKAccounts`, `extractPhones`, location heuristics). Include unit tests for Cyrillic texts.
   - Generate and store `posts.fingerprint` during `SaveMessage(...)`.

2) Schema
   - Migration: add `posts.fingerprint TEXT` + `CREATE INDEX idx_posts_fingerprint ON posts(fingerprint)`.
   - Optionally add `deliveries` (unified) or split per destination tables (recommended but can follow in phase 2).

3) Enqueue gating
   - Load `fp` for current `(owner_id, post_id)`; if empty, compute on the fly.
   - If `deliveries` exists: check `(dest_kind, dest_id, fp, first_sent_at >= now-DEDUP_WINDOW)`.
   - Always check in‑flight: `EXISTS SELECT 1 FROM outbox* o JOIN posts p ON o.owner_id=p.owner_id AND o.post_id=p.post_id WHERE p.fingerprint=? AND o.status IN ('pending','sending')` for the specific destination.

4) Worker update
   - After successful send, upsert `deliveries` for `(dest, fp)` with `message_id` and timestamps.

5) Cleanup
   - Add a lightweight periodic cleanup (or on startup) to delete old `deliveries` rows outside the window.

6) Tests
   - Table‑driven tests for `Fingerprint` covering: same content across groups, minor whitespace/punctuation changes, with/without phones, Cyrillic names, typical noise.
   - Integration tests for enqueue gating using in‑memory SQLite: insert two posts with the same fp → assert only one outbox row created; simulate worker send → assert later duplicates are suppressed within window.

**Trade‑offs & Future Upgrades**
- Add optional fuzzy layer (MinHash/SimHash or trigram Jaccard) if text‑only duplicates remain frequent without phones, but keep it behind a flag for performance.
- Incident “cases” would let us post an update (e.g., FOUND) by editing or commenting on the earlier destination message; can be added once baseline dedupe is stable.
- Media‑based signatures (perceptual image hashes) are possible in a later phase if text is too noisy (out of scope for now).

**Summary**
- Start with a deterministic fingerprint and a per‑destination sent index (or a minimal join‑based gate). This prevents cross‑group duplicates reliably, keeps the first occurrence, and provides clear knobs (TTL) and observability. The approach fits cleanly into current `SaveMessage` + enqueue + worker flow with modest schema additions and focused changes in `internal/telegram` and `internal/vk`.

