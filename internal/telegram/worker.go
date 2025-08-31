package telegram

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	sqldb "github.com/jehaby/lostdogs/internal/db"
)

type WorkerOptions struct {
	Rate       time.Duration // per-message delay (e.g., 1s)
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
		// Best-effort loop; do not block scanner on errors
		if err := w.tick(); err != nil {
			slog.Error("tg worker tick failed", "err", err)
		}
	}
}

func (w *Worker) tick() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Reap stale leases
	_ = w.q.ReapStale(ctx)

	lease := time.Now().Add(w.opt.LeaseTTL).Unix()
	// Mark a batch as sending
	if err := w.q.ClaimPendingMark(ctx, sqldb.ClaimPendingMarkParams{Lease: &lease, Limit: int64(w.opt.Batch)}); err != nil {
		return err
	}
	// Load claimed rows
	rows, err := w.q.ListSendingByLease(ctx, &lease)
	if err != nil {
		return err
	}
	for _, r := range rows {
		// Load post
		post, err := w.q.GetPost(ctx, sqldb.GetPostParams{OwnerID: r.OwnerID, PostID: r.PostID})
		if err != nil {
			// Mark failed permanently if cannot load post
			msg := "get post: " + err.Error()
			_ = w.q.MarkFailed(ctx, sqldb.MarkFailedParams{MaxRetries: int64(w.opt.MaxRetries), LastError: &msg, ID: r.ID})
			continue
		}
		// Build message
		text := BuildMessage(post)
		// Send with HTML parse mode
		resp, err := w.cli.Bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    w.cli.ChatID,
			Text:      text,
			ParseMode: models.ParseModeHTML,
		})
		if err != nil {
			msg := err.Error()
			// Simple retry policy: requeue until MaxRetries
			_ = w.q.MarkFailed(ctx, sqldb.MarkFailedParams{MaxRetries: int64(w.opt.MaxRetries), LastError: &msg, ID: r.ID})
			continue
		}
		// Mark sent with Telegram message id
		if resp != nil {
			mid := int64(resp.ID)
			_ = w.q.MarkSent(ctx, sqldb.MarkSentParams{TgMessageID: &mid, ID: r.ID})
		} else {
			_ = w.q.MarkSent(ctx, sqldb.MarkSentParams{TgMessageID: nil, ID: r.ID})
		}
		// brief delay between messages
		time.Sleep(w.opt.Rate)
	}
	return nil
}
