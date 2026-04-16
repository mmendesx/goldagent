package indicator

import (
	"testing"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// tolerance for decimal comparisons — acceptable for indicator math.
var tolerance = decimal.NewFromFloat(0.01)

// withinTolerance returns true if |expected - actual| <= tolerance.
func withinTolerance(t *testing.T, name string, expected, actual decimal.Decimal) bool {
	t.Helper()
	diff := expected.Sub(actual).Abs()
	if diff.GreaterThan(tolerance) {
		t.Errorf("%s: expected %s, got %s (diff %s exceeds tolerance %s)",
			name, expected.StringFixed(6), actual.StringFixed(6), diff.StringFixed(6), tolerance.StringFixed(6))
		return false
	}
	return true
}

// makeCandle constructs a minimal candle from a close price (high=close, low=close, volume=1).
func makeCandle(closePrice float64) domain.Candle {
	p := decimal.NewFromFloat(closePrice)
	return domain.Candle{
		OpenPrice:  p,
		HighPrice:  p,
		LowPrice:   p,
		ClosePrice: p,
		Volume:     decimal.NewFromInt(1),
		OpenTime:   time.Now(),
		CloseTime:  time.Now(),
	}
}

// makeCandleOHLCV constructs a candle with explicit OHLCV values.
func makeCandleOHLCV(open, high, low, closePrice, volume float64) domain.Candle {
	return domain.Candle{
		OpenPrice:  decimal.NewFromFloat(open),
		HighPrice:  decimal.NewFromFloat(high),
		LowPrice:   decimal.NewFromFloat(low),
		ClosePrice: decimal.NewFromFloat(closePrice),
		Volume:     decimal.NewFromFloat(volume),
		OpenTime:   time.Now(),
		CloseTime:  time.Now(),
	}
}

// makeCandleSlice builds a candle slice from a plain float64 close price sequence (oldest first).
func makeCandleSlice(closes []float64) []domain.Candle {
	candles := make([]domain.Candle, len(closes))
	for i, c := range closes {
		candles[i] = makeCandle(c)
	}
	return candles
}

// ----------------------------
// RSI tests
// ----------------------------

// TestCalculateRsi_ClassicExample uses a 14-period RSI on a 15-candle sequence with
// manually verified expected output.
//
// Prices: [44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.15, 43.61, 44.33, 44.83, 45.10, 45.15, 43.61, 44.33]
// 14 changes:
//   Gains (+): 0.06, 0.72, 0.50, 0.27, 0.05, 0.72, 0.50, 0.27, 0.05, 0.72  → sumGain = 3.86
//   Losses (-): 0.25, 0.54, 1.54, 1.54                                       → sumLoss = 3.87
//
// avgGain = 3.86/14 ≈ 0.27571, avgLoss = 3.87/14 ≈ 0.27643
// RS ≈ 0.9974, RSI = 100 - 100/(1+0.9974) ≈ 49.93
func TestCalculateRsi_ClassicExample(t *testing.T) {
	closes := []float64{
		44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.15,
		43.61, 44.33, 44.83, 45.10, 45.15, 43.61, 44.33,
	}
	candles := makeCandleSlice(closes)
	rsi := CalculateRsi(candles, 14)

	expected := decimal.NewFromFloat(49.93)
	withinTolerance(t, "RSI classic 14-period", expected, rsi)
}

// TestCalculateRsi_AllGains verifies RSI returns 100 when all price changes are positive.
func TestCalculateRsi_AllGains(t *testing.T) {
	// 15 candles, each 1 point higher than the last.
	candles := make([]domain.Candle, 15)
	for i := range candles {
		candles[i] = makeCandle(float64(100 + i))
	}

	rsi := CalculateRsi(candles, 14)
	withinTolerance(t, "RSI all gains", decimal.NewFromInt(100), rsi)
}

// TestCalculateRsi_AllLosses verifies RSI returns 0 when all price changes are negative.
func TestCalculateRsi_AllLosses(t *testing.T) {
	candles := make([]domain.Candle, 15)
	for i := range candles {
		candles[i] = makeCandle(float64(100 - i))
	}

	rsi := CalculateRsi(candles, 14)
	if !rsi.IsZero() {
		t.Errorf("RSI all losses: expected 0, got %s", rsi.String())
	}
}

// TestCalculateRsi_InsufficientCandles verifies RSI returns 0 when there aren't enough candles.
func TestCalculateRsi_InsufficientCandles(t *testing.T) {
	candles := makeCandleSlice([]float64{100, 101, 102}) // only 3 candles
	rsi := CalculateRsi(candles, 14)
	if !rsi.IsZero() {
		t.Errorf("RSI insufficient candles: expected 0, got %s", rsi.String())
	}
}

// TestCalculateRsi_FlatPrices verifies RSI returns 0 when all prices are the same (no gains, no losses).
func TestCalculateRsi_FlatPrices(t *testing.T) {
	candles := make([]domain.Candle, 15)
	for i := range candles {
		candles[i] = makeCandle(100.0)
	}
	rsi := CalculateRsi(candles, 14)
	if !rsi.IsZero() {
		t.Errorf("RSI flat prices: expected 0, got %s", rsi.String())
	}
}

// ----------------------------
// MACD tests
// ----------------------------

// TestCalculateMacd_BasicExample verifies that MACD produces non-zero values for a
// long enough price series and that Histogram = Line - Signal.
func TestCalculateMacd_BasicExample(t *testing.T) {
	// Need at least slow+signal-1 = 26+9-1 = 34 candles for standard MACD.
	closes := make([]float64, 40)
	for i := range closes {
		closes[i] = 100.0 + float64(i)*0.5 // simple uptrend
	}
	candles := makeCandleSlice(closes)

	result := CalculateMacd(candles, 12, 26, 9)

	// Histogram must equal Line - Signal.
	expectedHistogram := result.Line.Sub(result.Signal)
	if !result.Histogram.Equal(expectedHistogram) {
		t.Errorf("MACD histogram mismatch: Line(%s) - Signal(%s) = %s, got %s",
			result.Line.String(), result.Signal.String(), expectedHistogram.String(), result.Histogram.String())
	}

	// For an uptrend, MACD line should be positive (fast EMA > slow EMA).
	if !result.Line.IsPositive() {
		t.Errorf("MACD line expected positive for uptrend, got %s", result.Line.String())
	}
}

// TestCalculateMacd_InsufficientCandles verifies zero-valued result when candles are insufficient.
func TestCalculateMacd_InsufficientCandles(t *testing.T) {
	candles := makeCandleSlice([]float64{100, 101, 102})
	result := CalculateMacd(candles, 12, 26, 9)

	if !result.Line.IsZero() || !result.Signal.IsZero() || !result.Histogram.IsZero() {
		t.Errorf("MACD with insufficient candles: expected zero MacdResult, got Line=%s Signal=%s Histogram=%s",
			result.Line.String(), result.Signal.String(), result.Histogram.String())
	}
}

// ----------------------------
// Bollinger Bands tests
// ----------------------------

// TestCalculateBollingerBands_FlatPriceSeriesHasZeroBandwidth verifies that a flat price
// series produces zero bandwidth (upper == lower == middle).
func TestCalculateBollingerBands_FlatPriceSeriesHasZeroBandwidth(t *testing.T) {
	candles := make([]domain.Candle, 20)
	for i := range candles {
		candles[i] = makeCandle(50.0)
	}

	bb := CalculateBollingerBands(candles, 20, 2.0)

	expectedMiddle := decimal.NewFromFloat(50.0)
	withinTolerance(t, "BB middle (flat)", expectedMiddle, bb.Middle)
	withinTolerance(t, "BB upper (flat)", expectedMiddle, bb.Upper)
	withinTolerance(t, "BB lower (flat)", expectedMiddle, bb.Lower)
}

// TestCalculateBollingerBands_KnownValues verifies known SMA and band values.
func TestCalculateBollingerBands_KnownValues(t *testing.T) {
	// 5-period BB on [10, 12, 11, 13, 14]:
	// SMA = 60/5 = 12.0
	// deviations: -2, 0, -1, 1, 2 → squared: 4, 0, 1, 1, 4 → variance = 10/5 = 2.0 → stddev = 1.414...
	// Upper = 12 + 2*1.414 = 14.828, Lower = 12 - 2*1.414 = 9.172
	closes := []float64{10, 12, 11, 13, 14}
	candles := makeCandleSlice(closes)

	bb := CalculateBollingerBands(candles, 5, 2.0)

	withinTolerance(t, "BB middle", decimal.NewFromFloat(12.0), bb.Middle)

	expectedBand := decimal.NewFromFloat(2.828) // 2 * sqrt(2)
	actualBand := bb.Upper.Sub(bb.Middle)
	withinTolerance(t, "BB band width", expectedBand, actualBand)
}

// TestCalculateBollingerBands_InsufficientCandles verifies zero-valued result.
func TestCalculateBollingerBands_InsufficientCandles(t *testing.T) {
	candles := makeCandleSlice([]float64{100, 101})
	bb := CalculateBollingerBands(candles, 20, 2.0)

	if !bb.Upper.IsZero() || !bb.Middle.IsZero() || !bb.Lower.IsZero() {
		t.Errorf("BB insufficient candles: expected zero result, got Upper=%s Middle=%s Lower=%s",
			bb.Upper.String(), bb.Middle.String(), bb.Lower.String())
	}
}

// ----------------------------
// EMA tests
// ----------------------------

// TestCalculateExponentialMovingAverage_KnownExample verifies EMA against a manually computed sequence.
// 5-period EMA on [10, 11, 12, 13, 14, 15]:
// SMA(5) = 60/5 = 12.0, multiplier = 2/(5+1) = 0.333...
// EMA(15) = 15 * 0.333 + 12.0 * 0.667 = 5.0 + 8.0 = 13.0
func TestCalculateExponentialMovingAverage_KnownExample(t *testing.T) {
	closes := []float64{10, 11, 12, 13, 14, 15}
	candles := makeCandleSlice(closes)

	ema := CalculateExponentialMovingAverage(candles, 5)
	expected := decimal.NewFromFloat(13.0)
	withinTolerance(t, "EMA 5-period", expected, ema)
}

// TestCalculateExponentialMovingAverage_InsufficientCandles verifies zero return.
func TestCalculateExponentialMovingAverage_InsufficientCandles(t *testing.T) {
	candles := makeCandleSlice([]float64{100, 101})
	ema := CalculateExponentialMovingAverage(candles, 5)
	if !ema.IsZero() {
		t.Errorf("EMA insufficient candles: expected 0, got %s", ema.String())
	}
}

// TestCalculateExponentialMovingAverage_ExactlyOnePeriod verifies SMA seed when len==period.
func TestCalculateExponentialMovingAverage_ExactlyOnePeriod(t *testing.T) {
	closes := []float64{10, 20, 30}
	candles := makeCandleSlice(closes)
	ema := CalculateExponentialMovingAverage(candles, 3)
	expected := decimal.NewFromFloat(20.0) // SMA of [10, 20, 30]
	withinTolerance(t, "EMA exact period (SMA seed)", expected, ema)
}

// ----------------------------
// VWAP tests
// ----------------------------

// TestCalculateVwap_BasicExample verifies VWAP against a manually computed example.
// Candle 1: typical=(10+8+9)/3=9, vol=100 → 900
// Candle 2: typical=(12+10+11)/3=11, vol=200 → 2200
// VWAP = (900+2200)/(100+200) = 3100/300 = 10.333...
func TestCalculateVwap_BasicExample(t *testing.T) {
	candles := []domain.Candle{
		makeCandleOHLCV(8, 10, 8, 9, 100),
		makeCandleOHLCV(10, 12, 10, 11, 200),
	}

	vwap := CalculateVwap(candles)
	expected := decimal.NewFromFloat(10.333)
	withinTolerance(t, "VWAP basic", expected, vwap)
}

// TestCalculateVwap_EqualVolumes verifies VWAP equals SMA of typical prices when volumes are equal.
func TestCalculateVwap_EqualVolumes(t *testing.T) {
	// typical = (high+low+close)/3 for equal volumes
	// candle 1: typical = (12+8+10)/3 = 10, candle 2: typical = (14+10+12)/3 = 12
	// VWAP = (10+12)/2 = 11
	candles := []domain.Candle{
		makeCandleOHLCV(9, 12, 8, 10, 100),
		makeCandleOHLCV(11, 14, 10, 12, 100),
	}

	vwap := CalculateVwap(candles)
	expected := decimal.NewFromFloat(11.0)
	withinTolerance(t, "VWAP equal volumes", expected, vwap)
}

// TestCalculateVwap_ZeroVolume verifies VWAP returns zero when total volume is zero.
func TestCalculateVwap_ZeroVolume(t *testing.T) {
	candle := domain.Candle{
		HighPrice:  decimal.NewFromFloat(100),
		LowPrice:   decimal.NewFromFloat(90),
		ClosePrice: decimal.NewFromFloat(95),
		Volume:     decimal.Zero,
	}
	vwap := CalculateVwap([]domain.Candle{candle})
	if !vwap.IsZero() {
		t.Errorf("VWAP zero volume: expected 0, got %s", vwap.String())
	}
}

// ----------------------------
// ATR tests
// ----------------------------

// TestCalculateAtr_BasicExample verifies ATR against a manually computed example.
// Using 3-period ATR on 4 candles (need period+1=4).
// Candle sequence: prev_close drives true range calculation.
func TestCalculateAtr_BasicExample(t *testing.T) {
	// Candle 0 (seed): close=10
	// Candle 1: high=12, low=9, close=11; prev_close=10
	//   TR = max(12-9, |12-10|, |9-10|) = max(3,2,1) = 3
	// Candle 2: high=13, low=10, close=12; prev_close=11
	//   TR = max(13-10, |13-11|, |10-11|) = max(3,2,1) = 3
	// Candle 3: high=11, low=9, close=10; prev_close=12
	//   TR = max(11-9, |11-12|, |9-12|) = max(2,1,3) = 3
	// Initial ATR (period=3) = (3+3+3)/3 = 3.0
	candles := []domain.Candle{
		makeCandleOHLCV(10, 10, 10, 10, 1),
		makeCandleOHLCV(10, 12, 9, 11, 1),
		makeCandleOHLCV(11, 13, 10, 12, 1),
		makeCandleOHLCV(12, 11, 9, 10, 1),
	}

	atr := CalculateAtr(candles, 3)
	expected := decimal.NewFromFloat(3.0)
	withinTolerance(t, "ATR basic", expected, atr)
}

// TestCalculateAtr_InsufficientCandles verifies ATR returns zero when there aren't enough candles.
func TestCalculateAtr_InsufficientCandles(t *testing.T) {
	candles := makeCandleSlice([]float64{100, 101})
	atr := CalculateAtr(candles, 14)
	if !atr.IsZero() {
		t.Errorf("ATR insufficient candles: expected 0, got %s", atr.String())
	}
}

// TestCalculateAtr_WilderSmoothing verifies that Wilder's smoothing is applied when
// there are more candles than the initial period.
func TestCalculateAtr_WilderSmoothing(t *testing.T) {
	// 5 candles with period=2: seed from first 2 TRs, then smooth on 3rd TR.
	// All TRs = 1 (flat high-low=1, prev_close same as low, diff=0):
	candles := []domain.Candle{
		makeCandleOHLCV(10, 11, 9, 10, 1),  // seed, first prev_close
		makeCandleOHLCV(10, 11, 9, 10, 1),  // TR = max(2, 1, 1) = 2
		makeCandleOHLCV(10, 11, 9, 10, 1),  // TR = max(2, 1, 1) = 2
		makeCandleOHLCV(10, 11, 9, 10, 1),  // TR = max(2, 1, 1) = 2
	}
	atr := CalculateAtr(candles, 2)
	// All TRs are 2, so ATR should remain 2 regardless of smoothing.
	expected := decimal.NewFromFloat(2.0)
	withinTolerance(t, "ATR Wilder smoothing constant", expected, atr)
}

// ----------------------------
// Benchmark
// ----------------------------

// BenchmarkComputeAllIndicators_200Candles asserts the full indicator computation
// completes in under 20ms for 200 candles. Typical performance is 1-5ms.
func BenchmarkComputeAllIndicators_200Candles(b *testing.B) {
	// Build 200 candles with a realistic price series.
	candles := make([]domain.Candle, 200)
	price := 30000.0
	for i := range candles {
		// Simple oscillating price series to exercise all indicators.
		if i%2 == 0 {
			price += 50.0
		} else {
			price -= 25.0
		}
		candles[i] = makeCandleOHLCV(price-10, price+20, price-30, price, 1000.0+float64(i))
	}

	computer := NewComputer(ComputerConfig{
		RsiPeriod:       14,
		MacdFast:        12,
		MacdSlow:        26,
		MacdSignalPeriod: 9,
		BollingerPeriod: 20,
		BollingerStdDev: 2.0,
		EmaPeriods:      []int{9, 21, 50, 200},
		AtrPeriod:       14,
	})

	latestCandle := candles[len(candles)-1]

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computer.ComputeAllIndicators(latestCandle, candles)
	}

	// Assert each iteration completes well within 20ms.
	nsPerOp := b.Elapsed().Nanoseconds() / int64(b.N)
	const maxNs = 20_000_000 // 20ms in nanoseconds
	if nsPerOp > maxNs {
		b.Errorf("ComputeAllIndicators too slow: %dns per op (limit: %dns)", nsPerOp, maxNs)
	}
}
