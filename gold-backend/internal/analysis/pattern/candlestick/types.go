package candlestick

// PatternDirection indicates whether a pattern is bullish, bearish, or neutral.
type PatternDirection string

const (
	PatternDirectionBullish PatternDirection = "bullish"
	PatternDirectionBearish PatternDirection = "bearish"
	PatternDirectionNeutral PatternDirection = "neutral"
)

// PatternName identifies a detected candlestick pattern.
type PatternName string

const (
	PatternNameBullishEngulfing PatternName = "bullish_engulfing"
	PatternNameBearishEngulfing PatternName = "bearish_engulfing"
	PatternNameDoji             PatternName = "doji"
	PatternNameHammer           PatternName = "hammer"
	PatternNameInvertedHammer   PatternName = "inverted_hammer"
	PatternNameShootingStar     PatternName = "shooting_star"
	PatternNameMorningStar      PatternName = "morning_star"
	PatternNameEveningStar      PatternName = "evening_star"
)

// DetectedPattern is a single pattern identified by the detector.
type DetectedPattern struct {
	Name       PatternName
	Direction  PatternDirection
	Confidence int // 0-100
	AtCandle   int // index in the input slice where the pattern completes (last candle)
}
