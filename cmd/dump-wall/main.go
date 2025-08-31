package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
	"github.com/caarlos0/env/v11"
)

type config struct {
	VKToken  string     `env:"VK_TOKEN,required"`
	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`
}

type photoInfo struct {
	ID      int     `json:"id"`
	OwnerID int     `json:"owner_id"`
	URL     string  `json:"url"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	Type    string  `json:"type"`
}

type simplePost struct {
	OwnerID int         `json:"owner_id"`
	ID      int         `json:"id"`
	Date    int         `json:"date"`
	Text    string      `json:"text"`
	Photos  []photoInfo `json:"photos,omitempty"`
}

func main() {
	// Flags
	var (
		group = flag.String("group", "", "VK group screen name (e.g., zoopoisk_18)")
		count = flag.Int("count", 100, "Number of posts to fetch")
		out   = flag.String("out", "", "Output JSON file path")
	)
	flag.Parse()

	// Config from env
	cfg := config{}
	if err := env.Parse(&cfg); err != nil {
		slog.Error("error parsing config", "err", err)
		os.Exit(1)
	}
	slog.SetLogLoggerLevel(cfg.LogLevel)

	if *group == "" || *out == "" {
		slog.Error("missing required flags --group and --out")
		os.Exit(2)
	}

	// VK client
	vk := vkapi.NewVK(cfg.VKToken)
	vk.Client = &http.Client{Timeout: 10 * time.Second}
	slog.Info("VK client initialized", "timeout", 10*time.Second)

	// Resolve group id
	id, err := resolveGroupID(vk, *group)
	if err != nil {
		slog.Error("resolve group id failed", "group", *group, "err", err)
		os.Exit(1)
	}

	// Fetch posts
	resp, err := vk.WallGet(vkapi.Params{
		"owner_id": -id,
		"count":    *count,
	})
	if err != nil {
		slog.Error("wall.get failed", "err", err)
		os.Exit(1)
	}

	// Convert and write JSON
	items := make([]simplePost, 0, len(resp.Items))
	for _, p := range resp.Items {
		sp := simplePost{OwnerID: p.OwnerID, ID: p.ID, Date: p.Date, Text: p.Text}
		// collect photo attachments
		for _, att := range p.Attachments {
			if att.Type == "photo" && att.Photo.ID != 0 {
				// pick the largest size available
				sz := att.Photo.MaxSize()
				sp.Photos = append(sp.Photos, photoInfo{
					ID:      att.Photo.ID,
					OwnerID: att.Photo.OwnerID,
					URL:     sz.URL,
					Width:   sz.Width,
					Height:  sz.Height,
					Type:    sz.Type,
				})
			}
		}
		// optionally include photos from copy_history (reposts)
		for _, cp := range p.CopyHistory {
			for _, att := range cp.Attachments {
				if att.Type == "photo" && att.Photo.ID != 0 {
					sz := att.Photo.MaxSize()
					sp.Photos = append(sp.Photos, photoInfo{
						ID:      att.Photo.ID,
						OwnerID: att.Photo.OwnerID,
						URL:     sz.URL,
						Width:   sz.Width,
						Height:  sz.Height,
						Type:    sz.Type,
					})
				}
			}
		}
		items = append(items, sp)
	}
	f, err := os.Create(*out)
	if err != nil {
		slog.Error("create output failed", "path", *out, "err", err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	// Keep '&', '<', '>' as-is in URLs (avoid \u0026, etc.)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(items); err != nil {
		slog.Error("write json failed", "err", err)
		os.Exit(1)
	}
	slog.Info("dump completed", "out", *out, "count", len(items))
}

// resolveGroupID mirrors the helper in the root binary.
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
