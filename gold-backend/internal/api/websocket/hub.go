package websocket

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

const (
	// defaultMetricsBroadcastInterval is how often the hub pushes a metrics snapshot
	// to all connected clients when HubConfig.MetricsBroadcastInterval is zero.
	defaultMetricsBroadcastInterval = 1 * time.Second

	// registrationChannelBuffer is the buffer for client register/unregister signals.
	registrationChannelBuffer = 64
)

// MetricsProvider is the interface portfolio.Manager satisfies.
type MetricsProvider interface {
	CurrentMetrics() domain.PortfolioMetrics
}

// HubConfig holds the input event channels and dependencies.
type HubConfig struct {
	CandleChannel             <-chan domain.Candle
	PositionOpenedChannel     <-chan domain.Position
	PositionClosedChannel     <-chan domain.Position // optional — nil channel never fires
	MetricsProvider           MetricsProvider
	DecisionChannel           <-chan domain.Decision // optional — nil channel never fires
	MetricsBroadcastInterval  time.Duration         // how often to push metric snapshots; 0 = 1s
	Logger                    *slog.Logger
}

// Hub manages connected WebSocket clients and broadcasts events to them.
// Call Run in a goroutine; mount HandleWebSocket on the router.
type Hub struct {
	config       HubConfig
	clients      map[string]*connectedClient
	clientsMu    sync.RWMutex
	registerCh   chan *connectedClient
	unregisterCh chan *connectedClient
	upgrader     websocket.Upgrader
	logger       *slog.Logger
}

// NewHub constructs a Hub from the provided config.
func NewHub(config HubConfig) *Hub {
	if config.MetricsBroadcastInterval == 0 {
		config.MetricsBroadcastInterval = defaultMetricsBroadcastInterval
	}

	// NOTE: CheckOrigin returns true for all origins. This is intentional for
	// local-only / development use. Restrict to specific origins in production.
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return &Hub{
		config:       config,
		clients:      make(map[string]*connectedClient),
		registerCh:   make(chan *connectedClient, registrationChannelBuffer),
		unregisterCh: make(chan *connectedClient, registrationChannelBuffer),
		upgrader:     upgrader,
		logger:       config.Logger,
	}
}

// HandleWebSocket is an http.HandlerFunc that upgrades the request to a WebSocket
// connection, creates a connectedClient, and registers it with the hub.
// Mount on the router as: r.Get("/ws/v1/stream", hub.HandleWebSocket)
func (hub *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := hub.upgrader.Upgrade(w, r, nil)
	if err != nil {
		hub.logger.Error("websocket: upgrade failed",
			"remote_addr", r.RemoteAddr,
			"error", err,
		)
		return
	}

	clientID, err := generateClientID()
	if err != nil {
		hub.logger.Error("websocket: failed to generate client ID",
			"remote_addr", r.RemoteAddr,
			"error", err,
		)
		conn.Close()
		return
	}

	client := &connectedClient{
		id:           clientID,
		connection:   conn,
		sendChannel:  make(chan OutboundMessage, sendChannelBuffer),
		symbolFilter: make(map[string]struct{}),
		logger:       hub.logger,
	}

	hub.logger.Info("websocket: client connecting",
		"client_id", clientID,
		"remote_addr", r.RemoteAddr,
	)

	hub.registerCh <- client

	// The context passed to the loops is derived from the request context so
	// that if the server shuts down the connection is closed cleanly.
	ctx := r.Context()

	writeDone := make(chan struct{})
	go func() {
		defer close(writeDone)
		client.startWriteLoop(ctx)
	}()

	// The read loop blocks; when it returns, the write loop will also exit
	// because startWriteLoop closes the connection on its exit which causes
	// the read loop to error. Wait for the write loop to finish after.
	client.startReadLoop(ctx, hub)
	<-writeDone
}

