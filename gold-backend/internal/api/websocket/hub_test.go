package websocket

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mockMetricsProvider satisfies MetricsProvider for tests.
type mockMetricsProvider struct {
	mu      sync.Mutex
	metrics domain.PortfolioMetrics
}

func (m *mockMetricsProvider) CurrentMetrics() domain.PortfolioMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.metrics
}

func (m *mockMetricsProvider) setMetrics(metrics domain.PortfolioMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = metrics
}

// newTestHub constructs a Hub wired to a set of in-memory channels.
type testHubChannels struct {
	candle          chan domain.Candle
	positionOpened  chan domain.Position
	positionClosed  chan domain.Position
	decision        chan domain.Decision
	metricsProvider *mockMetricsProvider
}

func newTestHub(t *testing.T, interval time.Duration) (*Hub, *testHubChannels) {
	t.Helper()

	chans := &testHubChannels{
		candle:          make(chan domain.Candle, 8),
		positionOpened:  make(chan domain.Position, 8),
		positionClosed:  make(chan domain.Position, 8),
		decision:        make(chan domain.Decision, 8),
		metricsProvider: &mockMetricsProvider{},
	}

	if interval == 0 {
		interval = 50 * time.Millisecond
	}

	hub := NewHub(HubConfig{
		CandleChannel:            chans.candle,
		PositionOpenedChannel:    chans.positionOpened,
		PositionClosedChannel:    chans.positionClosed,
		MetricsProvider:          chans.metricsProvider,
		DecisionChannel:          chans.decision,
		MetricsBroadcastInterval: interval,
		Logger:                   newDiscardLogger(),
	})

	return hub, chans
}

// newDiscardLogger returns a slog.Logger that discards all output.
func newDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// makeClient creates a connectedClient with a buffered send channel and no real
// WebSocket connection. Useful for unit tests that don't need a live connection.
func makeClient(id string) *connectedClient {
	return &connectedClient{
		id:           id,
		sendChannel:  make(chan OutboundMessage, sendChannelBuffer),
		symbolFilter: make(map[string]struct{}),
		logger:       newDiscardLogger(),
	}
}

// drainOne reads from a sendChannel with a timeout. Returns (msg, true) on success
// or (zero, false) on timeout.
func drainOne(ch <-chan OutboundMessage, timeout time.Duration) (OutboundMessage, bool) {
	select {
	case msg, ok := <-ch:
		if !ok {
			return OutboundMessage{}, false
		}
		return msg, true
	case <-time.After(timeout):
		return OutboundMessage{}, false
	}
}

// ---------------------------------------------------------------------------
// Unit tests — client map operations (no real WebSocket)
// ---------------------------------------------------------------------------

func TestBroadcastEvent_DeliveredToMatchingClients(t *testing.T) {
	hub, _ := newTestHub(t, 0)

	c1 := makeClient("c1")
	c2 := makeClient("c2")

	hub.registerClient(c1)
	hub.registerClient(c2)

	hub.broadcastEvent(EventTypeCandleUpdate, domain.Candle{Symbol: "BTCUSDT"}, "BTCUSDT")

	for _, tc := range []struct {
		name   string
		client *connectedClient
	}{
		{"c1", c1},
		{"c2", c2},
	} {
		msg, ok := drainOne(tc.client.sendChannel, 100*time.Millisecond)
		if !ok {
			t.Fatalf("%s: expected to receive a message but got none", tc.name)
		}
		if msg.Type != EventTypeCandleUpdate {
			t.Errorf("%s: expected event type %q, got %q", tc.name, EventTypeCandleUpdate, msg.Type)
		}
	}
}

func TestBroadcastEvent_FilteredBySymbol(t *testing.T) {
	hub, _ := newTestHub(t, 0)

	btcClient := makeClient("btc-only")
	btcClient.symbolFilter["BTCUSDT"] = struct{}{}

	hub.registerClient(btcClient)

	// Broadcast an ETHUSDT event — btcClient must NOT receive it.
	hub.broadcastEvent(EventTypeCandleUpdate, domain.Candle{Symbol: "ETHUSDT"}, "ETHUSDT")

	_, received := drainOne(btcClient.sendChannel, 50*time.Millisecond)
	if received {
		t.Error("expected no message for ETHUSDT event but client received one")
	}

	// Now broadcast for BTCUSDT — btcClient MUST receive it.
	hub.broadcastEvent(EventTypeCandleUpdate, domain.Candle{Symbol: "BTCUSDT"}, "BTCUSDT")

	msg, ok := drainOne(btcClient.sendChannel, 100*time.Millisecond)
	if !ok {
		t.Fatal("expected BTCUSDT message but got none")
	}
	if msg.Type != EventTypeCandleUpdate {
		t.Errorf("expected event type %q, got %q", EventTypeCandleUpdate, msg.Type)
	}
}

