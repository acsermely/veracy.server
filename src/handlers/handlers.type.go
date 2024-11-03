package handlers

import "time"

type UserKeyBody struct {
	WalletID string `json:"wallet"`
	Key      string `json:"key"`
}

type LoginKeyBody struct {
	WalletID string `json:"wallet"`
	Chal     string `json:"challange"`
}

const (
	JWT_COOKIE_EXPIRATION = 24 * time.Hour
)