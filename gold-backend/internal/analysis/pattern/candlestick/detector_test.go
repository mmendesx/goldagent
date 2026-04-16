package candlestick_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// makeCandle constructs a domain.Candle with the given OHLC as float64 values.
// Volume is set to a nominal non-zero value; other fields left zero.
func makeCandle(open, high, low, close float64) domain.Candle {
	return domain.Candle{
		OpenPrice:  decimal.NewFromFloat(open),
		HighPrice:  decimal.NewFromFloat(high),
		LowPrice:   decimal.NewFromFloat(low),
		ClosePrice: decimal.NewFromFloat(close),
		Volume:     decimal.NewFromFloat(1000),
	}
}

// findPattern returns the first DetectedPattern with the given name, or nil.
func findPattern(patterns []candlestick.DetectedPattern, name candlestick.PatternName) *candlestick.DetectedPattern {
	for i := range patterns {
		if patterns[i].Name == name {
			return &patterns[i]
		}
	}
	return nil
}

// -------- Doji --------

func TestDetectDoji_SmallBodyDetected(t *testing.T) {
	// Body = |100.01 - 100.00| = 0.01; range = 110 - 90 = 20 → ratio ≈ 0.0005 (well under 0.1)
	candles := []domain.Candle{
		makeCandle(100.00, 110.00, 90.00, 100.01),
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameDoji)
	if p == nil {
		t.Fatal("expected doji to be detected, got none")
	}
	if p.Direction != candlestick.PatternDirectionNeutral {
		t.Errorf("expected neutral direction, got %s", p.Direction)
	}
	if p.Confidence < 70 || p.Confidence > 100 {
		t.Errorf("confidence %d out of expected range [70,100]", p.Confidence)
	}
	if p.AtCandle != 0 {
		t.Errorf("expected AtCandle=0, got %d", p.AtCandle)
	}
}

