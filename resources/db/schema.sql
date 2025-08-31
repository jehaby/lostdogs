-- SQLite schema for lostdogs

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS posts (
  owner_id        INTEGER   NOT NULL,
  post_id         INTEGER   NOT NULL,
  date            INTEGER   NOT NULL, -- Unix timestamp (seconds)
  text            TEXT      NOT NULL,
  -- additional parsed/annotated data
  raw             TEXT      NOT NULL,                -- original raw content
  -- constrained enum-like fields
  type            TEXT      NOT NULL DEFAULT 'unknown' CHECK (type IN ('unknown','lost','found','sighting','adoption','fundraising','news','link','empty')),
  animal          TEXT      NOT NULL DEFAULT 'unknown' CHECK (animal IN ('unknown','cat','dog','other')),
  sex             TEXT      NOT NULL DEFAULT 'unknown' CHECK (sex IN ('unknown','m','f')),
  -- free-form/nullable annotations
  name            TEXT                              DEFAULT NULL,
  location        TEXT                              DEFAULT NULL,
  "when"         TEXT                              DEFAULT NULL,
  phones          TEXT                              DEFAULT NULL, -- string[]
  contact_names   TEXT                              DEFAULT NULL, -- JSON array string
  vk_accounts     TEXT                              DEFAULT NULL, -- JSON array string
  photos          TEXT                              DEFAULT NULL, -- JSON array string (URLs)
  status_details  TEXT                              DEFAULT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (owner_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_posts_date ON posts(date);
