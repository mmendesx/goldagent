package websocket

// EventType is the type discriminator for outbound messages.
type EventType string

const (
	EventTypeCandleUpdate   EventType = "candle_update"
	EventTypePositionUpdate EventType = "position_update"
	EventTypePositionClosed EventType = "position_closed"
	EventTypeMetricUpdate   EventType = "metric_update"
	EventTypeTradeExecuted  EventType = "trade_executed"
	EventTypeDecisionMade   EventType = "decision_made"
)

// OutboundMessage is the JSON envelope sent to dashboard clients.
type OutboundMessage struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

// SubscribeMessage is what clients send to subscribe to a symbol filter.
// If Symbols is empty, the client receives all events.
type SubscribeMessage struct {
	Action  string   `json:"action"`  // "subscribe" or "unsubscribe"
	Symbols []string `json:"symbols"` // filter; empty = all
}
