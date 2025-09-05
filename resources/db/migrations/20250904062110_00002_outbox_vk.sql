-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS outbox_vk (
  id             INTEGER     PRIMARY KEY AUTOINCREMENT,
  owner_id       INTEGER     NOT NULL,
  post_id        INTEGER     NOT NULL,
  status         TEXT        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed')),
  retries        INTEGER     NOT NULL DEFAULT 0,
  last_error     TEXT,
  vk_post_id     INTEGER,
  leased_until   INTEGER,
  created_at     TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at     TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(owner_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_outbox_vk_status_created_at ON outbox_vk(status, created_at);
CREATE INDEX IF NOT EXISTS idx_outbox_vk_lease ON outbox_vk(status, leased_until);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.
DROP TABLE IF EXISTS outbox_vk;

