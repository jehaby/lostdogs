-- name: UpsertPost :exec
-- Insert or update a post with all parsed fields
INSERT INTO posts (
  owner_id,
  post_id,
  date,
  text,
  raw,
  type,
  animal,
  sex,
  name,
  location,
  "when",
  phones,
  contact_names,
  vk_accounts,
  photos,
  status_details
)
VALUES (
  @owner_id,
  @post_id,
  @date,
  @text,
  @raw,
  @type,
  @animal,
  @sex,
  @name,
  @location,
  @when,
  @phones,
  @contact_names,
  @vk_accounts,
  @photos,
  @status_details
)
ON CONFLICT(owner_id, post_id) DO UPDATE SET
  date = excluded.date,
  text = excluded.text,
  raw = excluded.raw,
  type = excluded.type,
  animal = excluded.animal,
  sex = excluded.sex,
  name = excluded.name,
  location = excluded.location,
  "when" = excluded."when",
  phones = excluded.phones,
  contact_names = excluded.contact_names,
  vk_accounts = excluded.vk_accounts,
  photos = excluded.photos,
  status_details = excluded.status_details;

-- name: ExistsPost :one
SELECT EXISTS(
  SELECT 1 FROM posts WHERE owner_id = ?1 AND post_id = ?2
);

-- Outbox queries

-- name: EnqueueOutbox :exec
INSERT INTO outbox (owner_id, post_id)
VALUES (@owner_id, @post_id)
ON CONFLICT(owner_id, post_id) DO NOTHING;

-- name: GetPost :one
SELECT owner_id, post_id, date, text, raw, type, animal, sex, name, location, "when",
       phones, contact_names, vk_accounts, status_details, created_at
FROM posts
WHERE owner_id = ?1 AND post_id = ?2;

-- name: ClaimPendingMark :exec
UPDATE outbox
SET status='sending', leased_until=@lease, updated_at=CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM outbox
  WHERE status='pending' AND (leased_until IS NULL OR leased_until < strftime('%s','now'))
  ORDER BY created_at ASC
  LIMIT @limit
);

-- name: ListSendingByLease :many
SELECT id, owner_id, post_id
FROM outbox
WHERE status='sending' AND leased_until=@lease
ORDER BY created_at ASC;

-- name: MarkSent :exec
UPDATE outbox
SET status='sent', tg_message_id=@tg_message_id, updated_at=CURRENT_TIMESTAMP
WHERE id=@id;

-- name: MarkFailed :exec
UPDATE outbox
SET status=CASE WHEN retries+1>=@max_retries THEN 'failed' ELSE 'pending' END,
    retries=retries+1,
    last_error=@last_error,
    leased_until=NULL,
    updated_at=CURRENT_TIMESTAMP
WHERE id=@id;

-- name: ReapStale :exec
UPDATE outbox
SET status='pending', leased_until=NULL, updated_at=CURRENT_TIMESTAMP
WHERE status='sending' AND leased_until < strftime('%s','now');
