package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Candle struct {
	ID          int64           `json:"id"`
	Symbol      string          `json:"symbol"`
	Interval    string          `json:"interval"`
	OpenTime    time.Time       `json:"openTime"`
	CloseTime   time.Time       `json:"closeTime"`
	OpenPrice   decimal.Decimal `json:"openPrice"`
	HighPrice   decimal.Decimal `json:"highPrice"`
	LowPrice    decimal.Decimal `json:"lowPrice"`
	ClosePrice  decimal.Decimal `json:"closePrice"`
	Volume      decimal.Decimal `json:"volume"`
	QuoteVolume decimal.Decimal `json:"quoteVolume"`
	TradeCount  int             `json:"tradeCount"`
	IsClosed    bool            `json:"isClosed"`
}

type Position struct {
	ID                   int64           `json:"id"`
	Symbol               string          `json:"symbol"`
	Side                 string          `json:"side"`
	EntryOrderID         *int64          `json:"entryOrderId,omitempty"`
	ExitOrderID          *int64          `json:"exitOrderId,omitempty"`
	EntryPrice           decimal.Decimal `json:"entryPrice"`
	ExitPrice            decimal.Decimal `json:"exitPrice,omitempty"`
	Quantity             decimal.Decimal `json:"quantity"`
	TakeProfitPrice      decimal.Decimal `json:"takeProfitPrice"`
	StopLossPrice        decimal.Decimal `json:"stopLossPrice"`
	TrailingStopDistance decimal.Decimal `json:"trailingStopDistance,omitempty"`
	TrailingStopPrice    decimal.Decimal `json:"trailingStopPrice,omitempty"`
	RealizedPnl          decimal.Decimal `json:"realizedPnl,omitempty"`
	FeeTotal             decimal.Decimal `json:"feeTotal"`
	Status               string          `json:"status"`
	CloseReason          string          `json:"closeReason,omitempty"`
	OpenedAt             time.Time       `json:"openedAt"`
	ClosedAt             *time.Time      `json:"closedAt,omitempty"`
	UpdatedAt            time.Time       `json:"updatedAt"`
}

type PortfolioMetrics struct {
	ID                     int64           `json:"id"`
	Balance                decimal.Decimal `json:"balance"`
	PeakBalance            decimal.Decimal `json:"peakBalance"`
	DrawdownPercent        decimal.Decimal `json:"drawdownPercent"`
	TotalPnl               decimal.Decimal `json:"totalPnl"`
	WinCount               int             `json:"winCount"`
	LossCount              int             `json:"lossCount"`
	TotalTrades            int             `json:"totalTrades"`
	WinRate                decimal.Decimal `json:"winRate"`
	ProfitFactor           decimal.Decimal `json:"profitFactor,omitempty"`
	AverageWin             decimal.Decimal `json:"averageWin,omitempty"`
	AverageLoss            decimal.Decimal `json:"averageLoss,omitempty"`
	SharpeRatio            decimal.Decimal `json:"sharpeRatio,omitempty"`
	MaxDrawdownPercent     decimal.Decimal `json:"maxDrawdownPercent"`
	IsCircuitBreakerActive bool            `json:"isCircuitBreakerActive"`
	SnapshotAt             time.Time       `json:"snapshotAt"`
}

type TickerPrice struct {
	Symbol    string          `json:"symbol"`
	Price     decimal.Decimal `json:"price"`
	Timestamp time.Time       `json:"timestamp"`
}

// Indicator stores all computed indicator values for a single candle.
type Indicator struct {
	ID              int64           `json:"id"`
	CandleID        int64           `json:"candleId"`
	Symbol          string          `json:"symbol"`
	Interval        string          `json:"interval"`
	Timestamp       time.Time       `json:"timestamp"`
	Rsi             decimal.Decimal `json:"rsi"`
	MacdLine        decimal.Decimal `json:"macdLine"`
	MacdSignal      decimal.Decimal `json:"macdSignal"`
	MacdHistogram   decimal.Decimal `json:"macdHistogram"`
	BollingerUpper  decimal.Decimal `json:"bollingerUpper"`
	BollingerMiddle decimal.Decimal `json:"bollingerMiddle"`
	BollingerLower  decimal.Decimal `json:"bollingerLower"`
	Ema9            decimal.Decimal `json:"ema9"`
	Ema21           decimal.Decimal `json:"ema21"`
	Ema50           decimal.Decimal `json:"ema50"`
	Ema200          decimal.Decimal `json:"ema200"`
	Vwap            decimal.Decimal `json:"vwap"`
	Atr             decimal.Decimal `json:"atr"`
}

type DecisionAction string

const (
	DecisionActionBuy  DecisionAction = "BUY"
	DecisionActionSell DecisionAction = "SELL"
	DecisionActionHold DecisionAction = "HOLD"
)

type DecisionExecutionStatus string

