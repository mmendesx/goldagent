package engine

import (
	"testing"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/shopspring/decimal"
)

// d is a shorthand for decimal.NewFromFloat in tests.
func d(v float64) decimal.Decimal {
	return decimal.NewFromFloat(v)
}

// assertScoreEqual fails if actual != expected, printing a descriptive message.
func assertScoreEqual(t *testing.T, name string, expected, actual decimal.Decimal) {
	t.Helper()
	if !expected.Equal(actual) {
		t.Errorf("%s: expected %s, got %s", name, expected.String(), actual.String())
	}
}

// assertScoreRange fails if actual is outside [min, max].
func assertScoreRange(t *testing.T, name string, min, max, actual decimal.Decimal) {
	t.Helper()
	if actual.LessThan(min) || actual.GreaterThan(max) {
		t.Errorf("%s: expected value in [%s, %s], got %s", name, min.String(), max.String(), actual.String())
	}
}

// --- ScoreFromRsi ---

func TestScoreFromRsi_Neutral(t *testing.T) {
	// RSI 50 → score 0
	assertScoreEqual(t, "RSI 50", decimal.Zero, ScoreFromRsi(d(50)))
}

func TestScoreFromRsi_Oversold(t *testing.T) {
	// RSI 20 → (50-20)*2 = +60
	assertScoreEqual(t, "RSI 20", d(60), ScoreFromRsi(d(20)))
}

func TestScoreFromRsi_DeepOversold(t *testing.T) {
	// RSI 0 → (50-0)*2 = 100 (clamped)
	assertScoreEqual(t, "RSI 0", d(100), ScoreFromRsi(d(0)))
}

func TestScoreFromRsi_Overbought(t *testing.T) {
	// RSI 80 → (50-80)*2 = -60
	assertScoreEqual(t, "RSI 80", d(-60), ScoreFromRsi(d(80)))
}

func TestScoreFromRsi_DeepOverbought(t *testing.T) {
	// RSI 100 → (50-100)*2 = -100 (clamped)
	assertScoreEqual(t, "RSI 100", d(-100), ScoreFromRsi(d(100)))
}

func TestScoreFromRsi_MildOversold(t *testing.T) {
	// RSI 30 → (50-30)*2 = +40
	assertScoreEqual(t, "RSI 30", d(40), ScoreFromRsi(d(30)))
}

func TestScoreFromRsi_MildOverbought(t *testing.T) {
	// RSI 70 → (50-70)*2 = -40
	assertScoreEqual(t, "RSI 70", d(-40), ScoreFromRsi(d(70)))
}

// --- ScoreFromMacdHistogram ---

func TestScoreFromMacdHistogram_Zero(t *testing.T) {
	assertScoreEqual(t, "histogram 0", decimal.Zero, ScoreFromMacdHistogram(d(0), d(50000)))
}

func TestScoreFromMacdHistogram_ZeroPrice(t *testing.T) {
	// Zero price → no score possible
	assertScoreEqual(t, "zero price", decimal.Zero, ScoreFromMacdHistogram(d(100), d(0)))
}

func TestScoreFromMacdHistogram_BullishSign(t *testing.T) {
	score := ScoreFromMacdHistogram(d(100), d(50000))
	if !score.IsPositive() {
		t.Errorf("positive histogram should produce positive score, got %s", score.String())
	}
}

func TestScoreFromMacdHistogram_BearishSign(t *testing.T) {
	score := ScoreFromMacdHistogram(d(-100), d(50000))
	if !score.IsNegative() {
		t.Errorf("negative histogram should produce negative score, got %s", score.String())
	}
}

func TestScoreFromMacdHistogram_Clamped(t *testing.T) {
	// Very large histogram should still be clamped to [-100, 100]
	score := ScoreFromMacdHistogram(d(1_000_000), d(1))
	assertScoreEqual(t, "large histogram clamped to +100", d(100), score)
}

// --- ScoreFromBollingerPosition ---