func TestDetectDoji_LargeBodyNotDetected(t *testing.T) {
	// Body = |110 - 90| = 20; range = 110 - 90 = 20 → ratio = 1.0 (not a doji)
	candles := []domain.Candle{
		makeCandle(90.00, 110.00, 90.00, 110.00),
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	if p := findPattern(patterns, candlestick.PatternNameDoji); p != nil {
		t.Fatal("expected no doji detection for large-body candle")
	}
}

// -------- Hammer --------

func TestDetectHammer_LongLowerShadow(t *testing.T) {
	// open=110, close=112, high=113, low=100
	// body=2, lower=10, upper=1  → lower(10) >= 2*body(4) ✓, upper(1) <= 0.3*body(0.6)? 1 > 0.6 ✗
	// Adjust: open=110, close=111, high=111.2, low=100
	// body=1, lower=10, upper=0.2  → lower(10) >= 2 ✓, upper(0.2) <= 0.3 ✓
	candles := []domain.Candle{
		makeCandle(110.00, 111.20, 100.00, 111.00),
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameHammer)
	if p == nil {
		t.Fatal("expected hammer to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBullish {
		t.Errorf("expected bullish direction, got %s", p.Direction)
	}
}

// -------- Shooting Star --------

func TestDetectShootingStar_LongUpperShadow(t *testing.T) {
	// open=100, close=101, high=111, low=99.7
	// body=1, upper=10, lower=0.3  → upper(10) >= 2 ✓, lower(0.3) <= 0.3*1=0.3 ✓
	candles := []domain.Candle{
		makeCandle(100.00, 111.00, 99.70, 101.00),
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameShootingStar)
	if p == nil {
		t.Fatal("expected shooting star to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBearish {
		t.Errorf("expected bearish direction, got %s", p.Direction)
	}
}

// -------- Bullish Engulfing --------

func TestDetectBullishEngulfing_SecondCandleEngulfsFirst(t *testing.T) {
	// candle 1: bearish — open=105, close=100 (body 5)
	// candle 2: bullish — open=99, close=106 (body 7, engulfs: 99<100 and 106>105)
	candles := []domain.Candle{
		makeCandle(105.00, 106.00, 99.00, 100.00), // bearish
		makeCandle(99.00, 107.00, 98.00, 106.00),  // bullish, engulfs
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameBullishEngulfing)
	if p == nil {
		t.Fatal("expected bullish engulfing to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBullish {
		t.Errorf("expected bullish direction, got %s", p.Direction)
	}
	if p.AtCandle != 1 {
		t.Errorf("expected AtCandle=1, got %d", p.AtCandle)
	}
}

// -------- Bearish Engulfing --------

func TestDetectBearishEngulfing_SecondCandleEngulfsFirst(t *testing.T) {
	// candle 1: bullish — open=100, close=105 (body 5)
	// candle 2: bearish — open=106, close=99 (body 7, engulfs: 106>105 and 99<100)
	candles := []domain.Candle{
		makeCandle(100.00, 106.00, 99.00, 105.00), // bullish
		makeCandle(106.00, 107.00, 98.00, 99.00),  // bearish, engulfs
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameBearishEngulfing)
	if p == nil {
		t.Fatal("expected bearish engulfing to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBearish {
		t.Errorf("expected bearish direction, got %s", p.Direction)
	}
	if p.AtCandle != 1 {
		t.Errorf("expected AtCandle=1, got %d", p.AtCandle)
	}
}

// -------- Morning Star --------

func TestDetectMorningStar_ThreeCandleSequence(t *testing.T) {
	// c1: long bearish — open=110, close=90, high=111, low=89 (body=20, range=22 → ratio≈0.91)
	// c2: small body, gaps down — open=89, close=88.7, high=89.5, low=86.5
	//     body=0.3, range=3.0 → ratio=0.1 (≤0.3 ✓); open(89)<c1.close(90) ✓
	// c3: long bullish, closes above midpoint of c1 — midpoint=(110+90)/2=100
	//     open=88, close=106, high=107, low=87 (body=18, range=20 → ratio=0.9 ✓; close(106)>100 ✓)
	candles := []domain.Candle{
		makeCandle(110.00, 111.00, 89.00, 90.00),   // c1: long bearish
		makeCandle(89.00, 89.50, 86.50, 88.70),     // c2: small body (0.3/3.0=0.10), gaps down
		makeCandle(88.00, 107.00, 87.00, 106.00),   // c3: long bullish
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameMorningStar)
	if p == nil {
		t.Fatal("expected morning star to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBullish {
		t.Errorf("expected bullish direction, got %s", p.Direction)
	}
	if p.AtCandle != 2 {
		t.Errorf("expected AtCandle=2, got %d", p.AtCandle)
	}
}

// -------- Evening Star --------

func TestDetectEveningStar_ThreeCandleSequence(t *testing.T) {
	// c1: long bullish — open=90, close=110, high=111, low=89 (body=20, range=22 → ratio≈0.91)
	// c2: small body, gaps up — open=111, close=111.3, high=113.5, low=110.5
	//     body=0.3, range=3.0 → ratio=0.1 (≤0.3 ✓); open(111)>c1.close(110) ✓
	// c3: long bearish, closes below midpoint of c1 — midpoint=(90+110)/2=100
	//     open=112, close=94, high=113, low=93 (body=18, range=20 → ratio=0.9 ✓; close(94)<100 ✓)
	candles := []domain.Candle{
		makeCandle(90.00, 111.00, 89.00, 110.00),    // c1: long bullish
		makeCandle(111.00, 113.50, 110.50, 111.30),  // c2: small body (0.3/3.0=0.10), gaps up
		makeCandle(112.00, 113.00, 93.00, 94.00),    // c3: long bearish
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	p := findPattern(patterns, candlestick.PatternNameEveningStar)
	if p == nil {
		t.Fatal("expected evening star to be detected")
	}
	if p.Direction != candlestick.PatternDirectionBearish {
		t.Errorf("expected bearish direction, got %s", p.Direction)
	}
	if p.AtCandle != 2 {
		t.Errorf("expected AtCandle=2, got %d", p.AtCandle)
	}
}

// -------- Edge cases --------

func TestDetectPatterns_NoMatches_EmptyResult(t *testing.T) {
	// Regular trending candle — long body, no shadows, not matching any pattern.
	// open=100, high=105, low=99, close=104 → not doji (body=4/range=6=0.67)
	// not hammer (no long lower shadow), not shooting star (no long upper shadow).
	// With single candle, no 2- or 3-candle patterns possible.
	candles := []domain.Candle{
		makeCandle(100.00, 105.00, 99.00, 104.00),
	}
	detector := candlestick.NewDetector()
	patterns := detector.DetectPatterns(candles)

	if len(patterns) != 0 {
		t.Errorf("expected no patterns, got %d: %+v", len(patterns), patterns)
	}
}

func TestDetectPatterns_InsufficientCandles_EmptyResult(t *testing.T) {
	detector := candlestick.NewDetector()

	// Empty slice should return nil / empty.
	patterns := detector.DetectPatterns([]domain.Candle{})
	if len(patterns) != 0 {
		t.Errorf("expected no patterns for empty input, got %d", len(patterns))
	}
}
