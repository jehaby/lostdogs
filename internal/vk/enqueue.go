package vk

import (
	"context"

	root "github.com/jehaby/lostdogs"
	sqldb "github.com/jehaby/lostdogs/internal/db"
)

// EnqueueIfMatchVK inserts a post into VK outbox if it matches delivery rules (lost only for now).
func EnqueueIfMatchVK(ctx context.Context, q *sqldb.Queries, params sqldb.EnqueueOutboxVKParams, p root.Post) error {
	if p.Type != root.TypeLost {
		return nil
	}
	return q.EnqueueOutboxVK(ctx, params)
}
