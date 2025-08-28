-- SQLite schema for lostdogs

PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS posts (
  owner_id        INTEGER   NOT NULL,
  post_id         INTEGER   NOT NULL,
  date            INTEGER   NOT NULL, -- Unix timestamp (seconds)
  text            TEXT      NOT NULL,
  -- additional parsed/annotated data
  raw             TEXT      NOT NULL,                -- original raw content
  type            TEXT                              DEFAULT NULL,
  animal          TEXT                              DEFAULT NULL,
  breed           TEXT                              DEFAULT NULL,
  sex             TEXT                              DEFAULT NULL,
  age             TEXT                              DEFAULT NULL,
  name            TEXT                              DEFAULT NULL,
  location        TEXT                              DEFAULT NULL,
  "when"         TEXT                              DEFAULT NULL,
  phones          TEXT                              DEFAULT NULL, -- JSON array string
  contact_names   TEXT                              DEFAULT NULL, -- JSON array string
  vk_accounts     TEXT                              DEFAULT NULL, -- JSON array string
  status_details  TEXT                              DEFAULT NULL,
  extras_sterilized INTEGER                         DEFAULT NULL CHECK (extras_sterilized IN (0,1)),
  extras_vaccinated INTEGER                         DEFAULT NULL CHECK (extras_vaccinated IN (0,1)),
  extras_chipped    INTEGER                         DEFAULT NULL CHECK (extras_chipped IN (0,1)),
  extras_litter_ok  INTEGER                         DEFAULT NULL CHECK (extras_litter_ok IN (0,1)),
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (owner_id, post_id)
);

CREATE INDEX IF NOT EXISTS idx_posts_date ON posts(date);
