package handlers

import (
	"time"

	"github.com/acsermely/veracy.server/src/db"
)

type UserKeyBody struct {
	WalletID string `json:"wallet"`
	Key      string `json:"key"`
}

type LoginKeyBody struct {
	WalletID string `json:"wallet"`
	Chal     string `json:"challange"`
}

type LoginAdminBody struct {
	Chal string `json:"challange"`
}

const (
	JWT_COOKIE_EXPIRATION = 24 * time.Hour
)

type ImageData struct {
	Id     int    `json:"id"`
	Wallet string `json:"address"`
	Post   string `json:"postId"`
	Data   []byte `json:"data"`
	Active bool   `json:"active"`
}

type SetImageActiveBody struct {
	Id     int    `json:"id"`
	Wallet string `json:"address"`
	Post   string `json:"postId"`
	Active bool   `json:"active"`
}

type FeedbackBody struct {
	Type    string `json:"feedbackType"`
	Target  string `json:"target"`
	Content string `json:"content"`
}

type SendMessageRequest struct {
	Recipient string `json:"recipient"`
	Message   string `json:"message"`
}

type GetMessagesResponse struct {
	Messages []db.InboxMessage `json:"messages"`
}

type MessageSavedRequest struct {
	Timestamps []time.Time `json:"timestamps"`
}
