package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// pingInterval is how often the write loop sends a ping to detect dead connections.
	pingInterval = 30 * time.Second

	// pongWait is how long to wait for a pong before considering the connection dead.
	// Must be greater than pingInterval.
	pongWait = 40 * time.Second

	// writeWait is the deadline for a single write operation.
	writeWait = 10 * time.Second

	// sendChannelBuffer is the number of outbound messages buffered per client.
	sendChannelBuffer = 256
)

// connectedClient represents a single connected dashboard client.
type connectedClient struct {
	id             string
	connection     *websocket.Conn
	sendChannel    chan OutboundMessage
	symbolFilter   map[string]struct{} // empty map = no filter (receive all)
	symbolFilterMu sync.RWMutex
	logger         *slog.Logger
}

// startReadLoop handles incoming messages (subscriptions) and detects disconnects.
// When the connection closes or an error occurs, it signals the hub to unregister
// and returns. The caller must start this in a goroutine.
func (client *connectedClient) startReadLoop(ctx context.Context, hub *Hub) {
	defer func() {
		hub.unregisterCh <- client
	}()

	// Extend the read deadline on every pong so the connection stays alive
	// as long as pongs keep arriving.
	client.connection.SetPongHandler(func(string) error {
		return client.connection.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Set the initial read deadline.
	if err := client.connection.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		client.logger.Error("websocket: failed to set initial read deadline",
			"client_id", client.id,
			"error", err,
		)
		return
	}

	for {
		_, raw, err := client.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived,
			) {
				client.logger.Warn("websocket: unexpected client disconnect",
					"client_id", client.id,
					"error", err,
				)
			} else {
				client.logger.Info("websocket: client disconnected",
					"client_id", client.id,
				)
			}
			return
		}

		var msg SubscribeMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			client.logger.Warn("websocket: received malformed subscription message",
				"client_id", client.id,
				"error", err,
			)
			continue
		}

		client.applySubscription(msg)
	}
}

// startWriteLoop drains sendChannel, writing each message as JSON.
// Sends a ping every pingInterval to detect dead connections.
// When the write loop exits (error or context cancellation), it closes the
// underlying connection so the read loop also unblocks and exits.
// The caller must start this in a goroutine.
func (client *connectedClient) startWriteLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		// Closing the connection here causes the read loop's ReadMessage to
		// return an error, triggering unregister from the read loop's defer.
		client.connection.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			// Send a close frame before shutting down.
			deadline := time.Now().Add(writeWait)
			_ = client.connection.SetWriteDeadline(deadline)
			_ = client.connection.WriteMessage(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"),
			)
			return

		case msg, ok := <-client.sendChannel:
			if !ok {
				// Hub closed the channel; send close and exit.
				deadline := time.Now().Add(writeWait)
				_ = client.connection.SetWriteDeadline(deadline)
				_ = client.connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.connection.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				client.logger.Error("websocket: failed to set write deadline",
					"client_id", client.id,
					"error", err,
				)
				return
			}

			if err := client.connection.WriteJSON(msg); err != nil {
				client.logger.Error("websocket: write failed, dropping client",
					"client_id", client.id,
					"error", err,
				)
				return
			}

		case <-ticker.C:
			if err := client.connection.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				client.logger.Error("websocket: failed to set ping write deadline",
					"client_id", client.id,
					"error", err,
				)
				return
			}

			if err := client.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				client.logger.Warn("websocket: ping failed, dropping client",
					"client_id", client.id,
					"error", err,
				)
				return
			}
		}
	}
}

// shouldReceive returns true if the client should receive an event for the given symbol.
// An empty filter (no subscriptions set) or an empty symbol means the client receives everything.
func (client *connectedClient) shouldReceive(symbol string) bool {
	if symbol == "" {
		return true
	}

	client.symbolFilterMu.RLock()
	defer client.symbolFilterMu.RUnlock()

	if len(client.symbolFilter) == 0 {
		return true
	}

	_, ok := client.symbolFilter[symbol]
	return ok
}

// applySubscription updates the client's symbol filter based on the incoming message.
func (client *connectedClient) applySubscription(msg SubscribeMessage) {
	client.symbolFilterMu.Lock()
	defer client.symbolFilterMu.Unlock()

	switch msg.Action {
	case "subscribe":
		for _, sym := range msg.Symbols {
			client.symbolFilter[sym] = struct{}{}
		}
		client.logger.Info("websocket: client updated subscription",
			"client_id", client.id,
			"action", "subscribe",
			"symbols", msg.Symbols,
		)

	case "unsubscribe":
		for _, sym := range msg.Symbols {
			delete(client.symbolFilter, sym)
		}
		client.logger.Info("websocket: client updated subscription",
			"client_id", client.id,
			"action", "unsubscribe",
			"symbols", msg.Symbols,
		)

	default:
		client.logger.Warn("websocket: unknown subscription action",
			"client_id", client.id,
			"action", msg.Action,
		)
	}
}
