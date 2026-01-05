package strategy

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
)

// TrailingStopStrategy exits when price drops from peak.
// Per STRATEGY_CATALOG.md: Event + Trailing Stop.
type TrailingStopStrategy struct {
	EntryEventType    string  // "NEW_TOKEN" or "ACTIVE_TOKEN"
	TrailPct          float64 // trailing stop percentage (e.g., 0.10 = 10%)
	InitialStopPct    float64 // initial stop loss percentage (e.g., 0.10 = 10%)
	MaxHoldDurationMs int64   // maximum hold time in milliseconds
}

// NewTrailingStopStrategy creates a new TrailingStopStrategy.
func NewTrailingStopStrategy(entryEventType string, trailPct, initialStopPct float64, maxHoldDurationMs int64) *TrailingStopStrategy {
	return &TrailingStopStrategy{
		EntryEventType:    entryEventType,
		TrailPct:          trailPct,
		InitialStopPct:    initialStopPct,
		MaxHoldDurationMs: maxHoldDurationMs,
	}
}

// ID returns the strategy identifier including parameters.
func (s *TrailingStopStrategy) ID() string {
	return fmt.Sprintf("TRAILING_STOP_%s_trail%.0f_stop%.0f_%dms",
		s.EntryEventType,
		s.TrailPct*100,
		s.InitialStopPct*100,
		s.MaxHoldDurationMs)
}

// BaseType returns the canonical base strategy type.
func (s *TrailingStopStrategy) BaseType() string {
	return domain.StrategyTypeTrailingStop
}

// Execute runs the strategy on price time series.
// Per SIMULATION_SPEC.md:
//   - initial_stop = entry_signal_price * (1 - initial_stop_pct)
//   - For each price event after entry:
//   - Update peak_price
//   - trailing_stop = peak_price * (1 - trail_pct)
//   - Check exits: INITIAL_STOP, TRAILING_STOP, MAX_DURATION
func (s *TrailingStopStrategy) Execute(_ context.Context, input *StrategyInput) (*domain.TradeRecord, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, err
	}

	entryPrice := input.EntrySignalPrice
	initialStop := entryPrice * (1 - s.InitialStopPct)
	peakPrice := entryPrice
	maxExitTime := input.EntrySignalTime + s.MaxHoldDurationMs

	var exitSignalTime int64
	var exitSignalPrice float64
	var exitReason string

	// Iterate through price events after entry
	for _, point := range input.PriceTimeseries {
		if point.TimestampMs <= input.EntrySignalTime {
			continue
		}

		t := point.TimestampMs
		price := point.Price

		// Update peak price
		if price > peakPrice {
			peakPrice = price
		}

		// Calculate trailing stop
		trailingStop := peakPrice * (1 - s.TrailPct)

		// Check exit conditions (order matters per SIMULATION_SPEC.md)
		if price <= initialStop {
			exitSignalTime = t
			exitSignalPrice = price
			exitReason = domain.ExitReasonInitialStop
			break
		}

		if price <= trailingStop {
			exitSignalTime = t
			exitSignalPrice = price
			exitReason = domain.ExitReasonTrailingStop
			break
		}

		if t-input.EntrySignalTime >= s.MaxHoldDurationMs {
			exitSignalTime = t
			exitSignalPrice = price
			exitReason = domain.ExitReasonMaxDuration
			break
		}
	}

	// If no exit triggered, use max duration
	if exitReason == "" {
		exitSignalTime = maxExitTime
		price, err := lookup.PriceAt(maxExitTime, input.PriceTimeseries)
		if err != nil {
			return nil, err
		}
		exitSignalPrice = price
		exitReason = domain.ExitReasonMaxDuration
	}

	peakPricePtr := &peakPrice

	return buildTradeRecord(
		input.CandidateID,
		s.ID(),
		input.Scenario.ScenarioID,
		input.EntrySignalTime,
		input.EntrySignalPrice,
		input.EntryLiquidity,
		exitSignalTime,
		exitSignalPrice,
		exitReason,
		input.Scenario,
		peakPricePtr,
		nil, // no min liquidity tracking
	), nil
}

// Ensure TrailingStopStrategy implements Strategy
var _ Strategy = (*TrailingStopStrategy)(nil)