func TestScoreFromBollingerPosition_AtMiddle(t *testing.T) {
	// Price at middle → 0
	score := ScoreFromBollingerPosition(d(100), d(110), d(100), d(90))
	assertScoreEqual(t, "price at middle", decimal.Zero, score)
}

func TestScoreFromBollingerPosition_AboveUpper(t *testing.T) {
	// Price above upper band → bearish (negative)
	score := ScoreFromBollingerPosition(d(115), d(110), d(100), d(90))
	if !score.IsNegative() {
		t.Errorf("price above upper band should produce negative score, got %s", score.String())
	}
}

func TestScoreFromBollingerPosition_BelowLower(t *testing.T) {
	// Price below lower band → bullish (positive)
	score := ScoreFromBollingerPosition(d(85), d(110), d(100), d(90))
	if !score.IsPositive() {
		t.Errorf("price below lower band should produce positive score, got %s", score.String())
	}
}

func TestScoreFromBollingerPosition_BetweenMiddleAndUpper(t *testing.T) {
	// Price between middle and upper → mild bearish (negative, price above middle)
	score := ScoreFromBollingerPosition(d(105), d(110), d(100), d(90))
	if !score.IsNegative() {
		t.Errorf("price above middle should produce negative score, got %s", score.String())
	}
}

func TestScoreFromBollingerPosition_ZeroBandWidth(t *testing.T) {
	// Degenerate case: all bands equal
	score := ScoreFromBollingerPosition(d(100), d(100), d(100), d(100))
	assertScoreEqual(t, "zero band width", decimal.Zero, score)
}

// --- ScoreFromEmaAlignment ---

func TestScoreFromEmaAlignment_FullyBullish(t *testing.T) {
	// EMA9 > EMA21 > EMA50 > EMA200
	score := ScoreFromEmaAlignment(d(200), d(150), d(100), d(50))
	if score.LessThan(d(90)) {
		t.Errorf("fully bullish EMA alignment should score near +99, got %s", score.String())
	}
}

func TestScoreFromEmaAlignment_FullyBearish(t *testing.T) {
	// EMA9 < EMA21 < EMA50 < EMA200
	score := ScoreFromEmaAlignment(d(50), d(100), d(150), d(200))
	if score.GreaterThan(d(-90)) {
		t.Errorf("fully bearish EMA alignment should score near -99, got %s", score.String())
	}
}

func TestScoreFromEmaAlignment_Neutral(t *testing.T) {
	// All EMAs equal → 0
	score := ScoreFromEmaAlignment(d(100), d(100), d(100), d(100))
	assertScoreEqual(t, "all EMAs equal", decimal.Zero, score)
}

func TestScoreFromEmaAlignment_PartialBullish(t *testing.T) {
	// EMA9 > EMA21, but EMA21 < EMA50 < EMA200 (2 bearish, 1 bullish)
	score := ScoreFromEmaAlignment(d(200), d(100), d(150), d(200))
	// 1 bullish (+33) - 2 bearish (-66) = -33
	if score.IsPositive() {
		t.Errorf("mixed EMA alignment (1 bull, 2 bear) should score negative, got %s", score.String())
	}
}

// --- ScoreFromCandlestickPatterns ---

func TestScoreFromCandlestickPatterns_NoPatterns(t *testing.T) {
	assertScoreEqual(t, "empty patterns", decimal.Zero, ScoreFromCandlestickPatterns(nil))
}

func TestScoreFromCandlestickPatterns_BullishOnly(t *testing.T) {
	patterns := []candlestick.DetectedPattern{
		{Name: candlestick.PatternNameHammer, Direction: candlestick.PatternDirectionBullish, Confidence: 80},
	}
	score := ScoreFromCandlestickPatterns(patterns)
	assertScoreEqual(t, "single bullish pattern 80% confidence", d(80), score)
}

func TestScoreFromCandlestickPatterns_BearishOnly(t *testing.T) {
	patterns := []candlestick.DetectedPattern{
		{Name: candlestick.PatternNameShootingStar, Direction: candlestick.PatternDirectionBearish, Confidence: 70},
	}
	score := ScoreFromCandlestickPatterns(patterns)
	assertScoreEqual(t, "single bearish pattern 70% confidence", d(-70), score)
}

