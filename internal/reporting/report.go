package reporting

import "time"

// Report represents the Phase 1 report structure.
type Report struct {
	// Metadata
	GeneratedAt   time.Time
	StrategyCount int
	ScenarioCount int

	// Data Summary
	DataSummary DataSummary

	// Data Quality (sufficiency checks)
	DataQuality DataQualitySection

	// Strategy Metrics (sorted by strategy_id, scenario_id, entry_event_type)
	StrategyMetrics []StrategyMetricRow

	// Comparisons
	SourceComparison    []SourceComparisonRow    // NEW_TOKEN vs ACTIVE_TOKEN
	ScenarioSensitivity []ScenarioSensitivityRow // realistic vs pessimistic vs degraded

	// Replay References (strategy_id, scenario_id, candidate_id)
	ReplayReferences []ReplayReferenceRow
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
	WinRate              float64 // trade-level
	TokenWinRate         float64 // token-level (tokens with positive mean outcome / total tokens)
	OutcomeMean          float64
	OutcomeMedian        float64
	OutcomeP10           float64
	OutcomeP90           float64
	MaxDrawdown          float64
	MaxConsecutiveLosses int
}

// SourceComparisonRow compares NEW_TOKEN vs ACTIVE_TOKEN.
type SourceComparisonRow struct {
	StrategyID         string
	ScenarioID         string
	NewTokenWinRate    float64
	ActiveTokenWinRate float64
	NewTokenMedian     float64
	ActiveTokenMedian  float64
}

// ScenarioSensitivityRow compares scenarios.
type ScenarioSensitivityRow struct {
	StrategyID      string
	EntryEventType  string
	RealisticMean   float64
	PessimisticMean float64
	DegradedMean    float64
	DegradationPct  float64 // (realistic - degraded) / realistic * 100, 0 if realistic == 0
}

// ReplayReferenceRow lists replay identifiers.
type ReplayReferenceRow struct {
	StrategyID  string
	ScenarioID  string
	CandidateID string
}
