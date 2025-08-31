package main

import (
	"fmt"
	"time"

	tele "github.com/jehaby/lostdogs/internal/telegram"
)

// telegramStart initializes the Telegram client and starts the worker in background.
func telegramStart(svc *service, cfg config) error {
	if cfg.TGToken == "" || cfg.TGChat == 0 {
		return fmt.Errorf("TG_TOKEN and TG_CHAT must be set when TG_ENABLED=true")
	}
	client, err := tele.NewClient(cfg.TGToken, cfg.TGChat)
	if err != nil {
		return err
	}
	// Start worker with conservative defaults
	w := tele.NewWorker(svc.queries, client, tele.WorkerOptions{
		Rate:       time.Second, // ~1 msg/sec
		MaxRetries: 5,
		LeaseTTL:   30 * time.Second,
		Batch:      10,
	})
	go w.Run()
	return nil
}
