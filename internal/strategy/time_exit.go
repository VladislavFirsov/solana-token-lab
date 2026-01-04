package strategy

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
)

// TimeExitStrategy exits after fixed hold duration.
// Per STRATEGY_CATALOG.md: Event + Time Exit.
type TimeExitStrategy struct {
	EntryEventType string // "NEW_TOKEN" or "ACTIVE_TOKEN"
	HoldDurationMs int64  // hold duration in milliseconds
}

// NewTimeExitStrategy creates a new TimeExitStrategy.
func NewTimeExitStrategy(entryEventType string, holdDurationMs int64) *TimeExitStrategy {
	return &TimeExitStrategy{
		EntryEventType: entryEventType,
		HoldDurationMs: holdDurationMs,
	}
}

// ID returns the strategy identifier including parameters.
func (s *TimeExitStrategy) ID() string {
	return fmt.Sprintf("TIME_EXIT_%s_%dms", s.EntryEventType, s.HoldDurationMs)
}

// Execute runs the strategy on price time series.
// Per SIMULATION_SPEC.md:
//   - exit_signal_time = entry_signal_time + hold_duration_ms
//   - exit_signal_price = price_at(exit_signal_time)
//   - exit_reason = "TIME_EXIT"
func (s *TimeExitStrategy) Execute(_ context.Context, input *StrategyInput) (*domain.TradeRecord, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Calculate exit signal time
	exitSignalTime := input.EntrySignalTime + s.HoldDurationMs

	// Get exit price at target time
	exitSignalPrice, err := lookup.PriceAt(exitSignalTime, input.PriceTimeseries)
	if err != nil {
		return nil, err
	}

	// Build trade record
	return buildTradeRecord(
		input.CandidateID,
		s.ID(),
		input.Scenario.ScenarioID,
		input.EntrySignalTime,
		input.EntrySignalPrice,
		input.EntryLiquidity,
		exitSignalTime,
		exitSignalPrice,
		domain.ExitReasonTimeExit,
		input.Scenario,
		nil, // no peak price tracking
		nil, // no min liquidity tracking
	), nil
}

// Ensure TimeExitStrategy implements Strategy
var _ Strategy = (*TimeExitStrategy)(nil)
