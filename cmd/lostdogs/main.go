package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"slices"
	"strings"
	"time"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
	object "github.com/SevereCloud/vksdk/v3/object"
	"github.com/caarlos0/env/v11"
	yaml "github.com/goccy/go-yaml"
	"github.com/jehaby/lostdogs"
	sqldb "github.com/jehaby/lostdogs/internal/db"
	itypes "github.com/jehaby/lostdogs/internal/types"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

type Group struct {
	ScreenName string
	ID         int // numeric id (positive). owner_id will be -ID
	LastTS     int64
}

type config struct {
	VKToken           string     `env:"VK_TOKEN,required"`
	LogLevel          slog.Level `env:"LOG_LEVEL" envDefault:"info"`
	TGBotDebugEnabled bool       `env:"TGBOT_DEBUG_ENABLED" envDefault:"false"`
	DBConnString      string     `env:"DB_CONN_STRING" envDefault:"file:./resources/db/lostdogs.db?cache=shared&mode=rwc"`
	TGEnabled         bool       `env:"TG_ENABLED" envDefault:"false"`
	TGToken           string     `env:"TG_TOKEN"`
	TGChat            int64      `env:"TG_CHAT"`
	// VK outbound (reposting) configuration
	VKOutEnabled     bool          `env:"VK_OUT_ENABLED" envDefault:"false"`
	VKOutToken       string        `env:"VK_OUT_TOKEN"`
	VKOutOwnerID     int64         `env:"VK_OUT_OWNER_ID"`
	VKOutRatePerSec  float64       `env:"VK_OUT_RATE_PER_SEC" envDefault:"1.0"`
	VKOutHTTPTimeout time.Duration `env:"VK_OUT_HTTP_TIMEOUT" envDefault:"10s"`
	VKOutFromGroup   bool          `env:"VK_OUT_FROM_GROUP" envDefault:"true"`
}

type service struct {
	db      *sql.DB
	queries *sqldb.Queries
	vk      *vkapi.VK
}

func newService(cfg config) *service {
	svc := &service{}
	// Open standard database/sql connection using sqlite3 driver
	var err error
	svc.db, err = sql.Open("sqlite3", cfg.DBConnString)
	if err != nil {
		slog.Error("open sqlite failed", "err", err)
		os.Exit(1)
	}
	if err := svc.db.Ping(); err != nil {
		slog.Error("ping sqlite failed", "err", err)
		os.Exit(1)
	}

	if err := applyMigrations(svc.db, "./resources/db/migrations"); err != nil {
		slog.Error("error applying migrations", "err", err)
		os.Exit(1)
	}

	svc.queries = sqldb.New(svc.db)

	// Initialize VK client
	vk := vkapi.NewVK(cfg.VKToken)
	client := &http.Client{Timeout: 10 * time.Second}
	vk.Client = client
	svc.vk = vk
	slog.Info("VK client initialized", "timeout", client.Timeout)
	return svc
}

func main() {
	// Parse config from environment
	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		slog.Error("error parsing config", "err", err)
		os.Exit(1)
	}
	initLogger(cfg.LogLevel)

	svc := newService(cfg)

	// Optionally start Telegram worker
	if cfg.TGEnabled {
		slog.Info("starting tg worker")
		if err := telegramStart(svc, cfg); err != nil {
			slog.Error("telegram start failed", "err", err)
		}
	}

	// Optionally start VK outbound worker
	if cfg.VKOutEnabled {
		slog.Info("starting vk worker")
		if err := vkStart(svc, cfg); err != nil {
			slog.Error("vk start failed", "err", err)
		}
	}

	var groups []string
	groups = loadGroupsFromYAML()

	// 1) Resolve group IDs once
	var gs []Group
	slog.Info("resolving groups", "requested", len(groups))
	for _, name := range groups {
		id, err := resolveGroupID(svc.vk, name)
		if err != nil {
			slog.Error("resolve group id failed", "screen_name", name, "err", err)
			continue
		}
		slog.Info("group resolved", "screen_name", name, "id", id)
		gs = append(gs, Group{ScreenName: name, ID: id})
	}
	slog.Info("groups ready", "count", len(gs))

	// Run initial scan immediately
	svc.scanAllGroups(gs)

	ticker := time.NewTicker(60 * time.Second) // polite polling
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			svc.scanAllGroups(gs)
		}
	}
}