func TestScoreFromCandlestickPatterns_Mixed(t *testing.T) {
	patterns := []candlestick.DetectedPattern{
		{Direction: candlestick.PatternDirectionBullish, Confidence: 80},
		{Direction: candlestick.PatternDirectionBearish, Confidence: 80},
	}
	// total=0, avg=0
	score := ScoreFromCandlestickPatterns(patterns)
	assertScoreEqual(t, "equal bullish and bearish", decimal.Zero, score)
}

func TestScoreFromCandlestickPatterns_NeutralIgnored(t *testing.T) {
	patterns := []candlestick.DetectedPattern{
		{Direction: candlestick.PatternDirectionNeutral, Confidence: 100},
		{Direction: candlestick.PatternDirectionBullish, Confidence: 60},
	}
	// total=60, count=2, avg=30
	score := ScoreFromCandlestickPatterns(patterns)
	assertScoreEqual(t, "neutral+bullish: avg 30", d(30), score)
}

// --- ScoreFromSupportResistance ---

func TestScoreFromSupportResistance_NoLevelsNoBreakouts(t *testing.T) {
	analysis := chart.AnalysisResult{}
	score := ScoreFromSupportResistance(d(100), analysis)
	assertScoreEqual(t, "empty analysis", decimal.Zero, score)
}

func TestScoreFromSupportResistance_UpBreakout(t *testing.T) {
	analysis := chart.AnalysisResult{
		Breakouts: []chart.BreakoutEvent{
			{Direction: chart.BreakoutDirectionUp},
		},
	}
	score := ScoreFromSupportResistance(d(100), analysis)
	if !score.IsPositive() {
		t.Errorf("upward breakout should produce positive score, got %s", score.String())
	}
}

func TestScoreFromSupportResistance_DownBreakout(t *testing.T) {
	analysis := chart.AnalysisResult{
		Breakouts: []chart.BreakoutEvent{
			{Direction: chart.BreakoutDirectionDown},
		},
	}
	score := ScoreFromSupportResistance(d(100), analysis)
	if !score.IsNegative() {
		t.Errorf("downward breakout should produce negative score, got %s", score.String())
	}
}

func TestScoreFromSupportResistance_ScoreInRange(t *testing.T) {
	analysis := chart.AnalysisResult{
		Breakouts: []chart.BreakoutEvent{
			{Direction: chart.BreakoutDirectionUp},
			{Direction: chart.BreakoutDirectionDown},
		},
	}
	score := ScoreFromSupportResistance(d(100), analysis)
	assertScoreRange(t, "mixed breakouts", d(-100), d(100), score)
}

// --- ScoreFromSentiment ---

func TestScoreFromSentiment_Positive(t *testing.T) {
	// rawSentiment 0.5 → 50
	assertScoreEqual(t, "sentiment 0.5", d(50), ScoreFromSentiment(d(0.5)))
}

func TestScoreFromSentiment_Negative(t *testing.T) {
	// rawSentiment -0.75 → -75
	assertScoreEqual(t, "sentiment -0.75", d(-75), ScoreFromSentiment(d(-0.75)))
}

func TestScoreFromSentiment_Zero(t *testing.T) {
	assertScoreEqual(t, "sentiment 0", decimal.Zero, ScoreFromSentiment(decimal.Zero))
}

func TestScoreFromSentiment_MaxPositive(t *testing.T) {
	assertScoreEqual(t, "sentiment 1.0", d(100), ScoreFromSentiment(d(1.0)))
}

func TestScoreFromSentiment_MaxNegative(t *testing.T) {
	assertScoreEqual(t, "sentiment -1.0", d(-100), ScoreFromSentiment(d(-1.0)))
}

func TestScoreFromSentiment_AboveMax_Clamped(t *testing.T) {
	// rawSentiment above 1.0 should be clamped to 100
	assertScoreEqual(t, "sentiment 2.0 clamped", d(100), ScoreFromSentiment(d(2.0)))
}
