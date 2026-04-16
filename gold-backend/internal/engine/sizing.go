package engine

import (
	"github.com/shopspring/decimal"
)

// SizingInputs holds the values needed to compute position size and TP/SL levels.
type SizingInputs struct {
	AccountBalance            decimal.Decimal // current portfolio balance
	PositionSizePercent       decimal.Decimal // e.g. 10.0 means 10%
	CurrentPrice              decimal.Decimal // entry price estimate
	AtrValue                  decimal.Decimal // for stop and target placement
	TakeProfitAtrMultiplier   decimal.Decimal // suggested 2.0 (TP at 2× ATR from entry)
	StopLossAtrMultiplier     decimal.Decimal // suggested 1.0 (SL at 1× ATR from entry)
	TrailingStopAtrMultiplier decimal.Decimal // from config; 0 = no trailing stop
}

// SizingResult holds the computed order values.
type SizingResult struct {
	Quantity             decimal.Decimal
	TakeProfitPrice      decimal.Decimal
	StopLossPrice        decimal.Decimal
	TrailingStopDistance decimal.Decimal
}

// ComputeSizingForLong computes quantity and TP/SL for a long (BUY) position.
//
//   - Quantity = (balance × positionSizePercent / 100) / currentPrice
//   - TakeProfit = currentPrice + (ATR × tpMultiplier)
//   - StopLoss = currentPrice - (ATR × slMultiplier)
//   - TrailingStopDistance = ATR × trailingMultiplier (zero when multiplier is zero)
func ComputeSizingForLong(inputs SizingInputs) SizingResult {
	if inputs.CurrentPrice.IsZero() {
		return SizingResult{}
	}

	notionalValue := inputs.AccountBalance.Mul(inputs.PositionSizePercent).Div(decimal.NewFromInt(100))
	quantity := notionalValue.Div(inputs.CurrentPrice)

	atrTP := inputs.AtrValue.Mul(inputs.TakeProfitAtrMultiplier)
	atrSL := inputs.AtrValue.Mul(inputs.StopLossAtrMultiplier)

	takeProfitPrice := inputs.CurrentPrice.Add(atrTP)
	stopLossPrice := inputs.CurrentPrice.Sub(atrSL)

	trailingStopDistance := decimal.Zero
	if !inputs.TrailingStopAtrMultiplier.IsZero() {
		trailingStopDistance = inputs.AtrValue.Mul(inputs.TrailingStopAtrMultiplier)
	}

	return SizingResult{
		Quantity:             quantity,
		TakeProfitPrice:      takeProfitPrice,
		StopLossPrice:        stopLossPrice,
		TrailingStopDistance: trailingStopDistance,
	}
}

// ComputeSizingForShort computes quantity and TP/SL for a short (SELL) position.
// For shorts, TP is below the entry price and SL is above it.
//
//   - Quantity = (balance × positionSizePercent / 100) / currentPrice
//   - TakeProfit = currentPrice - (ATR × tpMultiplier)
//   - StopLoss = currentPrice + (ATR × slMultiplier)
//   - TrailingStopDistance = ATR × trailingMultiplier (zero when multiplier is zero)
func ComputeSizingForShort(inputs SizingInputs) SizingResult {
	if inputs.CurrentPrice.IsZero() {
		return SizingResult{}
	}

	notionalValue := inputs.AccountBalance.Mul(inputs.PositionSizePercent).Div(decimal.NewFromInt(100))
	quantity := notionalValue.Div(inputs.CurrentPrice)

	atrTP := inputs.AtrValue.Mul(inputs.TakeProfitAtrMultiplier)
	atrSL := inputs.AtrValue.Mul(inputs.StopLossAtrMultiplier)

	takeProfitPrice := inputs.CurrentPrice.Sub(atrTP)
	stopLossPrice := inputs.CurrentPrice.Add(atrSL)

	trailingStopDistance := decimal.Zero
	if !inputs.TrailingStopAtrMultiplier.IsZero() {
		trailingStopDistance = inputs.AtrValue.Mul(inputs.TrailingStopAtrMultiplier)
	}

	return SizingResult{
		Quantity:             quantity,
		TakeProfitPrice:      takeProfitPrice,
		StopLossPrice:        stopLossPrice,
		TrailingStopDistance: trailingStopDistance,
	}
}
