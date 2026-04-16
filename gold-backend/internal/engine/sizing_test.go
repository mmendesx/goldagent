package engine

import (
	"testing"

	"github.com/shopspring/decimal"
)

func assertDecimalEqual(t *testing.T, name string, expected, actual decimal.Decimal) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("%s: expected %s, got %s", name, expected.String(), actual.String())
	}
}

func TestComputeSizingForLong_BasicCase(t *testing.T) {
	// balance=10000, size=10%, price=50000, ATR=1000, TP=2×ATR, SL=1×ATR, trailing=1×ATR
	inputs := SizingInputs{
		AccountBalance:            decimal.NewFromInt(10000),
		PositionSizePercent:       decimal.NewFromInt(10),
		CurrentPrice:              decimal.NewFromInt(50000),
		AtrValue:                  decimal.NewFromInt(1000),
		TakeProfitAtrMultiplier:   decimal.NewFromInt(2),
		StopLossAtrMultiplier:     decimal.NewFromInt(1),
		TrailingStopAtrMultiplier: decimal.NewFromInt(1),
	}
	result := ComputeSizingForLong(inputs)

	// Quantity = (10000 * 10 / 100) / 50000 = 1000 / 50000 = 0.02
	assertDecimalEqual(t, "quantity", decimal.NewFromFloat(0.02), result.Quantity)

	// TakeProfit = 50000 + (1000 * 2) = 52000
	assertDecimalEqual(t, "take profit", decimal.NewFromInt(52000), result.TakeProfitPrice)

	// StopLoss = 50000 - (1000 * 1) = 49000
	assertDecimalEqual(t, "stop loss", decimal.NewFromInt(49000), result.StopLossPrice)

	// TrailingStop = 1000 * 1 = 1000
	assertDecimalEqual(t, "trailing stop distance", decimal.NewFromInt(1000), result.TrailingStopDistance)
}

func TestComputeSizingForLong_NoTrailingStop(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance:            decimal.NewFromInt(10000),
		PositionSizePercent:       decimal.NewFromInt(10),
		CurrentPrice:              decimal.NewFromInt(50000),
		AtrValue:                  decimal.NewFromInt(1000),
		TakeProfitAtrMultiplier:   decimal.NewFromInt(2),
		StopLossAtrMultiplier:     decimal.NewFromInt(1),
		TrailingStopAtrMultiplier: decimal.Zero, // disabled
	}
	result := ComputeSizingForLong(inputs)

	if !result.TrailingStopDistance.IsZero() {
		t.Errorf("trailing stop should be zero when multiplier is zero, got %s", result.TrailingStopDistance.String())
	}
}

func TestComputeSizingForLong_ZeroPrice_ReturnsEmpty(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance:  decimal.NewFromInt(10000),
		CurrentPrice:    decimal.Zero,
		AtrValue:        decimal.NewFromInt(100),
	}
	result := ComputeSizingForLong(inputs)
	if !result.Quantity.IsZero() {
		t.Errorf("zero price should produce zero quantity, got %s", result.Quantity.String())
	}
}

func TestComputeSizingForLong_LargerBalance(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance:          decimal.NewFromInt(100000),
		PositionSizePercent:     decimal.NewFromFloat(5),
		CurrentPrice:            decimal.NewFromInt(2000),
		AtrValue:                decimal.NewFromInt(50),
		TakeProfitAtrMultiplier: decimal.NewFromInt(2),
		StopLossAtrMultiplier:   decimal.NewFromInt(1),
	}
	result := ComputeSizingForLong(inputs)

	// Quantity = (100000 * 5 / 100) / 2000 = 5000 / 2000 = 2.5
	assertDecimalEqual(t, "quantity", decimal.NewFromFloat(2.5), result.Quantity)

	// TakeProfit = 2000 + 100 = 2100
	assertDecimalEqual(t, "take profit", decimal.NewFromInt(2100), result.TakeProfitPrice)

	// StopLoss = 2000 - 50 = 1950
	assertDecimalEqual(t, "stop loss", decimal.NewFromInt(1950), result.StopLossPrice)
}

func TestComputeSizingForShort_BasicCase(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance:            decimal.NewFromInt(10000),
		PositionSizePercent:       decimal.NewFromInt(10),
		CurrentPrice:              decimal.NewFromInt(50000),
		AtrValue:                  decimal.NewFromInt(1000),
		TakeProfitAtrMultiplier:   decimal.NewFromInt(2),
		StopLossAtrMultiplier:     decimal.NewFromInt(1),
		TrailingStopAtrMultiplier: decimal.NewFromInt(1),
	}
	result := ComputeSizingForShort(inputs)

	// Quantity same calculation
	assertDecimalEqual(t, "quantity", decimal.NewFromFloat(0.02), result.Quantity)

	// TakeProfit = 50000 - (1000 * 2) = 48000 (below entry for short)
	assertDecimalEqual(t, "take profit", decimal.NewFromInt(48000), result.TakeProfitPrice)

	// StopLoss = 50000 + (1000 * 1) = 51000 (above entry for short)
	assertDecimalEqual(t, "stop loss", decimal.NewFromInt(51000), result.StopLossPrice)

	// TrailingStop = 1000
	assertDecimalEqual(t, "trailing stop distance", decimal.NewFromInt(1000), result.TrailingStopDistance)
}

func TestComputeSizingForShort_ZeroPrice_ReturnsEmpty(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance: decimal.NewFromInt(10000),
		CurrentPrice:   decimal.Zero,
		AtrValue:       decimal.NewFromInt(100),
	}
	result := ComputeSizingForShort(inputs)
	if !result.Quantity.IsZero() {
		t.Errorf("zero price should produce zero quantity, got %s", result.Quantity.String())
	}
}

func TestComputeSizingForShort_TakeProfitBelowEntryPrice(t *testing.T) {
	inputs := SizingInputs{
		AccountBalance:          decimal.NewFromInt(5000),
		PositionSizePercent:     decimal.NewFromInt(20),
		CurrentPrice:            decimal.NewFromInt(1000),
		AtrValue:                decimal.NewFromInt(30),
		TakeProfitAtrMultiplier: decimal.NewFromInt(2),
		StopLossAtrMultiplier:   decimal.NewFromInt(1),
	}
	result := ComputeSizingForShort(inputs)

	if !result.TakeProfitPrice.LessThan(inputs.CurrentPrice) {
		t.Errorf("short TP should be below entry price; TP=%s entry=%s",
			result.TakeProfitPrice.String(), inputs.CurrentPrice.String())
	}
	if !result.StopLossPrice.GreaterThan(inputs.CurrentPrice) {
		t.Errorf("short SL should be above entry price; SL=%s entry=%s",
			result.StopLossPrice.String(), inputs.CurrentPrice.String())
	}
}
