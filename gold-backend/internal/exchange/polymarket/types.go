package polymarket

import "encoding/json"

// subscriptionTopic describes a single topic subscription sent to the Polymarket WebSocket.
type subscriptionTopic struct {
	Topic    string               `json:"topic"`
	Type     string               `json:"type"`
	Filters  string               `json:"filters,omitempty"`
	ClobAuth *clobAuthCredentials `json:"clob_auth,omitempty"`
}

// clobAuthCredentials carries API credentials for authenticated topic subscriptions.
type clobAuthCredentials struct {
	ApiKey     string `json:"api_key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// subscribeRequest is serialised and sent to the server to initiate subscriptions.
type subscribeRequest struct {
	Action        string              `json:"action"` // "subscribe" or "unsubscribe"
	Subscriptions []subscriptionTopic `json:"subscriptions"`
}

// incomingMessage is the envelope for every message received from the server.
type incomingMessage struct {
	Topic   string          `json:"topic"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// cryptoPricePayload is the inner payload for topic "crypto_prices".
type cryptoPricePayload struct {
	Symbol    string `json:"symbol"`
	Value     string `json:"value"`
	Timestamp int64  `json:"timestamp"` // milliseconds since epoch
}

// activityPayload is the inner payload for topic "activity".
type activityPayload struct {
	EventType  string `json:"eventType"`
	MarketSlug string `json:"market_slug"`
	Side       string `json:"side"`
	Price      string `json:"price"`
	Size       string `json:"size"`
	Timestamp  int64  `json:"timestamp"` // milliseconds since epoch
	User       string `json:"user,omitempty"`
}
