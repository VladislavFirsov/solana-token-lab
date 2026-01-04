package reporting

import "time"

// Report represents the Phase 1 report structure.
type Report struct {
	// Metadata
	GeneratedAt   time.Time
	StrategyCount int
	ScenarioCount int

	// Executive Summary (per REPORTING_SPEC.md)
	ExecutiveSummary ExecutiveSummary

	// Data Summary
	DataSummary DataSummary

	// Data Quality (sufficiency checks)
	DataQuality DataQualitySection

	// Strategy Metrics (sorted by strategy_id, scenario_id, entry_event_type)
	StrategyMetrics []StrategyMetricRow

	// Comparisons
	SourceComparison    []SourceComparisonRow    // NEW_TOKEN vs ACTIVE_TOKEN
	ScenarioSensitivity []ScenarioSensitivityRow // optimistic vs realistic vs pessimistic vs degraded

	// Replay References (strategy_id, scenario_id, candidate_id)
	ReplayReferences []ReplayReferenceRow

	// Reproducibility metadata (per REPORTING_SPEC.md)
	Reproducibility ReproducibilityMetadata

	// Decision Checklist reference
	DecisionChecklistRef string
}

// ExecutiveSummary contains key decision metrics.
type ExecutiveSummary struct {
	Decision           string    // GO / NO-GO / INSUFFICIENT_DATA
	BestStrategy       string    // strategy_id of best performer
	BestEntryType      string    // entry_event_type of best performer
	WinRateRealistic   float64   // win rate under realistic scenario
	MedianRealistic    float64   // median outcome under realistic scenario
	MedianPessimistic  float64   // median outcome under pessimistic scenario
	DataPeriodStart    time.Time // data start time
	DataPeriodEnd      time.Time // data end time
	NewTokenCount      int       // count of NEW_TOKEN candidates
	ActiveTokenCount   int       // count of ACTIVE_TOKEN candidates
}

// ReproducibilityMetadata contains version info for reproducibility.
type ReproducibilityMetadata struct {
	ReportTimestamp   time.Time // report generation time
	GeneratorVersion  string    // report generator version
	DataVersion       string    // SHA256 hash of input data
	StrategyVersion   string    // git commit or semver of strategy code
	ReplayCommitHash  string    // git commit for replay
	ReplayCommand     string    // command to reproduce the report
}

// DataQualitySection contains data sufficiency checks and integrity errors.
type DataQualitySection struct {
	SufficiencyChecks []SufficiencyCheckRow
	IntegrityErrors   []string
	AllChecksPassed   bool
}

// SufficiencyCheckRow represents one sufficiency criterion.
type SufficiencyCheckRow struct {
	Name      string
	Threshold string
	Actual    string
	Pass      bool
}

// DataSummary contains data description.
type DataSummary struct {
	TotalCandidates       int
	NewTokenCandidates    int
	ActiveTokenCandidates int
	TotalTrades           int
	DateRangeStart        int64 // Unix ms
	DateRangeEnd          int64 // Unix ms
}

// StrategyMetricRow represents one row in strategy metrics table.
type StrategyMetricRow struct {
	StrategyID           string
	ScenarioID           string
	EntryEventType       string
	TotalTrades          int
	TotalTokens          int
	Wins                 int     // number of winning trades
	Losses               int     // number of losing trades
	WinRate              float64 // trade-level
	TokenWinRate         float64 // token-level (tokens with positive mean outcome / total tokens)
	OutcomeMean          float64
	OutcomeMedian        float64
	OutcomeP10           float64
	OutcomeP25           float64 // 25th percentile
	OutcomeP75           float64 // 75th percentile
	OutcomeP90           float64
	OutcomeMin           float64
	OutcomeMax           float64
	OutcomeStddev        float64
	MaxDrawdown          float64
	MaxConsecutiveLosses int
}

// SourceComparisonRow compares NEW_TOKEN vs ACTIVE_TOKEN (Realistic scenario only per REPORTING_SPEC.md).
type SourceComparisonRow struct {
	StrategyID         string
	ScenarioID         string // should always be "realistic"
	NewTokenWinRate    float64
	ActiveTokenWinRate float64
	DeltaWinRate       float64 // NewTokenWinRate - ActiveTokenWinRate
	NewTokenMedian     float64
	ActiveTokenMedian  float64
	DeltaMedian        float64 // NewTokenMedian - ActiveTokenMedian
}

// ScenarioSensitivityRow compares scenarios using median (per REPORTING_SPEC.md).
type ScenarioSensitivityRow struct {
	StrategyID         string
	EntryEventType     string
	OptimisticMedian   float64 // median outcome under optimistic scenario
	RealisticMedian    float64 // median outcome under realistic scenario
	PessimisticMedian  float64 // median outcome under pessimistic scenario
	DegradedMedian     float64 // median outcome under degraded scenario
	DegradationPct     float64 // (realistic - pessimistic) / realistic * 100, 0 if realistic == 0
}

// ReplayReferenceRow lists replay identifiers.
type ReplayReferenceRow struct {
	StrategyID  string
	ScenarioID  string
	CandidateID string
}