func TestBroadcastEvent_FullChannelDropsAndLogs(t *testing.T) {
	hub, _ := newTestHub(t, 0)

	// Create a client with a size-1 channel so we can fill it easily.
	client := &connectedClient{
		id:           "full-client",
		sendChannel:  make(chan OutboundMessage, 1),
		symbolFilter: make(map[string]struct{}),
		logger:       newDiscardLogger(),
	}

	hub.registerClient(client)

	// Fill the channel.
	client.sendChannel <- OutboundMessage{Type: EventTypeCandleUpdate}

	// This should not block even though the channel is full.
	done := make(chan struct{})
	go func() {
		hub.broadcastEvent(EventTypeCandleUpdate, domain.Candle{Symbol: "BTCUSDT"}, "BTCUSDT")
		close(done)
	}()

	select {
	case <-done:
		// Good — broadcastEvent returned without blocking.
	case <-time.After(200 * time.Millisecond):
		t.Fatal("broadcastEvent blocked on a full client channel")
	}
}

func TestUnregister_ClosesClient(t *testing.T) {
	hub, _ := newTestHub(t, 0)

	client := makeClient("to-remove")
	hub.registerClient(client)
	hub.unregisterClient(client)

	// After unregister the send channel must be closed.
	select {
	case _, open := <-client.sendChannel:
		if open {
			t.Error("expected closed channel after unregister, but channel is still open")
		}
	default:
		t.Error("expected closed channel to be immediately readable, but got nothing")
	}

	// Calling unregisterClient a second time must not panic (double-close guard).
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("second unregister panicked: %v", r)
			}
		}()
		hub.unregisterClient(client)
	}()
}

// ---------------------------------------------------------------------------
// Run loop integration tests
// ---------------------------------------------------------------------------

func TestRun_ContextCancelStopsHub(t *testing.T) {
	hub, _ := newTestHub(t, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan error, 1)
	go func() {
		runDone <- hub.Run(ctx)
	}()

	cancel()

	select {
	case err := <-runDone:
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("hub.Run did not stop after context cancellation")
	}
}

func TestRun_BroadcastsCandleEvents(t *testing.T) {
	hub, chans := newTestHub(t, 10*time.Second) // long interval so metrics tick doesn't interfere

	c1 := makeClient("c1")
	c2 := makeClient("c2")
	hub.registerClient(c1)
	hub.registerClient(c2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	candle := domain.Candle{Symbol: "BTCUSDT"}
	chans.candle <- candle

	for _, tc := range []struct {
		name   string
		client *connectedClient
	}{
		{"c1", c1},
		{"c2", c2},
	} {
		msg, ok := drainOne(tc.client.sendChannel, 500*time.Millisecond)
		if !ok {
			t.Fatalf("%s: expected candle event but got none", tc.name)
		}
		if msg.Type != EventTypeCandleUpdate {
			t.Errorf("%s: expected %q, got %q", tc.name, EventTypeCandleUpdate, msg.Type)
		}
	}
}

func TestRun_BroadcastsMetricsOnTick(t *testing.T) {
	hub, chans := newTestHub(t, 50*time.Millisecond)

	metrics := domain.PortfolioMetrics{TotalTrades: 42}
	chans.metricsProvider.setMetrics(metrics)

	client := makeClient("metrics-receiver")
	hub.registerClient(client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	msg, ok := drainOne(client.sendChannel, 500*time.Millisecond)
	if !ok {
		t.Fatal("expected metrics update but got none within timeout")
	}
	if msg.Type != EventTypeMetricUpdate {
		t.Errorf("expected %q, got %q", EventTypeMetricUpdate, msg.Type)
	}

	// Verify the payload round-trips to the right metrics.
	raw, err := json.Marshal(msg.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var got domain.PortfolioMetrics
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if got.TotalTrades != 42 {
		t.Errorf("expected TotalTrades=42, got %d", got.TotalTrades)
	}
}

// ---------------------------------------------------------------------------
// Integration test — real WebSocket via httptest.Server
// ---------------------------------------------------------------------------

func TestHandleWebSocket_UpgradesAndRegisters(t *testing.T) {
	hub, chans := newTestHub(t, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the hub run loop.
	runDone := make(chan error, 1)
	go func() {
		runDone <- hub.Run(ctx)
	}()

	// Spin up a real HTTP test server.
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	// Dial as a WebSocket client.
	wsURL := "ws" + server.URL[len("http"):]
	conn, resp, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v (status %v)", err, resp)
	}
	defer conn.Close()

	// Allow time for the registration to propagate through registerCh.
	time.Sleep(50 * time.Millisecond)

	hub.clientsMu.RLock()
	count := len(hub.clients)
	hub.clientsMu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 registered client, got %d", count)
	}

	// Push a candle event and verify the real WS client receives it.
	// The metrics ticker fires at 50ms so we may receive a metric_update before
	// the candle_update — drain messages until we find the candle or time out.
	candle := domain.Candle{Symbol: "BTCUSDT"}
	chans.candle <- candle

	deadline := time.Now().Add(1 * time.Second)
	found := false
	for time.Now().Before(deadline) {
		if err := conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}

		var received OutboundMessage
		if err := json.Unmarshal(raw, &received); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if received.Type == EventTypeCandleUpdate {
			found = true
			break
		}
		// Received a different event (e.g. metric_update); keep reading.
	}

	if !found {
		t.Errorf("never received a %q event within timeout", EventTypeCandleUpdate)
	}

	// Clean disconnect.
	conn.WriteMessage(gorillaws.CloseMessage,
		gorillaws.FormatCloseMessage(gorillaws.CloseNormalClosure, ""))

	cancel()

	select {
	case <-runDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("hub.Run did not stop after context cancellation")
	}
}
