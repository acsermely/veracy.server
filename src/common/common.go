package common

const (
	ARWEAVE_URL         = "https://arweave.net"
	TX_APP_CONTENT_TYPE = "application/json"
	TX_APP_VERSION      = "0.0.3"
	TX_APP_NAME         = "Test123"
	TX_TYPE_POST        = "post"
	TX_TYPE_PAYMENT     = "payment"
)

type Owner struct {
	Address string `json:"address"`
}

type Node struct {
	ID        string `json:"id"`
	Recipient string `json:"recipient"`
	Owner     Owner  `json:"owner"`
}

type Edge struct {
	Node Node `json:"node"`
}

type Transactions struct {
	Edges []Edge `json:"edges"`
}

type Data struct {
	Transactions Transactions `json:"transactions"`
}

type ArQueryResult struct {
	Data Data `json:"data"`
}
