package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
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
	// "dobrye_ryki_izh",
	// "otdam_zoo_izevsk",
	"zoopoisk_18",
	// "zhivotnye_izhevska",
	// "houseizhevsk",
	// "im_nuzhen_dom18",
	// "club199572952",
	// "club115762840",
	// "kyterg_izhevsk",
	// "haiblpes",
	// "animal18",
	// "teremok_izhevsk",
	// "club72714266",
	// "club133962148",
	// "pethelpudm",
	// "o_udm",
	// "poteryashka_18",
	// "kot_i_pec",
	// "volshebnyepsy",
}

var (
	// tweak to your needs; combine with city districts etc.
	kw = regexp.MustCompile(`(?i)\b(потерял[ась]|потерялась|пропал[аио]?|найден[ао]?|ищем\s+хозяина|помогите\s+найти|наш[е]л[аи]?|нашлась|ижевск|удмурт)\b`)
	// payload dedupe: memory map; use sqlite/redis in prod
	seen = make(map[string]struct{})
)

func main() {
	vkToken := os.Getenv("VK_TOKEN") // user or service token sufficient for public walls
	tgToken := os.Getenv("TG_TOKEN") // Telegram bot token
	tgChat := os.Getenv("TG_CHAT")   // Telegram chat id, e.g. -1001234567890

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
				if err := scanGroup(ctx, vk, &gs[i], tgToken, tgChat); err != nil {
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

func scanGroup(ctx context.Context, vk *vkapi.VK, g *Group, tgToken, tgChat string) error {
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
		fmt.Println("got msg ",  text)
		// if kw.MatchString(text) || kw.MatchString(attachmentsText(post.Attachments)) {
		// 	// Build a compact preview + link
		// 	link := fmt.Sprintf("https://vk.com/wall%d_%d", post.OwnerID, post.ID)
		// 	msg := fmt.Sprintf("🐾 %s\n\n%s", link, truncate(text, 900))
		// 	fmt.Println("got msg ", msg)
		// 	// Prefer Telegram
		// 	if tgToken != "" && tgChat != "" {
		// 		// if err := sendToTelegram(tgToken, tgChat, msg, firstPhotoURL(post.Attachments)); err != nil {
		// 		// 	log.Printf("telegram send: %v", err)
		// 		// }
		// 	}
		// 	// Or: queue for VK suggested posts via your own group (requires admin token)
		// 	// postToVK(vk, DEST_GROUP_ID, msg, photoAttachmentIDs...)
		// }
		seen[key] = struct{}{}
		if post.Date > int(g.LastTS) {
			g.LastTS = int64(post.Date)
		}
	}
	return nil
}

// func firstPhotoURL(atts []object.WallWallpostAttachment) string {
// 	for _, a := range atts {
// 		if a.Type == "photo" && a.Photo != nil && len(a.Photo.Sizes) > 0 {
// 			// choose the largest
// 			max := a.Photo.Sizes[0]
// 			for _, s := range a.Photo.Sizes {
// 				if s.Width*s.Height > max.Width*max.Height {
// 					max = s
// 				}
// 			}
// 			return max.URL
// 		}
// 	}
// 	return ""
// }

// func attachmentsText(atts []object.WallWallpostAttachment) string {
// 	var b strings.Builder
// 	for _, a := range atts {
// 		switch a.Type {
// 		case "link":
// 			if a.Link != nil {
// 				b.WriteString(" ")
// 				b.WriteString(a.Link.Title)
// 				b.WriteString(" ")
// 				b.WriteString(a.Link.Description)
// 			}
// 		case "doc":
// 			if a.Doc != nil {
// 				b.WriteString(" ")
// 				b.WriteString(a.Doc.Title)
// 			}
// 		}
// 	}
// 	return b.String()
// }

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

// // ----- Telegram -----
// func sendToTelegram(token, chatID, text, photoURL string) error {
// 	if photoURL == "" {
// 		_, err := http.PostForm(
// 			fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
// 			map[string][]string{
// 				"chat_id":                  {chatID},
// 				"text":                     {text},
// 				"disable_web_page_preview": {"false"},
// 				"parse_mode":               {"HTML"},
// 			},
// 		)
// 		return err
// 	}
// 	_, err := http.PostForm(
// 		fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", token),
// 		map[string][]string{
// 			"chat_id":    {chatID},
// 			"photo":      {photoURL},
// 			"caption":    {text},
// 			"parse_mode": {"HTML"},
// 		},
// 	)
// 	return err
// }
