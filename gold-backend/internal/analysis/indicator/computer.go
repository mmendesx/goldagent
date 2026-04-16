package indicator

import (
	"context"
	"log/slog"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
	"github.com/shopspring/decimal"
)

// ComputerConfig wires the indicator computer with all required dependencies and tuning parameters.
type ComputerConfig struct {
	InputChannel        <-chan domain.Candle
	CandleRepository    postgres.CandleRepository
	IndicatorRepository postgres.IndicatorRepository
	RsiPeriod           int
	MacdFast            int
	MacdSlow            int
	MacdSignalPeriod    int
	BollingerPeriod     int
	BollingerStdDev     float64
	EmaPeriods          []int  // e.g. [9, 21, 50, 200]
	AtrPeriod           int
	HistoryLimit        int // how many recent candles to fetch for context (e.g., 300)
	Logger              *slog.Logger
}

// Computer subscribes to closed-candle events, computes all technical indicators
// for the candle's symbol+interval, and persists results via the repository.
type Computer struct {
	config ComputerConfig
}

// NewComputer constructs a Computer with the provided configuration.
func NewComputer(config ComputerConfig) *Computer {
	return &Computer{config: config}
}

// Run drains the input channel, computes indicators on each closed-candle event, and
// persists results. Blocks until ctx is cancelled or the input channel closes.
// Errors on individual candles are logged and do not terminate the loop.
func (computer *Computer) Run(ctx context.Context) error {
	for {
		select {
		case candle, ok := <-computer.config.InputChannel:
			if !ok {
				computer.config.Logger.Info("indicator computer: input channel closed, shutting down")
				return nil
			}
			computer.processCandle(ctx, candle)

		case <-ctx.Done():
			computer.config.Logger.Info("indicator computer: context cancelled, shutting down",
				"reason", ctx.Err(),
			)
			return ctx.Err()
		}
	}
}

// processCandle fetches recent history, computes all indicators, and persists the result.
// Errors are logged; the loop continues regardless.
func (computer *Computer) processCandle(ctx context.Context, closedCandle domain.Candle) {
	// Fetch the most recent candles from the repository.
	// The aggregator persists the closed candle before emitting it, so the history
	// will include it — giving us the candle's real DB ID.
	// FindLatestCandles returns rows in DESC order; we reverse for chronological processing.
	descCandles, err := computer.config.CandleRepository.FindLatestCandles(
		ctx,
		closedCandle.Symbol,
		closedCandle.Interval,
		computer.config.HistoryLimit,
	)
	if err != nil {
		computer.config.Logger.Error("indicator computer: failed to fetch candle history",
			"symbol", closedCandle.Symbol,
			"interval", closedCandle.Interval,
			"timestamp", closedCandle.CloseTime,
			"error", err,
		)
		return
	}

	if len(descCandles) == 0 {
		computer.config.Logger.Warn("indicator computer: no candle history found, skipping",
			"symbol", closedCandle.Symbol,
			"interval", closedCandle.Interval,
		)
		return
	}

	// Reverse to chronological order (oldest first, most recent last).
	history := reverseCandles(descCandles)

	// The most recent candle in the reversed history is the just-closed candle
	// with its real DB ID (as persisted by the aggregator).
	latestCandle := history[len(history)-1]

	indicator := computer.ComputeAllIndicators(latestCandle, history)

	id, err := computer.config.IndicatorRepository.InsertIndicator(ctx, indicator)
	if err != nil {
		computer.config.Logger.Error("indicator computer: failed to persist indicator",
			"symbol", closedCandle.Symbol,
			"interval", closedCandle.Interval,
			"candleId", indicator.CandleID,
			"timestamp", indicator.Timestamp,
			"error", err,
		)
		return
	}

	computer.config.Logger.Info("indicator computer: indicators persisted",
		"symbol", closedCandle.Symbol,
		"interval", closedCandle.Interval,
		"candleId", indicator.CandleID,
		"indicatorId", id,
		"timestamp", indicator.Timestamp,
	)
}

// ComputeAllIndicators is the pure-function core: given a slice of candles
// (oldest first, most recent last) and the latest candle (last in history),
// returns a fully populated domain.Indicator with all computed values.
// This function is safe to call directly in tests without any I/O.
func (computer *Computer) ComputeAllIndicators(latestCandle domain.Candle, history []domain.Candle) domain.Indicator {
	macd := CalculateMacd(history, computer.config.MacdFast, computer.config.MacdSlow, computer.config.MacdSignalPeriod)
	bb := CalculateBollingerBands(history, computer.config.BollingerPeriod, computer.config.BollingerStdDev)

	indicator := domain.Indicator{
		CandleID:        latestCandle.ID,
		Symbol:          latestCandle.Symbol,
		Interval:        latestCandle.Interval,
		Timestamp:       latestCandle.CloseTime,
		Rsi:             CalculateRsi(history, computer.config.RsiPeriod),
		MacdLine:        macd.Line,
		MacdSignal:      macd.Signal,
		MacdHistogram:   macd.Histogram,
		BollingerUpper:  bb.Upper,
		BollingerMiddle: bb.Middle,
		BollingerLower:  bb.Lower,
		Vwap:            CalculateVwap(history),
		Atr:             CalculateAtr(history, computer.config.AtrPeriod),
	}

	// Map EMA periods to their named fields.
	// EmaPeriods is user-configured; we map the standard four.
	indicator.Ema9, indicator.Ema21, indicator.Ema50, indicator.Ema200 = computeNamedEmas(history, computer.config.EmaPeriods)

	return indicator
}

// computeNamedEmas computes EMA values for each standard period and returns them
// in the order: ema9, ema21, ema50, ema200. Periods absent from configuredPeriods
// yield decimal.Zero.
func computeNamedEmas(candles []domain.Candle, configuredPeriods []int) (ema9, ema21, ema50, ema200 decimal.Decimal) {
	periodSet := make(map[int]decimal.Decimal, len(configuredPeriods))
	for _, p := range configuredPeriods {
		periodSet[p] = CalculateExponentialMovingAverage(candles, p)
	}

	lookup := func(period int) decimal.Decimal {
		if v, ok := periodSet[period]; ok {
			return v
		}
		return decimal.Zero
	}

	return lookup(9), lookup(21), lookup(50), lookup(200)
}

// reverseCandles returns a new slice with the candles in reversed order.
func reverseCandles(candles []domain.Candle) []domain.Candle {
	n := len(candles)
	reversed := make([]domain.Candle, n)
	for i, c := range candles {
		reversed[n-1-i] = c
	}
	return reversed
}

