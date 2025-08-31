package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	object "github.com/SevereCloud/vksdk/v3/object"
	root "github.com/jehaby/lostdogs"
	sqldb "github.com/jehaby/lostdogs/internal/db"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

// minimal structure of fixture posts produced by cmd/dump-wall
type fixturePost struct {
	OwnerID int    `json:"owner_id"`
	ID      int    `json:"id"`
	Date    int    `json:"date"`
	Text    string `json:"text"`
}

func TestProcessPosts_CountLostFromFixture(t *testing.T) {
	t.Parallel()

	// Open isolated in-memory SQLite and apply schema
	db, err := sql.Open("sqlite3", "file:memdb_process_posts?cache=shared&mode=memory")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	schema := filepath.Join("..", "..", "resources", "db", "schema.sql")
	require.NoError(t, applySchemaFile(db, schema))

	svc := &service{
		db:      db,
		queries: sqldb.New(db),
	}

	// Load fixture posts
	fixturePath := filepath.Join("..", "..", "resources", "fixtures", "wall_zoopoisk_18_100.json")
	b, err := os.ReadFile(fixturePath)
	require.NoError(t, err)

	var fps []fixturePost
	require.NoError(t, json.Unmarshal(b, &fps))

	// Convert to VK SDK type expected by processPosts
	posts := make([]object.WallWallpost, 0, len(fps))
	var expectLost int
	for _, p := range fps {
		posts = append(posts, object.WallWallpost{
			OwnerID: p.OwnerID,
			ID:      p.ID,
			Date:    p.Date,
			Text:    p.Text,
		})
		if root.Parse(p.ID, p.Text).Type == root.TypeLost {
			expectLost++
		}
	}

	// Run processing against empty DB; expect all posts to be inserted
	ctx := context.Background()
	g := &Group{ID: 0, LastTS: 0}
	svc.processPosts(ctx, posts, g)

	// Query DB for count of rows with type='lost'
	var gotLost int
	err = db.QueryRowContext(ctx, "SELECT COUNT(1) FROM posts WHERE type = 'lost'").Scan(&gotLost)
	require.NoError(t, err)

	require.Equal(t, expectLost, gotLost, "lost posts count should match parse results")
}
