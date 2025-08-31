package telegram

import (
	"fmt"
	"html"
	"strings"
	"text/template"

	sqldb "github.com/jehaby/lostdogs/internal/db"
)

var msgTmpl = template.Must(template.New("tgmsg").Parse(`{{- if .Title -}}{{.Title}}
{{end}}{{- if .Text -}}
{{.Text}}
{{end}}<a href="{{.Link}}">Источник VK</a>`))

type tmplData struct {
	Title string
	Text  string // already HTML-escaped
	Link  string // raw URL
}

// BuildMessage builds a Telegram-ready HTML message body from a stored post using text/template.
func BuildMessage(p sqldb.GetPostRow) string {
	// Prepare values
	title := typeTitle(p.Type)
	body := p.Text
	if len(body) > 3500 {
		body = body[:3500] + "…"
	}
	data := tmplData{
		Title: title,
		Text:  html.EscapeString(body),
		Link:  vkLink(p.OwnerID, p.PostID),
	}

	var b strings.Builder
	_ = msgTmpl.Execute(&b, data)
	return b.String()
}

func vkLink(ownerID, postID int64) string {
	return fmt.Sprintf("https://vk.com/wall%d_%d", ownerID, postID)
}

func typeTitle(t string) string {
	switch t {
	case "lost":
		return "🔎 Пропал питомец"
	case "found":
		return "✅ Найден питомец"
	case "sighting":
		return "👀 Замечен питомец"
	case "adoption":
		return "🏠 Ищет дом"
	case "fundraising":
		return "💳 Сбор помощи"
	default:
		return ""
	}
}
