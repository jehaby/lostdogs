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
    "github.com/jmoiron/sqlx"
    _ "github.com/mattn/go-sqlite3"
    "github.com/caarlos0/env/v11"
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

// Config parsed from environment.
type config struct {
    VKToken           string     `env:"VK_TOKEN,required"`
    LogLevel          slog.Level `env:"LOG_LEVEL" envDefault:"info"`
    TGBotDebugEnabled bool       `env:"TGBOT_DEBUG_ENABLED" envDefault:"false"`
    DBConnString      string     `env:"DB_CONN_STRING" envDefault:"file:./db/bot.db?cache=shared&mode=rwc"`
}

func main() {
    // Parse config from environment
    cfg := config{}
    if err := env.Parse(&cfg); err != nil {
        slog.Error("error parsing config", "err", err)
        os.Exit(1)
    }
    initLogger(cfg.LogLevel)
    // Initialize DB using sqlx and apply schema
    var svc service
    svc.db = sqlx.MustConnect("sqlite3", cfg.DBConnString)
    svc.db.MustExec(schema)

    vk := vkapi.NewVK(cfg.VKToken)
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
				if err := scanGroup(ctx, vk, &svc, &gs[i]); err != nil {
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

func scanGroup(ctx context.Context, vk *vkapi.VK, svc *service, g *Group) error {
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
		link := fmt.Sprintf("https://vk.com/wall%d_%d", post.OwnerID, post.ID)
		slog.Info("got msg", "owner_id", post.OwnerID, "post_id", post.ID, "date", post.Date, "text", text, "link", link)
		// Persist new message in SQLite (best-effort)
		if err := svc.SaveMessage(post.OwnerID, post.ID, int64(post.Date), text, link, ""); err != nil {
			slog.Error("db save failed", "err", err, "owner_id", post.OwnerID, "post_id", post.ID)
		}
		seen[key] = struct{}{}
		if post.Date > int(g.LastTS) {
			old := g.LastTS
			g.LastTS = int64(post.Date)
			slog.Debug("last_ts updated", "old", old, "new", g.LastTS)
		}
	}
	return nil
}

func initLogger(level slog.Level) {
    levelVar := new(slog.LevelVar)
    levelVar.Set(level)

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
