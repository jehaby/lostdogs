package telegram

import (
	"net/http"
	"time"

	"github.com/go-telegram/bot"
)

type Client struct {
	Bot    *bot.Bot
	ChatID int64
}

func NewClient(token string, chatID int64) (*Client, error) {
	b, err := bot.New(token, bot.WithHTTPClient(10*time.Second, &http.Client{Timeout: 10 * time.Second}))
	if err != nil {
		return nil, err
	}
	return &Client{Bot: b, ChatID: chatID}, nil
}
