package vk

import (
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
	v := vkapi.NewVK(token)
	v.Client = &http.Client{Timeout: timeout}
	return &Client{VK: v, DestOwnerID: destOwnerID, FromGroup: fromGroup}, nil
}
