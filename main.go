package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
)

type Group struct {
	ScreenName string
	ID         int // numeric id (positive). owner_id will be -ID
	LastTS     int64
}

var groups = []string{
	"zoopoisk_18",
}

var (
	// payload dedupe: memory map; use sqlite/redis in prod
	seen = make(map[string]struct{})
)

func main() {
	initLogger()
	vkToken := os.Getenv("VK_TOKEN") // user or service token sufficient for public walls

	if vkToken == "" {
		slog.Error("VK_TOKEN is required")
		os.Exit(1)
	}

	vk := vkapi.NewVK(vkToken)
	client := &http.Client{Timeout: 10 * time.Second}
	vk.Client = client
	slog.Info("VK client initialized", "timeout", client.Timeout)

	// 1) Resolve group IDs once
	var gs []Group
	slog.Info("resolving groups", "requested", len(groups))
	for _, name := range groups {
		id, err := resolveGroupID(vk, name)
		if err != nil {
			slog.Error("resolve group id failed", "screen_name", name, "err", err)
			continue
		}
		slog.Info("group resolved", "screen_name", name, "id", id)
		gs = append(gs, Group{ScreenName: name, ID: id})
	}
	slog.Info("groups ready", "count", len(gs))

	ticker := time.NewTicker(30 * time.Second) // polite polling
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			slog.Debug("tick: scanning groups", "count", len(gs))
			for i := range gs {
				if err := scanGroup(ctx, vk, &gs[i]); err != nil {
					slog.Error("scan group failed", "screen_name", gs[i].ScreenName, "err", err)
				}
				time.Sleep(500 * time.Millisecond) // rate-limit spacing
			}
			cancel()
		}
	}
}

func resolveGroupID(vk *vkapi.VK, screen string) (int, error) {
	resp, err := vk.UtilsResolveScreenName(vkapi.Params{"screen_name": screen})
	if err != nil {
		return 0, err
	}
	if resp.Type != "group" {
		return 0, fmt.Errorf("%s is not a group", screen)
	}
	return resp.ObjectID, nil
}

func scanGroup(ctx context.Context, vk *vkapi.VK, g *Group) error {
	// Get last N posts; if you need deeper history, implement offset pagination.
	slog.Debug("wall.get request", "owner_id", -g.ID, "count", 50, "last_ts", g.LastTS)
	resp, err := vk.WallGet(vkapi.Params{
		"owner_id": -g.ID,
		"count":    50,
	})
	if err != nil {
		return err
	}
	slog.Info("wall.get ok", "owner_id", -g.ID, "items", len(resp.Items))
	for i := len(resp.Items) - 1; i >= 0; i-- { // oldest → newest
		post := resp.Items[i]
		if post.Date < int(g.LastTS) {
			slog.Debug("skip old post", "post_id", post.ID, "date", post.Date, "last_ts", g.LastTS)
			continue
		}
		key := fmt.Sprintf("%d_%d", post.OwnerID, post.ID)
		if _, ok := seen[key]; ok {
			slog.Debug("skip seen post", "key", key)
			continue
		}
		text := normalize(post.Text)
		slog.Info("got msg", "owner_id", post.OwnerID, "post_id", post.ID, "date", post.Date, "text", text)
		seen[key] = struct{}{}
		if post.Date > int(g.LastTS) {
			old := g.LastTS
			g.LastTS = int64(post.Date)
			slog.Debug("last_ts updated", "old", old, "new", g.LastTS)
		}
	}
	return nil
}

func initLogger() {
	levelVar := new(slog.LevelVar)
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "warn", "warning":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}

	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: levelVar})
	}
	slog.SetDefault(slog.New(handler))
}

func normalize(s string) string {
	s = strings.ReplaceAll(s, "\u00A0", " ")
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// ----- End