// loadGroupsFromYAML loads group screen names from config YAML (config.yaml/config.yml)
func loadGroupsFromYAML() []string {
	candidates := []string{"config.yaml", "config.yml"}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			b, err := os.ReadFile(p)
			if err != nil {
				slog.Error("read config failed", "path", p, "err", err)
				continue
			}
			var fc struct {
				VKGroups []string `yaml:"vk-groups"`
			}
			if err := yaml.Unmarshal(b, &fc); err != nil {
				slog.Error("yaml unmarshal failed", "path", p, "err", err)
				continue
			}
			if len(fc.VKGroups) == 0 {
				slog.Warn("yaml has no vk-groups", "path", p)
			} else {
				slog.Info("loaded groups from yaml", "path", p, "count", len(fc.VKGroups))
			}
			return fc.VKGroups
		}
	}
	slog.Warn("no config yaml found", "candidates", candidates)
	return nil
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

func (svc *service) scanGroup(ctx context.Context, g *Group) error {
	// Get last N posts; if you need deeper history, implement offset pagination.
	slog.Debug("wall.get request", "owner_id", -g.ID, "count", 50, "last_ts", g.LastTS)
	resp, err := svc.vk.WallGet(vkapi.Params{
		"owner_id": -g.ID,
		"count":    50,
	})
	if err != nil {
		return err
	}
	slog.Debug("wall.get ok", "owner_id", -g.ID, "items", len(resp.Items))
	svc.processPosts(ctx, resp.Items, g)
	return nil
}

// processPosts processes VK posts for a group, updating last timestamp and
// skipping posts already present in SQLite.
func (svc *service) processPosts(ctx context.Context, posts []object.WallWallpost, g *Group) {
	for i := len(posts) - 1; i >= 0; i-- { // oldest → newest
		post := posts[i]
		if post.Date < int(g.LastTS) {
			slog.Debug("skip old post", "post_id", post.ID, "date", post.Date, "last_ts", g.LastTS)
			continue
		}
		// Skip if already saved in DB (persistent dedupe). Use a short-lived context so
		// this check is not coupled to the outer scan timeout.
		exCtx, exCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		n, err := svc.queries.ExistsPost(exCtx, sqldb.ExistsPostParams{OwnerID: int64(post.OwnerID), PostID: int64(post.ID)})
		exCancel()
		if err != nil {
			slog.Error("exists check failed", "owner_id", post.OwnerID, "post_id", post.ID, "err", err)
			// best-effort: continue as new to avoid missing data
		} else if n > 0 {
			slog.Debug("skip seen post (db)", "owner_id", post.OwnerID, "post_id", post.ID)
			continue
		}
		text := normalize(post.Text)
		link := fmt.Sprintf("https://vk.com/wall%d_%d", post.OwnerID, post.ID)
		// collect photo URLs (including copy_history)
		var photos []string
		phTmp := post.Attachments
		for _, cp := range post.CopyHistory {
			phTmp = slices.Concat(phTmp, cp.Attachments)
		}
		for _, att := range phTmp {
			if att.Type == "photo" && att.Photo.ID != 0 {
				sz := att.Photo.MaxSize()
				if sz.URL != "" {
					photos = append(photos, sz.URL)
				}
			}
		}
		slog.Debug("got msg", "owner_id", post.OwnerID, "post_id", post.ID, "date", post.Date, "text", text, "link", link)
		// Persist new message in SQLite (best-effort)
		if err := svc.SaveMessage(post.OwnerID, post.ID, int64(post.Date), post.Text, text, link, photos); err != nil {
			slog.Error("db save failed", "err", err, "owner_id", post.OwnerID, "post_id", post.ID)
		}
		if post.Date > int(g.LastTS) {
			old := g.LastTS
			g.LastTS = int64(post.Date)
			slog.Debug("last_ts updated", "old", old, "new", g.LastTS)
		}
	}
}

