package vk

import (
	"fmt"
	"strings"
	"text/template"

	sqldb "github.com/jehaby/lostdogs/internal/db"
)

var msgTmpl = template.Must(template.New("vkmsg").Parse(`{{- if .Title -}}{{.Title}}
{{end}}{{- if .Text -}}
{{.Text}}
{{end}}Источник VK: {{.Link}}`))

type tmplData struct {
	Title string
	Text  string
	Link  string
}

// BuildMessage builds a plain-text message for wall.post using text/template.
func BuildMessage(p sqldb.GetPostRow) string {
	title := typeTitle(p.Type)
	body := p.Text
	if len(body) > 3500 {
		body = body[:3500] + "…"
	}
	data := tmplData{
		Title: title,
		Text:  body,
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
