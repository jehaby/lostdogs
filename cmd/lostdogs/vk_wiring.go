package main

import (
	"fmt"
	"time"

	vkout "github.com/jehaby/lostdogs/internal/vk"
)

// vkStart initializes the VK outbound client and starts the worker in background.
func vkStart(svc *service, cfg config) error {
	token := cfg.VKOutToken
	if token == "" {
		token = cfg.VKToken
	}
	if token == "" || cfg.VKOutOwnerID == 0 {
		return fmt.Errorf("VK token and VK_OUT_OWNER_ID must be set when VK_OUT_ENABLED=true")
	}
	cli, err := vkout.NewClient(token, cfg.VKOutOwnerID, cfg.VKOutFromGroup, cfg.VKOutHTTPTimeout)
	if err != nil {
		return err
	}
	// Start worker with conservative defaults derived from config
	rate := time.Second
	if cfg.VKOutRatePerSec > 0 {
		rate = time.Duration(float64(time.Second) / cfg.VKOutRatePerSec)
	}
	w := vkout.NewWorker(svc.queries, cli, vkout.WorkerOptions{
		Rate:       rate,
		MaxRetries: 5,
		LeaseTTL:   30 * time.Second,
		Batch:      10,
	})
	go w.Run()
	return nil
}
