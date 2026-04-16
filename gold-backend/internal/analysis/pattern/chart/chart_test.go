package chart

import (
	"testing"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// makeCandle builds a domain.Candle with the supplied OHLCV values.
// CloseTime increments by one hour per index to give distinct timestamps.
func makeCandle(index int, open, high, low, close, volume float64) domain.Candle {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return domain.Candle{
		OpenTime:   base.Add(time.Duration(index) * time.Hour),
		CloseTime:  base.Add(time.Duration(index+1) * time.Hour),
		OpenPrice:  decimal.NewFromFloat(open),
		HighPrice:  decimal.NewFromFloat(high),
		LowPrice:   decimal.NewFromFloat(low),
		ClosePrice: decimal.NewFromFloat(close),
		Volume:     decimal.NewFromFloat(volume),
	}
}

// buildZigZagCandles builds a series of candles whose highs and lows alternate
// between two sequences, creating obvious swing points.
// The body of each candle spans the midpoint between the two extremes.
func buildZigZagCandles() []domain.Candle {
	// Index:    0     1     2     3     4     5     6     7     8     9     10    11    12
	// High:   102   110   104   112   105   108   103   115   104   111   103   109   102
	// Low:     98    93    96    90    95    94    97    88    96    91    97    92    98
	// Swing highs expected at index 1 (110), 3 (112), 7 (115), 9 (111) with pivotWindow=2
	// Swing lows expected at index 2 (96), 4 (95), 6 (97), ...
	data := []struct{ high, low float64 }{
		{102, 98},  // 0
		{110, 93},  // 1 — swing high
		{104, 96},  // 2 — swing low
		{112, 90},  // 3 — swing high
		{105, 95},  // 4 — swing low (with window=2, 95 < 96 and 95 < 94 — actually not strict)
		{108, 94},  // 5
		{103, 97},  // 6
		{115, 88},  // 7 — swing high, swing low
		{104, 96},  // 8
		{111, 91},  // 9
		{103, 97},  // 10
		{109, 92},  // 11
		{102, 98},  // 12
	}
	candles := make([]domain.Candle, len(data))
	for i, d := range data {
		mid := (d.high + d.low) / 2.0
		candles[i] = makeCandle(i, mid-0.5, d.high, d.low, mid+0.5, 100)
	}
	return candles
}

// ---- TestFindSwingPoints_SimpleZigZag ----

func TestFindSwingPoints_SimpleZigZag(t *testing.T) {
	// Build a deterministic zigzag: alternating clear peaks and troughs.
	// Pattern (high/low per index):
	//   idx 0: H=100, L=98
	//   idx 1: H=110, L=99   <- peak (H=110 > H[0]=100 and H[2]=104)
	//   idx 2: H=104, L=96   <- trough (L=96 < L[1]=99 and L[3]=99)
	//   idx 3: H=113, L=99   <- peak
	//   idx 4: H=106, L=95   <- trough
	//   idx 5: H=100, L=98
	candles := []domain.Candle{
		makeCandle(0, 99, 100, 98, 99, 100),
		makeCandle(1, 105, 110, 99, 106, 100),
		makeCandle(2, 100, 104, 96, 100, 100),
		makeCandle(3, 106, 113, 99, 107, 100),
		makeCandle(4, 101, 106, 95, 101, 100),
		makeCandle(5, 99, 100, 98, 99, 100),
	}

	pivotWindow := 1
	points := FindSwingPoints(candles, pivotWindow)

	if len(points) == 0 {
		t.Fatal("expected swing points, got none")
	}

	var highs, lows []SwingPoint
	for _, p := range points {
		if p.IsHigh {
			highs = append(highs, p)
		} else {
			lows = append(lows, p)
		}
	}

	if len(highs) == 0 {
		t.Error("expected at least one swing high, got none")
	}
	if len(lows) == 0 {
		t.Error("expected at least one swing low, got none")
	}

	// Index 1 should be a swing high (110 > 100 and 110 > 104)
	foundHigh1 := false
	for _, h := range highs {
		if h.CandleIndex == 1 {
			foundHigh1 = true
			if !h.Price.Equal(decimal.NewFromFloat(110)) {
				t.Errorf("swing high at index 1: expected price 110, got %s", h.Price)
			}
		}
	}
	if !foundHigh1 {
		t.Error("expected swing high at candle index 1")
	}

	// Index 2 should be a swing low (96 < 99 and 96 < 99)
	foundLow2 := false
	for _, l := range lows {
		if l.CandleIndex == 2 {
			foundLow2 = true
			if !l.Price.Equal(decimal.NewFromFloat(96)) {
				t.Errorf("swing low at index 2: expected price 96, got %s", l.Price)
			}
		}
	}
	if !foundLow2 {
		t.Error("expected swing low at candle index 2")
	}
}

// ---- TestIdentifyResistanceLevels_TwoTouchesGroupedIntoLevel ----

func TestIdentifyResistanceLevels_TwoTouchesGroupedIntoLevel(t *testing.T) {
	// Construct two distinct swing highs at nearly identical prices (110.0, 110.3)
	// separated by a swing low, then surrounded by padding candles.
	candles := []domain.Candle{
		makeCandle(0, 100, 102, 99, 101, 100),  // pad
		makeCandle(1, 100, 102, 99, 101, 100),  // pad
		makeCandle(2, 108, 110, 107, 109, 100), // swing high ~110
		makeCandle(3, 105, 107, 100, 102, 100), // swing low between peaks
		makeCandle(4, 108, 110.3, 107, 109, 100), // swing high ~110.3 (within 0.5% of 110)
		makeCandle(5, 100, 102, 99, 101, 100),  // pad
		makeCandle(6, 100, 102, 99, 101, 100),  // pad
	}

	levels := IdentifyResistanceLevels(candles, 1, 0.5)

	if len(levels) == 0 {
		t.Fatal("expected at least one resistance level, got none")
	}

	// The two highs (110.0, 110.3) are within 0.5% of each other (diff = 0.27%)
	// and should be grouped into a single level with strength = 2.
	found := false
	for _, lvl := range levels {
		if lvl.Strength >= 2 {
			// Level price should be around 110.15 (average of 110.0 and 110.3)
			expected := decimal.NewFromFloat(110.15)
			diff := lvl.Price.Sub(expected).Abs()
			if diff.LessThan(decimal.NewFromFloat(0.5)) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a resistance level around 110.15 with strength >= 2; got levels: %+v", levels)
	}
}

// ---- TestIdentifySupportLevels_TwoTouchesGroupedIntoLevel ----

func TestIdentifySupportLevels_TwoTouchesGroupedIntoLevel(t *testing.T) {
	// Two swing lows at similar prices (90.0, 90.2) within 0.5% tolerance.
	candles := []domain.Candle{
		makeCandle(0, 100, 102, 99, 101, 100),  // pad
		makeCandle(1, 100, 102, 99, 101, 100),  // pad
		makeCandle(2, 92, 94, 90.0, 91, 100),   // swing low ~90.0
		makeCandle(3, 95, 97, 94, 96, 100),     // swing high between troughs
		makeCandle(4, 92, 94, 90.2, 91, 100),   // swing low ~90.2 (within 0.5% of 90.0)
		makeCandle(5, 100, 102, 99, 101, 100),  // pad
		makeCandle(6, 100, 102, 99, 101, 100),  // pad
	}

	levels := IdentifySupportLevels(candles, 1, 0.5)

	if len(levels) == 0 {
		t.Fatal("expected at least one support level, got none")
	}

	found := false
	for _, lvl := range levels {
		if lvl.Strength >= 2 {
			expected := decimal.NewFromFloat(90.1)
			diff := lvl.Price.Sub(expected).Abs()
			if diff.LessThan(decimal.NewFromFloat(0.5)) {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected a support level around 90.1 with strength >= 2; got levels: %+v", levels)
	}
}

// ---- TestDetectBreakouts_PriceBreaksResistanceWithVolume_DetectsUp ----

func TestDetectBreakouts_PriceBreaksResistanceWithVolume_DetectsUp(t *testing.T) {
	// 22 candles: first 20 with volume=100, candle 21 (breakout) with volume=200.
	// Resistance level at 110. Last candle closes at 112.
	candles := make([]domain.Candle, 22)
	for i := 0; i < 21; i++ {
		candles[i] = makeCandle(i, 105, 108, 104, 106, 100)
	}
	// Breakout candle: close above resistance level, volume = 200 (ratio = 2.0 >= 1.5)
	candles[21] = makeCandle(21, 109, 113, 108, 112, 200)

	resistance := []PriceLevel{
		{Price: decimal.NewFromFloat(110), Strength: 2},
	}

	events := DetectBreakouts(candles, nil, resistance, 20, 1.5)

	if len(events) == 0 {
		t.Fatal("expected a breakout event, got none")
	}

	found := false
	for _, e := range events {
		if e.Direction == BreakoutDirectionUp {
			found = true
			if !e.BrokenLevel.Equal(decimal.NewFromFloat(110)) {
				t.Errorf("expected broken level 110, got %s", e.BrokenLevel)
			}
			if !e.BreakoutPrice.Equal(decimal.NewFromFloat(112)) {
				t.Errorf("expected breakout price 112, got %s", e.BreakoutPrice)
			}
			if e.VolumeRatio.LessThan(decimal.NewFromFloat(1.5)) {
				t.Errorf("expected volume ratio >= 1.5, got %s", e.VolumeRatio)
			}
		}
	}
	if !found {
		t.Error("expected breakout direction 'up', not found in events")
	}
}

// ---- TestDetectBreakouts_PriceBreaksResistanceWithoutVolume_DoesNotDetect ----

func TestDetectBreakouts_PriceBreaksResistanceWithoutVolume_DoesNotDetect(t *testing.T) {
	// Same setup but volume on breakout candle is only 120 (ratio = 1.2, below 1.5 threshold).
	candles := make([]domain.Candle, 22)
	for i := 0; i < 21; i++ {
		candles[i] = makeCandle(i, 105, 108, 104, 106, 100)
	}
	candles[21] = makeCandle(21, 109, 113, 108, 112, 120) // volume ratio = 1.2

	resistance := []PriceLevel{
		{Price: decimal.NewFromFloat(110), Strength: 2},
	}

	events := DetectBreakouts(candles, nil, resistance, 20, 1.5)

	if len(events) != 0 {
		t.Errorf("expected no breakout events (volume too low), got %d", len(events))
	}
}

// ---- TestDetectBreakouts_PriceBreaksSupportWithVolume_DetectsDown ----

func TestDetectBreakouts_PriceBreaksSupportWithVolume_DetectsDown(t *testing.T) {
	candles := make([]domain.Candle, 22)
	for i := 0; i < 21; i++ {
		candles[i] = makeCandle(i, 105, 108, 104, 106, 100)
	}
	// Breakdown candle: close below support level at 100, volume = 200
	candles[21] = makeCandle(21, 103, 104, 97, 98, 200)

	support := []PriceLevel{
		{Price: decimal.NewFromFloat(100), Strength: 2},
	}

	events := DetectBreakouts(candles, support, nil, 20, 1.5)

	if len(events) == 0 {
		t.Fatal("expected a breakdown event, got none")
	}

	found := false
	for _, e := range events {
		if e.Direction == BreakoutDirectionDown {
			found = true
			if !e.BrokenLevel.Equal(decimal.NewFromFloat(100)) {
				t.Errorf("expected broken level 100, got %s", e.BrokenLevel)
			}
		}
	}
	if !found {
		t.Error("expected breakout direction 'down', not found in events")
	}
}

// ---- TestDetectDoubleTopBottom_ClassicDoubleTop_Detected ----

func TestDetectDoubleTopBottom_ClassicDoubleTop_Detected(t *testing.T) {
	// Build a sequence with a clear double top:
	// pad - peak1 (~115) - trough (~105) - peak2 (~115) - confirmation close below trough
	candles := []domain.Candle{
		makeCandle(0, 108, 110, 107, 109, 100), // pad
		makeCandle(1, 108, 110, 107, 109, 100), // pad
		makeCandle(2, 112, 115, 110, 113, 100), // peak 1
		makeCandle(3, 108, 110, 105, 107, 100), // trough (neckline ~105)
		makeCandle(4, 112, 115.2, 110, 113, 100), // peak 2 (within 0.5% of 115)
		makeCandle(5, 108, 110, 107, 109, 100), // pad
		makeCandle(6, 108, 110, 107, 109, 100), // pad
		makeCandle(7, 103, 104, 101, 103, 100), // confirmation: close below 105 neckline
	}

	patterns := DetectDoubleTopBottom(candles, 1, 0.5)

	found := false
	for _, p := range patterns {
		if p.Name == ChartPatternNameDoubleTop && p.Direction == "bearish" {
			found = true
			if p.Confidence < 50 || p.Confidence > 100 {
				t.Errorf("expected confidence 50-100, got %d", p.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("expected a double_top bearish pattern; got patterns: %+v", patterns)
	}
}

// ---- TestDetectDoubleTopBottom_ClassicDoubleBottom_Detected ----

func TestDetectDoubleTopBottom_ClassicDoubleBottom_Detected(t *testing.T) {
	// Build a sequence with a clear double bottom:
	// pad - trough1 (~88) - peak (~98) - trough2 (~88) - confirmation close above peak
	candles := []domain.Candle{
		makeCandle(0, 93, 95, 92, 94, 100),    // pad
		makeCandle(1, 93, 95, 92, 94, 100),    // pad
		makeCandle(2, 90, 92, 88.0, 89, 100),  // trough 1
		makeCandle(3, 95, 98, 93, 97, 100),    // peak (neckline ~98)
		makeCandle(4, 90, 92, 88.2, 89, 100),  // trough 2 (within 0.5% of 88)
		makeCandle(5, 93, 95, 92, 94, 100),    // pad
		makeCandle(6, 93, 95, 92, 94, 100),    // pad
		makeCandle(7, 99, 101, 98, 100, 100),  // confirmation: close above 98 neckline
	}

	patterns := DetectDoubleTopBottom(candles, 1, 0.5)

	found := false
	for _, p := range patterns {
		if p.Name == ChartPatternNameDoubleBottom && p.Direction == "bullish" {
			found = true
			if p.Confidence < 50 || p.Confidence > 100 {
				t.Errorf("expected confidence 50-100, got %d", p.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("expected a double_bottom bullish pattern; got patterns: %+v", patterns)
	}
}

// ---- TestAnalyzer_FullSeries_ReturnsAllResults ----

func TestAnalyzer_FullSeries_ReturnsAllResults(t *testing.T) {
	// Build a 60-candle series with identifiable structure:
	// - candles 0-57: alternating prices with volume=100
	// - two swing highs near 110 (resistance) and two swing lows near 90 (support)
	// - last candle (index 59) closes above 110 with volume=200 (breakout)
	candles := make([]domain.Candle, 60)

	// Fill the series with a subtle zigzag to generate swing points
	prices := []float64{
		100, 110, 95, 108, 96, 109.8, 97, 104, // 0-7: two highs near 110
		100, 90, 103, 89.8, 104, 100, 102, 98,  // 8-15: two lows near 90
		100, 101, 99, 102, 98, 103, 97, 102,    // 16-23
		100, 101, 99, 102, 98, 103, 97, 102,    // 24-31
		100, 101, 99, 102, 98, 103, 97, 102,    // 32-39
		100, 101, 99, 102, 98, 103, 97, 102,    // 40-47
		100, 101, 99, 102, 98, 103, 97, 102,    // 48-55
		100, 101, 100, 102,                     // 56-59
	}

	for i := 0; i < 59; i++ {
		p := prices[i]
		candles[i] = makeCandle(i, p-1, p+1, p-2, p, 100)
	}
	// Breakout candle: close at 113 (above resistance ~110), high volume
	candles[59] = makeCandle(59, 109, 115, 108, 113, 200)

	analyzer := NewAnalyzer(AnalyzerConfig{
		PivotWindow:           2,
		LevelTolerancePercent: 1.0,
		VolumeAverageWindow:   20,
		VolumeRatioThreshold:  1.5,
	})

	result := analyzer.Analyze(candles)

	// The result struct must be returned without panicking
	// and must have the correct field types.
	_ = result.SupportLevels
	_ = result.ResistanceLevels
	_ = result.Breakouts
	_ = result.Patterns

	t.Logf("support levels: %d, resistance levels: %d, breakouts: %d, patterns: %d",
		len(result.SupportLevels),
		len(result.ResistanceLevels),
		len(result.Breakouts),
		len(result.Patterns),
	)
}

// ---- TestNewAnalyzer_DefaultsApplied ----

func TestNewAnalyzer_DefaultsApplied(t *testing.T) {
	analyzer := NewAnalyzer(AnalyzerConfig{}) // all zeros

	if analyzer.config.PivotWindow != 3 {
		t.Errorf("expected PivotWindow=3, got %d", analyzer.config.PivotWindow)
	}
	if analyzer.config.LevelTolerancePercent != 0.5 {
		t.Errorf("expected LevelTolerancePercent=0.5, got %f", analyzer.config.LevelTolerancePercent)
	}
	if analyzer.config.VolumeAverageWindow != 20 {
		t.Errorf("expected VolumeAverageWindow=20, got %d", analyzer.config.VolumeAverageWindow)
	}
	if analyzer.config.VolumeRatioThreshold != 1.5 {
		t.Errorf("expected VolumeRatioThreshold=1.5, got %f", analyzer.config.VolumeRatioThreshold)
	}
}
