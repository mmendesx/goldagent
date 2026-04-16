package binance

import "encoding/json"

// combinedStreamMessage is the outer envelope Binance sends on combined stream connections.
// The stream field identifies which stream the data belongs to (e.g., "btcusdt@kline_5m").
type combinedStreamMessage struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// klineMessage is the parsed body of a kline/candlestick event.
type klineMessage struct {
	EventType string       `json:"e"`
	EventTime int64        `json:"E"`
	Symbol    string       `json:"s"`
	Kline     klinePayload `json:"k"`
}

// klinePayload holds the candlestick data fields from the Binance kline stream.
// All price and volume fields are string-encoded decimals as Binance sends them.
type klinePayload struct {
	OpenTime    int64  `json:"t"`
	CloseTime   int64  `json:"T"`
	Symbol      string `json:"s"`
	Interval    string `json:"i"`
	Open        string `json:"o"`
	Close       string `json:"c"`
	High        string `json:"h"`
	Low         string `json:"l"`
	Volume      string `json:"v"`
	QuoteVolume string `json:"q"`
	TradeCount  int    `json:"n"`
	IsClosed    bool   `json:"x"`
}

// tradeMessage is the parsed body of an individual trade event.
// Price fields are string-encoded decimals.
type tradeMessage struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"t"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Timestamp int64  `json:"T"`
}

// subscribeMessage is the JSON payload sent to Binance to subscribe to streams
// after the connection is established.
type subscribeMessage struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int      `json:"id"`
}
