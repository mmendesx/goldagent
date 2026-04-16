package execution

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// wsUpgrader upgrades HTTP connections to WebSocket in tests.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// newTestWebSocketServer creates an httptest.Server that upgrades connections
// and calls the provided handler for each connection.
func newTestWebSocketServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("websocket upgrade error: %v", err)
			return
		}
		defer conn.Close()
		handler(conn)
	}))
	return server
}

// wsURL converts an http:// URL to a ws:// URL for use with the gorilla dialer.
func wsURL(serverURL string) string {
	return strings.Replace(serverURL, "http://", "ws://", 1)
}

// TestBinanceOrderClient_SuccessfulOrder verifies that PlaceMarketOrder sends a correctly shaped
// signed request and correctly parses a successful fill response.
func TestBinanceOrderClient_SuccessfulOrder(t *testing.T) {
	const (
		apiKey    = "testApiKey"
		apiSecret = "testApiSecret"
		symbol    = "BTCUSDT"
	)

	quantity := decimal.NewFromFloat(0.001)

	// Canned Binance response for a successful market order.
	successResponse := map[string]interface{}{
		"status": 200,
		"result": map[string]interface{}{
			"symbol":       "BTCUSDT",
			"orderId":      int64(99001),
			"status":       "FILLED",
			"origQty":      "0.001",
			"executedQty":  "0.001",
			"transactTime": int64(1234567890000),
			"fills": []map[string]interface{}{
				{
					"price":           "65000.00",
					"qty":             "0.0005",
					"commission":      "0.0000005",
					"commissionAsset": "BTC",
				},
				{
					"price":           "65100.00",
					"qty":             "0.0005",
					"commission":      "0.0000005",
					"commissionAsset": "BTC",
				},
			},
		},
	}

	server := newTestWebSocketServer(t, func(conn *websocket.Conn) {
		// Read the incoming order request.
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("server: read message error: %v", err)
			return
		}

		// Parse and validate the request shape.
		var req orderRequestMessage
		if err := json.Unmarshal(msgBytes, &req); err != nil {
			t.Errorf("server: unmarshal request: %v", err)
			return
		}

		if req.Method != "order.place" {
			t.Errorf("server: expected method %q, got %q", "order.place", req.Method)
		}
		if req.Params["symbol"] != symbol {
			t.Errorf("server: expected symbol %q, got %q", symbol, req.Params["symbol"])
		}
		if req.Params["side"] != "BUY" {
			t.Errorf("server: expected side BUY, got %q", req.Params["side"])
		}
		if req.Params["type"] != "MARKET" {
			t.Errorf("server: expected type MARKET, got %q", req.Params["type"])
		}
		if req.Params["apiKey"] != apiKey {
			t.Errorf("server: expected apiKey %q, got %q", apiKey, req.Params["apiKey"])
		}
		if req.Params["timestamp"] == "" {
			t.Error("server: expected non-empty timestamp in params")
		}
		if req.Params["signature"] == "" {
			t.Error("server: expected non-empty signature in params")
		}

		// Verify the signature is correct by recomputing it.
		paramsWithoutSig := make(map[string]string)
		for k, v := range req.Params {
			if k != "signature" {
				paramsWithoutSig[k] = v
			}
		}
		expectedSig := SignBinanceParams(paramsWithoutSig, apiSecret)
		if req.Params["signature"] != expectedSig {
			t.Errorf("server: invalid signature: got %q, want %q", req.Params["signature"], expectedSig)
		}

		// Send the success response with the matching request ID.
		successResponse["id"] = req.ID
		respBytes, _ := json.Marshal(successResponse)
		if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
			t.Errorf("server: write response error: %v", err)
		}
	})
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewBinanceOrderClient(BinanceOrderClientConfig{
		WebSocketApiUrl: wsURL(server.URL),
		ApiKey:          apiKey,
		ApiSecret:       apiSecret,
		Logger:          logger,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	result, err := client.PlaceMarketOrder(ctx, symbol, domain.OrderSideBuy, quantity)
	if err != nil {
		t.Fatalf("PlaceMarketOrder returned unexpected error: %v", err)
	}

	if result.OrderID != 99001 {
		t.Errorf("OrderID: got %d, want 99001", result.OrderID)
	}
	if result.Symbol != "BTCUSDT" {
		t.Errorf("Symbol: got %q, want BTCUSDT", result.Symbol)
	}
	if result.Status != "FILLED" {
		t.Errorf("Status: got %q, want FILLED", result.Status)
	}

	// Average fill price = (65000 * 0.0005 + 65100 * 0.0005) / 0.001 = 65050
	expectedAvgPrice := decimal.NewFromFloat(65050.0)
	if !result.AverageFillPrice.Equal(expectedAvgPrice) {
		t.Errorf("AverageFillPrice: got %s, want %s", result.AverageFillPrice, expectedAvgPrice)
	}

	// Total commission = 0.0000005 + 0.0000005 = 0.000001
	expectedCommission := decimal.NewFromFloat(0.000001)
	if !result.TotalCommission.Equal(expectedCommission) {
		t.Errorf("TotalCommission: got %s, want %s", result.TotalCommission, expectedCommission)
	}

	if result.CommissionAsset != "BTC" {
		t.Errorf("CommissionAsset: got %q, want BTC", result.CommissionAsset)
	}

	if len(result.RawResponse) == 0 {
		t.Error("RawResponse should be populated with the raw JSON bytes")
	}
}

// TestBinanceOrderClient_BinanceRejection verifies that a non-200 Binance response
// causes PlaceMarketOrder to return a descriptive error.
func TestBinanceOrderClient_BinanceRejection(t *testing.T) {
	server := newTestWebSocketServer(t, func(conn *websocket.Conn) {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("server: read message error: %v", err)
			return
		}

		var req orderRequestMessage
		if err := json.Unmarshal(msgBytes, &req); err != nil {
			t.Errorf("server: unmarshal request: %v", err)
			return
		}

		errorResponse := map[string]interface{}{
			"id":     req.ID,
			"status": 400,
			"error": map[string]interface{}{
				"code": -2010,
				"msg":  "Account has insufficient balance",
			},
		}
		respBytes, _ := json.Marshal(errorResponse)
		conn.WriteMessage(websocket.TextMessage, respBytes)
	})
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := NewBinanceOrderClient(BinanceOrderClientConfig{
		WebSocketApiUrl: wsURL(server.URL),
		ApiKey:          "key",
		ApiSecret:       "secret",
		Logger:          logger,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	_, err := client.PlaceMarketOrder(ctx, "BTCUSDT", domain.OrderSideBuy, decimal.NewFromFloat(0.001))
	if err == nil {
		t.Fatal("expected an error for rejected order, got nil")
	}

	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Errorf("expected error to contain 'insufficient balance', got: %v", err)
	}
}

// TestBinanceOrderClient_AverageFillPriceCalculation verifies the weighted average
// fill price computation for a multi-fill scenario.
func TestBinanceOrderClient_AverageFillPriceCalculation(t *testing.T) {
	fills := []orderFillPayload{
		{Price: "100.00", Qty: "1.0", Commission: "0.001", CommissionAsset: "USDT"},
		{Price: "200.00", Qty: "3.0", Commission: "0.003", CommissionAsset: "USDT"},
	}

	avgPrice, totalCommission, commissionAsset, err := computeFillMetrics(fills)
	if err != nil {
		t.Fatalf("computeFillMetrics returned error: %v", err)
	}

	// Average = (100 * 1 + 200 * 3) / (1 + 3) = (100 + 600) / 4 = 175
	expectedAvg := decimal.NewFromFloat(175.0)
	if !avgPrice.Equal(expectedAvg) {
		t.Errorf("AverageFillPrice: got %s, want %s", avgPrice, expectedAvg)
	}

	expectedCommission := decimal.NewFromFloat(0.004)
	if !totalCommission.Equal(expectedCommission) {
		t.Errorf("TotalCommission: got %s, want %s", totalCommission, expectedCommission)
	}

	if commissionAsset != "USDT" {
		t.Errorf("CommissionAsset: got %q, want USDT", commissionAsset)
	}
}

// TestBinanceOrderClient_EmptyFills verifies that zero fills returns zero values without error.
func TestBinanceOrderClient_EmptyFills(t *testing.T) {
	avgPrice, totalCommission, commissionAsset, err := computeFillMetrics(nil)
	if err != nil {
		t.Fatalf("computeFillMetrics returned error for empty fills: %v", err)
	}
	if !avgPrice.IsZero() {
		t.Errorf("expected zero avg price for empty fills, got %s", avgPrice)
	}
	if !totalCommission.IsZero() {
		t.Errorf("expected zero commission for empty fills, got %s", totalCommission)
	}
	if commissionAsset != "" {
		t.Errorf("expected empty commission asset for empty fills, got %q", commissionAsset)
	}
}
