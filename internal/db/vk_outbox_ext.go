package db

import (
	"context"
)

// Hand-written extensions for VK outbox, to avoid regenerating sqlc now.

type EnqueueOutboxVKParams struct {
	OwnerID int64 `json:"owner_id"`
	PostID  int64 `json:"post_id"`
}

func (q *Queries) EnqueueOutboxVK(ctx context.Context, arg EnqueueOutboxVKParams) error {
	const stmt = `INSERT INTO outbox_vk (owner_id, post_id)
VALUES (?, ?)
ON CONFLICT(owner_id, post_id) DO NOTHING`
	_, err := q.db.ExecContext(ctx, stmt, arg.OwnerID, arg.PostID)
	return err
}

type ClaimPendingMarkVKParams struct {
	Lease *int64 `json:"lease"`
	Limit int64  `json:"limit"`
}

func (q *Queries) ClaimPendingMarkVK(ctx context.Context, arg ClaimPendingMarkVKParams) error {
	const stmt = `UPDATE outbox_vk
SET status='sending', leased_until=?, updated_at=CURRENT_TIMESTAMP
WHERE id IN (
  SELECT id FROM outbox_vk
  WHERE status='pending' AND (leased_until IS NULL OR leased_until < strftime('%s','now'))
  ORDER BY created_at ASC
  LIMIT ?
)`
	_, err := q.db.ExecContext(ctx, stmt, arg.Lease, arg.Limit)
	return err
}

type ListSendingByLeaseVKRow struct {
	ID      int64 `json:"id"`
	OwnerID int64 `json:"owner_id"`
	PostID  int64 `json:"post_id"`
}

func (q *Queries) ListSendingByLeaseVK(ctx context.Context, lease *int64) ([]ListSendingByLeaseVKRow, error) {
	const stmt = `SELECT id, owner_id, post_id
FROM outbox_vk
WHERE status='sending' AND leased_until=?
ORDER BY created_at ASC`
	rows, err := q.db.QueryContext(ctx, stmt, lease)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []ListSendingByLeaseVKRow
	for rows.Next() {
		var r ListSendingByLeaseVKRow
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.PostID); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

type MarkSentVKParams struct {
	VkPostID *int64 `json:"vk_post_id"`
	ID       int64  `json:"id"`
}

func (q *Queries) MarkSentVK(ctx context.Context, arg MarkSentVKParams) error {
	const stmt = `UPDATE outbox_vk
SET status='sent', vk_post_id=?, updated_at=CURRENT_TIMESTAMP
WHERE id=?`
	_, err := q.db.ExecContext(ctx, stmt, arg.VkPostID, arg.ID)
	return err
}

type MarkFailedVKParams struct {
	MaxRetries int64   `json:"max_retries"`
	LastError  *string `json:"last_error"`
	ID         int64   `json:"id"`
}

func (q *Queries) MarkFailedVK(ctx context.Context, arg MarkFailedVKParams) error {
	const stmt = `UPDATE outbox_vk
SET status=CASE WHEN retries+1>=? THEN 'failed' ELSE 'pending' END,
    retries=retries+1,
    last_error=?,
    leased_until=NULL,
    updated_at=CURRENT_TIMESTAMP
WHERE id=?`
	_, err := q.db.ExecContext(ctx, stmt, arg.MaxRetries, arg.LastError, arg.ID)
	return err
}

func (q *Queries) ReapStaleVK(ctx context.Context) error {
	const stmt = `UPDATE outbox_vk
SET status='pending', leased_until=NULL, updated_at=CURRENT_TIMESTAMP
WHERE status='sending' AND leased_until < strftime('%s','now')`
	_, err := q.db.ExecContext(ctx, stmt)
	return err
}
