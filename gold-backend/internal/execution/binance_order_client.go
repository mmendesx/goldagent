package execution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

const orderResponseTimeout = 30 * time.Second

// BinanceOrderClientConfig holds the configuration for the Binance WebSocket order client.
type BinanceOrderClientConfig struct {
	WebSocketApiUrl string
	ApiKey          string
	ApiSecret       string
	Logger          *slog.Logger
}

// BinanceOrderResult is the parsed result of a successfully placed market order.
type BinanceOrderResult struct {
	OrderID          int64
	Symbol           string
	Status           string
	OriginalQuantity decimal.Decimal
	ExecutedQuantity decimal.Decimal
	AverageFillPrice decimal.Decimal
	TotalCommission  decimal.Decimal
	CommissionAsset  string
	RawResponse      []byte
}

// BinanceOrderClient maintains a persistent WebSocket connection to the Binance Order API
// and provides synchronous (blocking) order placement with per-request response routing.
type BinanceOrderClient struct {
	config           BinanceOrderClientConfig
	connection       *websocket.Conn
	pendingResponses map[string]chan orderResponseMessage
	mu               sync.Mutex // protects pendingResponses map
	writeMu          sync.Mutex // serializes writes to the WebSocket connection
}

// NewBinanceOrderClient constructs a BinanceOrderClient. Does not dial — call Connect first.
func NewBinanceOrderClient(config BinanceOrderClientConfig) *BinanceOrderClient {
	return &BinanceOrderClient{
		config:           config,
		pendingResponses: make(map[string]chan orderResponseMessage),
	}
}

// Connect dials the Binance WebSocket API endpoint and starts the read loop goroutine.
// Returns once the connection is established.
func (client *BinanceOrderClient) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{}
	conn, _, err := dialer.DialContext(ctx, client.config.WebSocketApiUrl, nil)
	if err != nil {
		return fmt.Errorf("connect to Binance WebSocket API at %q: %w", client.config.WebSocketApiUrl, err)
	}

	client.connection = conn
	go client.readLoop()
	return nil
}

// Close gracefully closes the WebSocket connection.
func (client *BinanceOrderClient) Close() error {
	if client.connection == nil {
		return nil
	}

	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	err := client.connection.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	if err != nil {
		client.config.Logger.Warn("binance order client: error sending close frame", "error", err)
	}
	return client.connection.Close()
}

// PlaceMarketOrder places a signed MARKET order on Binance and blocks until the response
// arrives (up to 30 seconds). Returns a parsed BinanceOrderResult on success, or an error
// on Binance-side rejection, timeout, or transport failure.
func (client *BinanceOrderClient) PlaceMarketOrder(
	ctx context.Context,
	symbol string,
	side domain.OrderSide,
	quantity decimal.Decimal,
) (BinanceOrderResult, error) {
	requestID, err := generateRequestID()
	if err != nil {
		return BinanceOrderResult{}, fmt.Errorf("generate request id: %w", err)
	}

	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())

	params := map[string]string{
		"symbol":    symbol,
		"side":      string(side),
		"type":      "MARKET",
		"quantity":  quantity.String(),
		"apiKey":    client.config.ApiKey,
		"timestamp": timestamp,
	}
	params["signature"] = SignBinanceParams(params, client.config.ApiSecret)

	request := orderRequestMessage{
		ID:     requestID,
		Method: "order.place",
		Params: params,
	}

	responseChan := make(chan orderResponseMessage, 1)

	client.mu.Lock()
	client.pendingResponses[requestID] = responseChan
	client.mu.Unlock()

	defer func() {
		client.mu.Lock()
		delete(client.pendingResponses, requestID)
		client.mu.Unlock()
	}()

	requestBytes, err := json.Marshal(request)
	if err != nil {
		return BinanceOrderResult{}, fmt.Errorf("marshal order request for symbol %q: %w", symbol, err)
	}

	client.writeMu.Lock()
	writeErr := client.connection.WriteMessage(websocket.TextMessage, requestBytes)
	client.writeMu.Unlock()

	if writeErr != nil {
		return BinanceOrderResult{}, fmt.Errorf("send order request for symbol %q to Binance: %w", symbol, writeErr)
	}

	client.config.Logger.Info("binance order client: order request sent",
		"requestId", requestID,
		"symbol", symbol,
		"side", side,
		"quantity", quantity.String(),
	)

	select {
	case response, ok := <-responseChan:
		if !ok {
			return BinanceOrderResult{}, fmt.Errorf("binance order client: connection closed while waiting for response to request %q", requestID)
		}
		return parseOrderResponse(response)

	case <-time.After(orderResponseTimeout):
		return BinanceOrderResult{}, fmt.Errorf("binance order client: timeout waiting for response to request %q (symbol %q, side %q)", requestID, symbol, side)

	case <-ctx.Done():
		return BinanceOrderResult{}, fmt.Errorf("binance order client: context cancelled while waiting for response to request %q: %w", requestID, ctx.Err())
	}
}

