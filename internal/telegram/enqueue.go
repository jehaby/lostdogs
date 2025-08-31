package telegram

import (
	"context"

	root "github.com/jehaby/lostdogs"
	sqldb "github.com/jehaby/lostdogs/internal/db"
)

// EnqueueIfMatch inserts a post into outbox if it matches delivery rules.
// Accepts sqldb.EnqueueOutboxParams for clarity at call sites.
// Currently: only posts with TypeLost are enqueued.
func EnqueueIfMatch(ctx context.Context, q *sqldb.Queries, params sqldb.EnqueueOutboxParams, p root.Post) error {
	if p.Type != root.TypeLost {
		return nil
	}
	return q.EnqueueOutbox(ctx, params)
}
