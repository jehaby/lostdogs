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
  status_details = excluded.status_details;

-- name: ExistsPost :one
SELECT EXISTS(
  SELECT 1 FROM posts WHERE owner_id = ?1 AND post_id = ?2
);
