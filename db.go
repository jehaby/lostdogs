package main

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

// schema holds the DDL for initializing the local SQLite database.
// Keep this the single source of truth for schema migrations in this repo.
const schema = `
CREATE TABLE IF NOT EXISTS messages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  key TEXT NOT NULL UNIQUE,             -- owner_id_post_id
  owner_id INTEGER NOT NULL,
  post_id INTEGER NOT NULL,
  ts INTEGER NOT NULL,                  -- VK post unix timestamp
  text TEXT NOT NULL,
  link TEXT NOT NULL,                   -- https://vk.com/wall{owner_id}_{post_id}
  photo_url TEXT,                       -- chosen preview photo (if any)
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_ts ON messages(ts);
`

// SaveMessage persists a found/forwarded post; duplicate keys are ignored.
func (s *service) SaveMessage(ownerID, postID int, ts int64, text, link, photoURL string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil service db")
	}
	key := fmt.Sprintf("%d_%d", ownerID, postID)
	const q = `INSERT OR IGNORE INTO messages (key, owner_id, post_id, ts, text, link, photo_url)
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(q, key, ownerID, postID, ts, text, link, photoURL)
	return err
}
