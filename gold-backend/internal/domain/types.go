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
	EntryPrice           decimal.Decimal `json:"entryPrice"`
	ExitPrice            decimal.Decimal `json:"exitPrice,omitempty"`
	Quantity             decimal.Decimal `json:"quantity"`
	TakeProfitPrice      decimal.Decimal `json:"takeProfitPrice"`
	StopLossPrice        decimal.Decimal `json:"stopLossPrice"`
	TrailingStopDistance decimal.Decimal `json:"trailingStopDistance,omitempty"`
	TrailingStopPrice    decimal.Decimal `json:"trailingStopPrice,omitempty"`
	RealizedPnl          decimal.Decimal `json:"realizedPnl,omitempty"`
	Status               string          `json:"status"`
	CloseReason          string          `json:"closeReason,omitempty"`
	OpenedAt             time.Time       `json:"openedAt"`
	ClosedAt             *time.Time      `json:"closedAt,omitempty"`
}

type PortfolioMetrics struct {
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
}

type TickerPrice struct {
	Symbol    string          `json:"symbol"`
	Price     decimal.Decimal `json:"price"`
	Timestamp time.Time       `json:"timestamp"`
}
