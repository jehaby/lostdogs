package vk

import (
	"context"
	"log/slog"
	"strings"
	"time"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
	sqldb "github.com/jehaby/lostdogs/internal/db"
)

type WorkerOptions struct {
	Rate       time.Duration // per-message delay
	MaxRetries int           // before marking failed
	LeaseTTL   time.Duration // how long a claim is valid
	Batch      int           // claim up to this many per tick
}

type Worker struct {
	q   *sqldb.Queries
	cli *Client
	opt WorkerOptions
}

func NewWorker(q *sqldb.Queries, cli *Client, opt WorkerOptions) *Worker {
	if opt.Rate <= 0 {
		opt.Rate = time.Second
	}
	if opt.LeaseTTL <= 0 {
		opt.LeaseTTL = 30 * time.Second
	}
	if opt.Batch <= 0 {
		opt.Batch = 10
	}
	if opt.MaxRetries <= 0 {
		opt.MaxRetries = 5
	}
	return &Worker{q: q, cli: cli, opt: opt}
}

func (w *Worker) Run() {
	ticker := time.NewTicker(w.opt.Rate)
	defer ticker.Stop()
	for range ticker.C {
		if err := w.tick(); err != nil {
			slog.Error("vk worker tick failed", "err", err)
		}
	}
}

func (w *Worker) tick() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Reap stale leases
	_ = w.q.ReapStaleVK(ctx)

	lease := time.Now().Add(w.opt.LeaseTTL).Unix()
	if err := w.q.ClaimPendingMarkVK(ctx, sqldb.ClaimPendingMarkVKParams{Lease: &lease, Limit: int64(w.opt.Batch)}); err != nil {
		return err
	}
	rows, err := w.q.ListSendingByLeaseVK(ctx, &lease)
	if err != nil {
		return err
	}
	for _, r := range rows {
		post, err := w.q.GetPost(ctx, sqldb.GetPostParams{OwnerID: r.OwnerID, PostID: r.PostID})
		if err != nil {
			msg := "get post: " + err.Error()
			_ = w.q.MarkFailedVK(ctx, sqldb.MarkFailedVKParams{MaxRetries: int64(w.opt.MaxRetries), LastError: &msg, ID: r.ID})
			slog.Error("vk worker: load post failed", "owner_id", r.OwnerID, "post_id", r.PostID, "err", err)
			continue
		}
		text := BuildMessage(post)
		text = strings.ToValidUTF8(text, "")

		params := vkapi.Params{
			"owner_id":   w.cli.DestOwnerID,
			"message":    text,
			"from_group": 0,
		}
		if w.cli.FromGroup {
			params["from_group"] = 1
		}

		// Post to VK wall; ignore returned post id for now
		_, err = w.cli.VK.WallPost(params)
		if err != nil {
			slog.Error("vk wall.post failed", "dest_owner_id", w.cli.DestOwnerID, "owner_id", r.OwnerID, "post_id", r.PostID, "err", err)
			msg := err.Error()
			_ = w.q.MarkFailedVK(ctx, sqldb.MarkFailedVKParams{MaxRetries: int64(w.opt.MaxRetries), LastError: &msg, ID: r.ID})
			continue
		}
		_ = w.q.MarkSentVK(ctx, sqldb.MarkSentVKParams{VkPostID: nil, ID: r.ID})
		time.Sleep(w.opt.Rate)
	}
	return nil
}
