package chart

import (
	"time"

	"github.com/shopspring/decimal"
)

// PriceLevel represents a price level identified as support or resistance.
type PriceLevel struct {
	Price       decimal.Decimal
	Strength    int // number of touches that confirm this level (higher = stronger)
	LastTouchAt time.Time
}

// BreakoutDirection indicates which direction a breakout occurred.
type BreakoutDirection string

const (
	BreakoutDirectionUp   BreakoutDirection = "up"   // price broke above resistance
	BreakoutDirectionDown BreakoutDirection = "down" // price broke below support
)

// BreakoutEvent describes a confirmed breakout from a key level.
type BreakoutEvent struct {
	Direction     BreakoutDirection
	BrokenLevel   decimal.Decimal
	BreakoutPrice decimal.Decimal
	VolumeRatio   decimal.Decimal // current volume / average volume (must be > 1.5 for confirmation)
	AtCandle      int             // index of the candle where breakout occurred
}

// ChartPatternName identifies a detected chart pattern.
type ChartPatternName string

const (
	ChartPatternNameDoubleTop    ChartPatternName = "double_top"
	ChartPatternNameDoubleBottom ChartPatternName = "double_bottom"
)

// DetectedChartPattern is a higher-level pattern (multi-candle, weeks/days).
type DetectedChartPattern struct {
	Name       ChartPatternName
	Direction  string          // "bullish" | "bearish"
	Confidence int             // 0-100
	KeyPrice   decimal.Decimal // the main reference price (e.g., the resistance for double top)
	AtCandle   int             // index where pattern completes
}