// scanAllGroups performs one pass over all groups with a timeout context.
func (s *service) scanAllGroups(gs []Group) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	slog.Debug("tick: scanning groups", "count", len(gs))
	for i := range gs {
		if err := s.scanGroup(ctx, &gs[i]); err != nil {
			slog.Error("scan group failed", "screen_name", gs[i].ScreenName, "err", err)
		}
		time.Sleep(500 * time.Millisecond) // rate-limit spacing
	}
}

// SaveMessage parses raw VK text and persists it via sqlc UpsertPost.
func (s *service) SaveMessage(ownerID int, postID int, date int64, raw, normalized, link string, photos []string) error {
	// Parse domain-level fields from raw text
	p := lostdogs.Parse(postID, raw)

	// Map domain enums to DB strings
	sex := string(p.Sex)
	if sex == "" {
		sex = "unknown"
	}
	animal := string(p.Animal)
	if animal == "" {
		animal = "unknown"
	}
	ptype := string(p.Type)
	if ptype == "" {
		ptype = "unknown"
	}
	// Optional string pointers
	sPtr := func(v string) *string {
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return &v
	}

	// Slices -> JSON-backed StringSlice
	var phones itypes.StringSlice
	if len(p.Phones) > 0 {
		phones = itypes.StringSlice(p.Phones)
	}
	var contactNames itypes.StringSlice
	if len(p.ContactNames) > 0 {
		contactNames = itypes.StringSlice(p.ContactNames)
	}
	var vkAccounts itypes.StringSlice
	if len(p.VKAccounts) > 0 {
		vkAccounts = itypes.StringSlice(p.VKAccounts)
	}
	var photoURLs itypes.StringSlice
	if len(photos) > 0 {
		photoURLs = itypes.StringSlice(photos)
	}

	params := sqldb.UpsertPostParams{
		OwnerID:       int64(ownerID),
		PostID:        int64(postID),
		Date:          date,
		Text:          normalized,
		Raw:           raw,
		Type:          ptype,
		Animal:        animal,
		Sex:           sex,
		Name:          sPtr(p.Name),
		Location:      sPtr(p.Location),
		When:          sPtr(p.When),
		Phones:        phones,
		ContactNames:  contactNames,
		VkAccounts:    vkAccounts,
		Photos:        photoURLs,
		StatusDetails: sPtr(p.StatusDetails),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.queries.UpsertPost(ctx, params); err != nil {
		return err
	}

	if shouldPost(p) {
		// Enqueue to Telegram outbox for matching posts (e.g., lost)
		if err := s.queries.EnqueueOutbox(ctx, sqldb.EnqueueOutboxParams{OwnerID: int64(ownerID), PostID: int64(postID)}); err != nil {
			slog.Error("telegram enqueue failed", "err", err, "owner_id", ownerID, "post_id", postID)
		}
		// Enqueue to VK outbox for matching posts (e.g., lost)
		if err := s.queries.EnqueueOutboxVK(ctx, sqldb.EnqueueOutboxVKParams{OwnerID: int64(ownerID), PostID: int64(postID)}); err != nil {
			slog.Error("vk enqueue failed", "err", err, "owner_id", ownerID, "post_id", postID)
		}
	}

	return nil
}

var allowedTypes = []lostdogs.PostType{lostdogs.TypeLost, lostdogs.TypeFound, lostdogs.TypeSighting}

func shouldPost(p lostdogs.Post) bool {
	if slices.Contains(allowedTypes, p.Type) && p.Animal == lostdogs.AnimalDog {
		return true
	}
	return false
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

// ----- End

func applyMigrations(db *sql.DB, dir string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
