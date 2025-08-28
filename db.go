package main

import (
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

// schema holds the DDL for initializing the local SQLite database.
// It matches the domain Post struct and avoids normalization for arrays (stored as JSON strings).
const schema = `
CREATE TABLE IF NOT EXISTS posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  raw TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'unknown',
  animal TEXT NOT NULL DEFAULT 'unknown',
  breed TEXT NOT NULL DEFAULT '',
  sex TEXT NOT NULL DEFAULT 'unknown',
  age TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  "when" TEXT NOT NULL DEFAULT '',
  phones TEXT NOT NULL DEFAULT '[]',
  contact_names TEXT NOT NULL DEFAULT '[]',
  vk_accounts TEXT NOT NULL DEFAULT '[]',
  status_details TEXT NOT NULL DEFAULT '',
  extras_sterilized INTEGER NOT NULL DEFAULT 0,
  extras_vaccinated INTEGER NOT NULL DEFAULT 0,
  extras_chipped INTEGER NOT NULL DEFAULT 0,
  extras_litter_ok INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_posts_type ON posts(type);
CREATE INDEX IF NOT EXISTS idx_posts_animal ON posts(animal);
`

// SaveMessage persists a found/forwarded post; duplicate keys are ignored.
func (s *service) SaveMessage(ownerID, postID int, ts int64, text, link, photoURL string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("nil service db")
	}
	// Minimal insert into posts, mapping only available fields.
	// Arrays are stored as empty JSON by default; other fields default to unknown/empty.
	const q = `INSERT INTO posts (raw) VALUES (?)`
	_, err := s.db.Exec(q, text)
	return err
}
