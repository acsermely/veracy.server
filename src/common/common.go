package common

const (
	// ARWEAVE_URL             = "https://arweave.net"
	ARWEAVE_URL             = "https://node2.irys.xyz"
	TX_APP_CONTENT_TYPE     = "application/json"
	TX_APP_VERSION          = "0.0.4"
	TX_APP_NAME             = "VeracyApp"
	TX_TYPE_POST            = "post"
	TX_TYPE_PAYMENT         = "payment"
	TX_POST_PRIVACY_PRIVATE = "PRIVATE"
	TX_POST_PRIVACY_PUBLIC  = "PUBLIC"
	TX_POST_TYPE_IMG        = "IMG"
	TX_POST_TYPE_TEXT       = "TEXT"
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

type Post struct {
	ID       string        `json:"id"`
	Content  []PostContent `json:"content"`
	Title    *string       `json:"title,omitempty"`
	Tags     *[]string     `json:"tags,omitempty"`
	Uploader string        `json:"uploader"`
	Price    *int32        `json:"price,omitempty"`
}

type PostContent struct {
	Type    string  `json:"type"`
	Privacy string  `json:"privacy"`
	Data    string  `json:"data"`
	Align   *string `json:"align,omitempty"`
}
