package vk

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	vkapi "github.com/SevereCloud/vksdk/v3/api"
)

type Client struct {
	VK          *vkapi.VK
	DestOwnerID int64
	FromGroup   bool
}

func NewClient(token string, destOwnerID int64, fromGroup bool, timeout time.Duration) (*Client, error) {
	slog.Info("creating new vk client: ", "token", tokenForLogging(token), "fromGroup", fromGroup, "destOwnerID", destOwnerID)
	v := vkapi.NewVK(token)
	v.Client = &http.Client{Timeout: timeout}
	return &Client{VK: v, DestOwnerID: destOwnerID, FromGroup: fromGroup}, nil
}

func tokenForLogging(token string) string {
	if len(token) < 5 {
		return "******"
	}
	return fmt.Sprintf("%s***%s", token[:2], token[len(token)-3:])
}