// Run starts the event broadcast loop. It reads from all configured input channels
// and forwards events to matching connected clients. It also handles client
// registration and deregistration, and pushes a metrics snapshot on every tick.
// Blocks until ctx is cancelled, returning ctx.Err().
func (hub *Hub) Run(ctx context.Context) error {
	metricsTicker := time.NewTicker(hub.config.MetricsBroadcastInterval)
	defer metricsTicker.Stop()

	// Use nil-channel idiom for optional channels: a nil channel in a select
	// case blocks forever and never fires, so absent channels are safe to leave nil.
	positionClosedCh := hub.config.PositionClosedChannel
	decisionCh := hub.config.DecisionChannel

	hub.logger.Info("websocket hub: starting run loop")

	for {
		select {
		case <-ctx.Done():
			hub.logger.Info("websocket hub: context cancelled, stopping",
				"reason", ctx.Err().Error(),
			)
			return ctx.Err()

		case client := <-hub.registerCh:
			hub.registerClient(client)

		case client := <-hub.unregisterCh:
			hub.unregisterClient(client)

		case candle, ok := <-hub.config.CandleChannel:
			if !ok {
				hub.logger.Info("websocket hub: candle channel closed")
				continue
			}
			hub.broadcastEvent(EventTypeCandleUpdate, candle, candle.Symbol)

		case position, ok := <-hub.config.PositionOpenedChannel:
			if !ok {
				hub.logger.Info("websocket hub: position opened channel closed")
				continue
			}
			hub.broadcastEvent(EventTypeTradeExecuted, position, position.Symbol)

		case position, ok := <-positionClosedCh:
			if !ok {
				hub.logger.Info("websocket hub: position closed channel closed")
				continue
			}
			hub.broadcastEvent(EventTypePositionClosed, position, position.Symbol)

		case decision, ok := <-decisionCh:
			if !ok {
				hub.logger.Info("websocket hub: decision channel closed")
				continue
			}
			hub.broadcastEvent(EventTypeDecisionMade, decision, decision.Symbol)

		case <-metricsTicker.C:
			if hub.config.MetricsProvider == nil {
				continue
			}
			metrics := hub.config.MetricsProvider.CurrentMetrics()
			// Empty symbol = no filter; all clients receive metrics updates.
			hub.broadcastEvent(EventTypeMetricUpdate, metrics, "")
		}
	}
}

// registerClient adds a client to the hub's client map.
func (hub *Hub) registerClient(client *connectedClient) {
	hub.clientsMu.Lock()
	defer hub.clientsMu.Unlock()

	hub.clients[client.id] = client
	hub.logger.Info("websocket hub: client registered",
		"client_id", client.id,
		"total_clients", len(hub.clients),
	)
}

// unregisterClient removes a client from the hub and closes its send channel.
// Calling this more than once for the same client is safe: the second call is
// a no-op if the client is no longer in the map.
func (hub *Hub) unregisterClient(client *connectedClient) {
	hub.clientsMu.Lock()
	defer hub.clientsMu.Unlock()

	if _, exists := hub.clients[client.id]; !exists {
		return
	}

	delete(hub.clients, client.id)
	close(client.sendChannel)

	hub.logger.Info("websocket hub: client unregistered",
		"client_id", client.id,
		"total_clients", len(hub.clients),
	)
}

// broadcastEvent sends an OutboundMessage to all connected clients that pass the
// symbol filter for this event. Non-blocking per client: if a client's send channel
// is full the message is dropped and a warning is logged rather than blocking the
// broadcast loop.
func (hub *Hub) broadcastEvent(eventType EventType, payload interface{}, symbol string) {
	msg := OutboundMessage{
		Type:    eventType,
		Payload: payload,
	}

	hub.clientsMu.RLock()
	defer hub.clientsMu.RUnlock()

	for _, client := range hub.clients {
		if !client.shouldReceive(symbol) {
			continue
		}

		select {
		case client.sendChannel <- msg:
		default:
			hub.logger.Warn("websocket hub: client send channel full, dropping message",
				"client_id", client.id,
				"event_type", eventType,
				"symbol", symbol,
			)
		}
	}
}

// generateClientID returns a 16-byte random hex string suitable for use as a
// unique client identifier.
func generateClientID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
