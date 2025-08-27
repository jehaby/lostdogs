package main

import (
	"context"
	"fmt"
	"log"
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
	vkToken := os.Getenv("VK_TOKEN") // user or service token sufficient for public walls

	if vkToken == "" {
		log.Fatal("VK_TOKEN is required")
	}

	vk := vkapi.NewVK(vkToken)
	client := &http.Client{Timeout: 10 * time.Second}
	vk.Client = client

	// 1) Resolve group IDs once
	var gs []Group
	for _, name := range groups {
		id, err := resolveGroupID(vk, name)
		if err != nil {
			log.Printf("resolve %s: %v", name, err)
			continue
		}
		gs = append(gs, Group{ScreenName: name, ID: id})
	}

	ticker := time.NewTicker(30 * time.Second) // polite polling
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			for i := range gs {
				if err := scanGroup(ctx, vk, &gs[i]); err != nil {
					log.Printf("scan %s: %v", gs[i].ScreenName, err)
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
	resp, err := vk.WallGet(vkapi.Params{
		"owner_id": -g.ID,
		"count":    50,
	})
	if err != nil {
		return err
	}
	for i := len(resp.Items) - 1; i >= 0; i-- { // oldest → newest
		post := resp.Items[i]
		if post.Date < int(g.LastTS) {
			continue
		}
		key := fmt.Sprintf("%d_%d", post.OwnerID, post.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		text := normalize(post.Text)
		fmt.Println("got msg ", text)
		seen[key] = struct{}{}
		if post.Date > int(g.LastTS) {
			g.LastTS = int64(post.Date)
		}
	}
	return nil
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