// readLoop runs in a dedicated goroutine, reading all incoming WebSocket messages and
// routing each response to the appropriate pending channel by request ID.
// Exits when the connection is closed or returns an error; drains all pending channels.
func (client *BinanceOrderClient) readLoop() {
	defer func() {
		// Close all pending channels so PlaceMarketOrder callers don't hang.
		client.mu.Lock()
		for id, ch := range client.pendingResponses {
			close(ch)
			delete(client.pendingResponses, id)
		}
		client.mu.Unlock()
	}()

	for {
		_, messageBytes, err := client.connection.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				client.config.Logger.Error("binance order client: read loop error", "error", err)
			}
			return
		}

		var response orderResponseMessage
		if parseErr := json.Unmarshal(messageBytes, &response); parseErr != nil {
			client.config.Logger.Error("binance order client: failed to parse response JSON",
				"error", parseErr,
				"raw", string(messageBytes),
			)
			continue
		}
		response.RawBytes = messageBytes

		client.mu.Lock()
		ch, found := client.pendingResponses[response.ID]
		client.mu.Unlock()

		if !found {
			client.config.Logger.Warn("binance order client: received response for unknown request id",
				"requestId", response.ID,
			)
			continue
		}

		ch <- response
	}
}

// parseOrderResponse converts a raw orderResponseMessage into a BinanceOrderResult.
// Returns an error if the Binance status indicates failure or the result cannot be parsed.
func parseOrderResponse(response orderResponseMessage) (BinanceOrderResult, error) {
	if response.Status != 200 {
		if response.Error != nil {
			return BinanceOrderResult{}, fmt.Errorf(
				"binance order rejected: code %d — %s",
				response.Error.Code,
				response.Error.Msg,
			)
		}
		return BinanceOrderResult{}, fmt.Errorf(
			"binance order rejected: HTTP status %d (no error detail)",
			response.Status,
		)
	}

	var result orderResultPayload
	if err := json.Unmarshal(response.Result, &result); err != nil {
		return BinanceOrderResult{}, fmt.Errorf("parse order result payload: %w", err)
	}

	originalQty, err := decimal.NewFromString(result.OrigQty)
	if err != nil {
		return BinanceOrderResult{}, fmt.Errorf("parse origQty %q: %w", result.OrigQty, err)
	}

	executedQty, err := decimal.NewFromString(result.ExecutedQty)
	if err != nil {
		return BinanceOrderResult{}, fmt.Errorf("parse executedQty %q: %w", result.ExecutedQty, err)
	}

	averageFillPrice, totalCommission, commissionAsset, err := computeFillMetrics(result.Fills)
	if err != nil {
		return BinanceOrderResult{}, err
	}

	return BinanceOrderResult{
		OrderID:          result.OrderID,
		Symbol:           result.Symbol,
		Status:           result.Status,
		OriginalQuantity: originalQty,
		ExecutedQuantity: executedQty,
		AverageFillPrice: averageFillPrice,
		TotalCommission:  totalCommission,
		CommissionAsset:  commissionAsset,
		RawResponse:      response.RawBytes,
	}, nil
}

// computeFillMetrics calculates average fill price and total commission across all fills.
// Average fill price = sum(price * qty) / sum(qty). Guards against zero total quantity.
func computeFillMetrics(fills []orderFillPayload) (averageFillPrice, totalCommission decimal.Decimal, commissionAsset string, err error) {
	var totalValue decimal.Decimal
	var totalQty decimal.Decimal

	for i, fill := range fills {
		price, parseErr := decimal.NewFromString(fill.Price)
		if parseErr != nil {
			return decimal.Zero, decimal.Zero, "", fmt.Errorf("parse fill[%d].price %q: %w", i, fill.Price, parseErr)
		}
		qty, parseErr := decimal.NewFromString(fill.Qty)
		if parseErr != nil {
			return decimal.Zero, decimal.Zero, "", fmt.Errorf("parse fill[%d].qty %q: %w", i, fill.Qty, parseErr)
		}
		commission, parseErr := decimal.NewFromString(fill.Commission)
		if parseErr != nil {
			return decimal.Zero, decimal.Zero, "", fmt.Errorf("parse fill[%d].commission %q: %w", i, fill.Commission, parseErr)
		}

		totalValue = totalValue.Add(price.Mul(qty))
		totalQty = totalQty.Add(qty)
		totalCommission = totalCommission.Add(commission)

		if fill.CommissionAsset != "" {
			commissionAsset = fill.CommissionAsset
		}
	}

	if totalQty.IsZero() {
		return decimal.Zero, totalCommission, commissionAsset, nil
	}

	averageFillPrice = totalValue.Div(totalQty)
	return averageFillPrice, totalCommission, commissionAsset, nil
}

// generateRequestID creates a random 16-byte request ID as a hex string.
// Uses crypto/rand so IDs are unpredictable and collision-resistant.
func generateRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random request id: %w", err)
	}
	return hex.EncodeToString(b), nil
}
