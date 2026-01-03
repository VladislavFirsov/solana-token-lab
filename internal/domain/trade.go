package domain

// TradeRecord represents a simulated trade with full execution details.
// Corresponds to trade_records table in SIMULATION_SPEC.md.
type TradeRecord struct {
	TradeID     string // deterministic hash
	CandidateID string // token candidate
	StrategyID  string // strategy identifier
	ScenarioID  string // execution scenario

	// Entry
	EntrySignalTime  int64    // signal detection timestamp (ms)
	EntrySignalPrice float64  // price at signal
	EntryActualTime  int64    // after delay applied (ms)
	EntryActualPrice float64  // after slippage applied
	EntryLiquidity   *float64 // pool liquidity at entry (nullable)
	PositionSize     float64  // base units (default 1.0)
	PositionValue    float64  // entry_actual_price * position_size

	// Exit
	ExitSignalTime  int64   // exit trigger timestamp (ms)
	ExitSignalPrice float64 // price at exit trigger
	ExitActualTime  int64   // after delay applied (ms)
	ExitActualPrice float64 // after slippage applied
	ExitReason      string  // reason code

	// Costs
	EntryCostSOL float64 // fee + priority fee
	ExitCostSOL  float64 // fee + priority fee
	MEVCostSOL   float64 // MEV penalty
	TotalCostSOL float64 // sum of all costs
	TotalCostPct float64 // as % of position

	// Outcome
	GrossReturn  float64 // before costs
	Outcome      float64 // after costs
	OutcomeClass string  // "WIN" | "LOSS"

	// Metadata
	HoldDurationMs int64    // actual hold time (ms)
	PeakPrice      *float64 // max price during hold (for trailing stop)
	MinLiquidity   *float64 // min liquidity during hold
}

// Exit reason codes
const (
	ExitReasonTimeExit      = "TIME_EXIT"
	ExitReasonInitialStop   = "INITIAL_STOP"
	ExitReasonTrailingStop  = "TRAILING_STOP"
	ExitReasonMaxDuration   = "MAX_DURATION"
	ExitReasonLiquidityDrop = "LIQUIDITY_DROP"
)

// Outcome class constants
const (
	OutcomeClassWin  = "WIN"
	OutcomeClassLoss = "LOSS"
)
