package execution

import "encoding/json"

// orderRequestMessage is the JSON envelope sent to the Binance WebSocket API.
type orderRequestMessage struct {
	ID     string            `json:"id"`
	Method string            `json:"method"`
	Params map[string]string `json:"params"`
}

// orderResponseMessage is the JSON envelope received from the Binance WebSocket API.
// RawBytes is populated by the read loop with the original message bytes for DB storage.
type orderResponseMessage struct {
	ID       string             `json:"id"`
	Status   int                `json:"status"`
	Result   json.RawMessage    `json:"result,omitempty"`
	Error    *orderErrorPayload `json:"error,omitempty"`
	RawBytes []byte             `json:"-"`
}

// orderErrorPayload carries the Binance error code and human-readable message.
type orderErrorPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// orderResultPayload is the "result" object inside a successful order response.
type orderResultPayload struct {
	Symbol       string             `json:"symbol"`
	OrderID      int64              `json:"orderId"`
	Status       string             `json:"status"`
	OrigQty      string             `json:"origQty"`
	ExecutedQty  string             `json:"executedQty"`
	TransactTime int64              `json:"transactTime"`
	Fills        []orderFillPayload `json:"fills"`
}

// orderFillPayload represents a single fill within an order result.
type orderFillPayload struct {
	Price           string `json:"price"`
	Qty             string `json:"qty"`
	Commission      string `json:"commission"`
	CommissionAsset string `json:"commissionAsset"`
}
