package domain

// StrategyAggregate represents per-strategy aggregate metrics.
// Corresponds to strategy_aggregates table in SIMULATION_SPEC.md.
type StrategyAggregate struct {
	StrategyID     string // strategy identifier
	ScenarioID     string // execution scenario
	EntryEventType string // NEW_TOKEN | ACTIVE_TOKEN

	// Counts
	TotalTrades  int
	TotalTokens  int // unique candidate_id count
	Wins         int
	Losses       int
	WinRate      float64 // wins / total_trades (trade-level)
	TokenWinRate float64 // tokens with positive mean outcome / total tokens (token-level)

	// Outcome Distribution
	OutcomeMean   float64
	OutcomeMedian float64
	OutcomeP10    float64 // 10th percentile
	OutcomeP25    float64 // 25th percentile
	OutcomeP75    float64 // 75th percentile
	OutcomeP90    float64 // 90th percentile
	OutcomeMin    float64
	OutcomeMax    float64
	OutcomeStddev float64

	// Drawdown
	MaxDrawdown          float64 // worst peak-to-trough
	MaxConsecutiveLosses int

	// Sensitivity (cross-scenario comparison)
	OutcomeRealistic   *float64 // baseline (Realistic scenario)
	OutcomePessimistic *float64 // Pessimistic scenario
	OutcomeDegraded    *float64 // Degraded scenario
}

// StrategyConfig represents strategy configuration parameters.
type StrategyConfig struct {
	StrategyType   string // "TIME_EXIT" | "TRAILING_STOP" | "LIQUIDITY_GUARD"
	EntryEventType string // "NEW_TOKEN" | "ACTIVE_TOKEN"

	// TIME_EXIT parameters
	HoldDurationMs *int64

	// TRAILING_STOP parameters
	TrailPct       *float64
	InitialStopPct *float64

	// LIQUIDITY_GUARD parameters
	LiquidityDropPct *float64

	// Common parameters
	MaxHoldDurationMs *int64
}

// Strategy type constants
const (
	StrategyTypeTimeExit       = "TIME_EXIT"
	StrategyTypeTrailingStop   = "TRAILING_STOP"
	StrategyTypeLiquidityGuard = "LIQUIDITY_GUARD"
)
