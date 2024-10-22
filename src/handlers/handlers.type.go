package handlers

type UserKeyBody struct {
	WalletID string `json:"wallet"`
	Key      string `json:"key"`
}