const (
	DecisionExecutionStatusExecuted                 DecisionExecutionStatus = "executed"
	DecisionExecutionStatusBelowConfidenceThreshold DecisionExecutionStatus = "below_confidence_threshold"
	DecisionExecutionStatusMaxPositionsReached      DecisionExecutionStatus = "max_positions_reached"
	DecisionExecutionStatusCircuitBreakerActive     DecisionExecutionStatus = "circuit_breaker_active"
	DecisionExecutionStatusDryRun                   DecisionExecutionStatus = "dry_run"
	DecisionExecutionStatusRejected                 DecisionExecutionStatus = "rejected"
	DecisionExecutionStatusPending                  DecisionExecutionStatus = "pending"
)

type Decision struct {
	ID                      int64                   `json:"id"`
	Symbol                  string                  `json:"symbol"`
	Action                  DecisionAction          `json:"action"`
	Confidence              int                     `json:"confidence"`
	ExecutionStatus         DecisionExecutionStatus `json:"executionStatus"`
	RejectionReason         string                  `json:"rejectionReason,omitempty"`
	RsiSignal               decimal.Decimal         `json:"rsiSignal"`
	MacdSignal              decimal.Decimal         `json:"macdSignal"`
	BollingerSignal         decimal.Decimal         `json:"bollingerSignal"`
	EmaSignal               decimal.Decimal         `json:"emaSignal"`
	PatternSignal           decimal.Decimal         `json:"patternSignal"`
	SentimentSignal         decimal.Decimal         `json:"sentimentSignal"`
	SupportResistanceSignal decimal.Decimal         `json:"supportResistanceSignal"`
	CompositeScore          decimal.Decimal         `json:"compositeScore"`
	IsDryRun                bool                    `json:"isDryRun"`
	CreatedAt               time.Time               `json:"createdAt"`
}

type OrderSide string

const (
	OrderSideBuy  OrderSide = "BUY"
	OrderSideSell OrderSide = "SELL"
)

type OrderStatus string

const (
	OrderStatusPending        OrderStatus = "pending"
	OrderStatusFilled         OrderStatus = "filled"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusCancelled      OrderStatus = "cancelled"
	OrderStatusRejected       OrderStatus = "rejected"
	OrderStatusExpired        OrderStatus = "expired"
)

type OrderExchange string

const (
	OrderExchangeBinance    OrderExchange = "binance"
	OrderExchangePolymarket OrderExchange = "polymarket"
)

type Order struct {
	ID              int64           `json:"id"`
	Exchange        OrderExchange   `json:"exchange"`
	ExternalOrderID string          `json:"externalOrderId,omitempty"`
	DecisionID      *int64          `json:"decisionId,omitempty"`
	Symbol          string          `json:"symbol"`
	Side            OrderSide       `json:"side"`
	Quantity        decimal.Decimal `json:"quantity"`
	Price           decimal.Decimal `json:"price,omitempty"`
	FilledQuantity  decimal.Decimal `json:"filledQuantity"`
	FilledPrice     decimal.Decimal `json:"filledPrice,omitempty"`
	Fee             decimal.Decimal `json:"fee"`
	FeeAsset        string          `json:"feeAsset,omitempty"`
	Status          OrderStatus     `json:"status"`
	RawResponse     []byte          `json:"rawResponse,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

type NewsArticle struct {
	ID         int64     `json:"id"`
	ExternalID string    `json:"externalId,omitempty"`
	Source     string    `json:"source"`
	Title      string    `json:"title"`
	URL        string    `json:"url,omitempty"`
	PublishedAt time.Time `json:"publishedAt"`
	RawContent string    `json:"rawContent,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type SentimentDirection string

const (
	SentimentDirectionPositive SentimentDirection = "positive"
	SentimentDirectionNegative SentimentDirection = "negative"
	SentimentDirectionNeutral  SentimentDirection = "neutral"
)

type SentimentScore struct {
	ID         int64              `json:"id"`
	ArticleID  int64              `json:"articleId"`
	Symbol     string             `json:"symbol"`
	Direction  SentimentDirection `json:"direction"`
	Confidence decimal.Decimal    `json:"confidence"`
	RawScore   decimal.Decimal    `json:"rawScore,omitempty"`
	ModelUsed  string             `json:"modelUsed,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
}

// PolymarketActivity represents a trade or market matching event from Polymarket.
type PolymarketActivity struct {
	EventType  string          `json:"eventType"`  // e.g., "TRADE", "ORDER_MATCH"
	MarketSlug string          `json:"marketSlug"`
	Side       string          `json:"side"`       // BUY / SELL
	Price      decimal.Decimal `json:"price"`
	Size       decimal.Decimal `json:"size"`
	Timestamp  time.Time       `json:"timestamp"`
	User       string          `json:"user,omitempty"`
}

// PolymarketCryptoPrice represents a crypto price update from Polymarket.
type PolymarketCryptoPrice struct {
	Symbol    string          `json:"symbol"`
	Value     decimal.Decimal `json:"value"`
	Timestamp time.Time       `json:"timestamp"`
}
