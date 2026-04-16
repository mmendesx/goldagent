package chart

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// AnalyzerConfig configures the chart pattern analyzer.
type AnalyzerConfig struct {
	PivotWindow           int     // default 3
	LevelTolerancePercent float64 // default 0.5 (0.5%)
	VolumeAverageWindow   int     // default 20
	VolumeRatioThreshold  float64 // default 1.5
}

// AnalysisResult bundles all chart-pattern findings from a single Analyze call.
type AnalysisResult struct {
	SupportLevels    []PriceLevel
	ResistanceLevels []PriceLevel
	Breakouts        []BreakoutEvent
	Patterns         []DetectedChartPattern
}

// Analyzer is the entry point — runs all chart detectors in order.
type Analyzer struct {
	config AnalyzerConfig
}

// NewAnalyzer constructs an Analyzer with the given config.
// Zero values in config are replaced with sensible defaults:
// PivotWindow=3, LevelTolerancePercent=0.5, VolumeAverageWindow=20, VolumeRatioThreshold=1.5.
func NewAnalyzer(config AnalyzerConfig) *Analyzer {
	if config.PivotWindow == 0 {
		config.PivotWindow = 3
	}
	if config.LevelTolerancePercent == 0 {
		config.LevelTolerancePercent = 0.5
	}
	if config.VolumeAverageWindow == 0 {
		config.VolumeAverageWindow = 20
	}
	if config.VolumeRatioThreshold == 0 {
		config.VolumeRatioThreshold = 1.5
	}
	return &Analyzer{config: config}
}

// Analyze runs all chart-pattern detectors over the given candle series.
// Requires at least ~50 candles for meaningful results; shorter series will
// return partial results based on what can be computed.
func (analyzer *Analyzer) Analyze(candles []domain.Candle) AnalysisResult {
	cfg := analyzer.config

	supportLevels := IdentifySupportLevels(candles, cfg.PivotWindow, cfg.LevelTolerancePercent)
	resistanceLevels := IdentifyResistanceLevels(candles, cfg.PivotWindow, cfg.LevelTolerancePercent)

	breakouts := DetectBreakouts(
		candles,
		supportLevels,
		resistanceLevels,
		cfg.VolumeAverageWindow,
		cfg.VolumeRatioThreshold,
	)

	patterns := DetectDoubleTopBottom(candles, cfg.PivotWindow, cfg.LevelTolerancePercent)

	return AnalysisResult{
		SupportLevels:    supportLevels,
		ResistanceLevels: resistanceLevels,
		Breakouts:        breakouts,
		Patterns:         patterns,
	}
}
