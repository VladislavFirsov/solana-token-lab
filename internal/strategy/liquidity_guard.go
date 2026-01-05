package strategy

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
)

// ErrNoEntryLiquidity is returned when entry liquidity cannot be determined.
var ErrNoEntryLiquidity = errors.New("entry liquidity cannot be determined")

// LiquidityGuardStrategy exits when liquidity drops below threshold.
// Per STRATEGY_CATALOG.md: Event + Liquidity Guard.
type LiquidityGuardStrategy struct {
	EntryEventType    string  // "NEW_TOKEN" or "ACTIVE_TOKEN"
	LiquidityDropPct  float64 // liquidity drop threshold (e.g., 0.30 = 30% drop)
	MaxHoldDurationMs int64   // maximum hold time in milliseconds
}

// NewLiquidityGuardStrategy creates a new LiquidityGuardStrategy.
func NewLiquidityGuardStrategy(entryEventType string, liquidityDropPct float64, maxHoldDurationMs int64) *LiquidityGuardStrategy {
	return &LiquidityGuardStrategy{
		EntryEventType:    entryEventType,
		LiquidityDropPct:  liquidityDropPct,
		MaxHoldDurationMs: maxHoldDurationMs,
	}
}

// ID returns the strategy identifier including parameters.
func (s *LiquidityGuardStrategy) ID() string {
	return fmt.Sprintf("LIQUIDITY_GUARD_%s_drop%.0f_%dms",
		s.EntryEventType,
		s.LiquidityDropPct*100,
		s.MaxHoldDurationMs)
}

// BaseType returns the canonical base strategy type.
func (s *LiquidityGuardStrategy) BaseType() string {
	return domain.StrategyTypeLiquidityGuard
}

// mergedEvent represents a unified event for iteration.
type mergedEvent struct {
	TimestampMs int64
	Slot        int64
	IsPrice     bool // true = price event, false = liquidity event
}

// mergePriceAndLiquidity merges price and liquidity events ordered by (timestamp_ms, slot).
// Per REPLAY_PROTOCOL.md: price events before liquidity events when same timestamp/slot.
func mergePriceAndLiquidity(prices []*domain.PriceTimeseriesPoint, liq []*domain.LiquidityTimeseriesPoint) []mergedEvent {
	var events []mergedEvent

	for _, p := range prices {
		events = append(events, mergedEvent{
			TimestampMs: p.TimestampMs,
			Slot:        p.Slot,
			IsPrice:     true,
		})
	}

	for _, l := range liq {
		events = append(events, mergedEvent{
			TimestampMs: l.TimestampMs,
			Slot:        l.Slot,
			IsPrice:     false,
		})
	}

	// Sort by (timestamp_ms ASC, slot ASC, price before liquidity)
	sort.Slice(events, func(i, j int) bool {
		if events[i].TimestampMs != events[j].TimestampMs {
			return events[i].TimestampMs < events[j].TimestampMs
		}
		if events[i].Slot != events[j].Slot {
			return events[i].Slot < events[j].Slot
		}
		// Price events before liquidity events (deterministic tie-breaker)
		return events[i].IsPrice && !events[j].IsPrice
	})

	return events
}

// Execute runs the strategy on price and liquidity time series.
// Per SIMULATION_SPEC.md and REPLAY_PROTOCOL.md:
//   - liquidity_threshold = entry_liquidity * (1 - liquidity_drop_pct)
//   - Iterate merged events (price + liquidity) ordered by (timestamp_ms, slot)
//   - At each event: compute liquidity_at and price_at, check exits
func (s *LiquidityGuardStrategy) Execute(_ context.Context, input *StrategyInput) (*domain.TradeRecord, error) {
	// Validate input
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// Get entry liquidity
	var entryLiquidityPtr *float64
	if input.EntryLiquidity != nil {
		entryLiquidityPtr = input.EntryLiquidity
	} else {
		// Try to get from timeseries per REPLAY_PROTOCOL.md
		liq, err := lookup.LiquidityAt(input.EntrySignalTime, input.LiquidityTimeseries)
		if err != nil {
			return nil, err
		}
		entryLiquidityPtr = liq
	}

	// If entry liquidity cannot be determined, return error
	if entryLiquidityPtr == nil {
		return nil, ErrNoEntryLiquidity
	}

	entryLiquidity := *entryLiquidityPtr
	liquidityThreshold := entryLiquidity * (1 - s.LiquidityDropPct)
	minLiquidity := entryLiquidity

	var exitSignalTime int64
	var exitSignalPrice float64
	var exitReason string

	// Merge and sort events per REPLAY_PROTOCOL.md
	mergedEvents := mergePriceAndLiquidity(input.PriceTimeseries, input.LiquidityTimeseries)

	// Iterate through merged events after entry
	for _, event := range mergedEvents {
		if event.TimestampMs <= input.EntrySignalTime {
			continue
		}

		t := event.TimestampMs

		// Get current values using lookup functions
		currentLiqPtr, err := lookup.LiquidityAt(t, input.LiquidityTimeseries)
		if err != nil {
			return nil, err
		}
		currentPrice, err := lookup.PriceAt(t, input.PriceTimeseries)
		if err != nil {
			return nil, err
		}

		// Track min liquidity if available
		if currentLiqPtr != nil && *currentLiqPtr < minLiquidity {
			minLiquidity = *currentLiqPtr
		}

		// Check liquidity drop (only if we have liquidity data)
		if currentLiqPtr != nil && *currentLiqPtr < liquidityThreshold {
			exitSignalTime = t
			exitSignalPrice = currentPrice
			exitReason = domain.ExitReasonLiquidityDrop
			break
		}

		// Check max duration
		if t-input.EntrySignalTime >= s.MaxHoldDurationMs {
			exitSignalTime = t
			exitSignalPrice = currentPrice
			exitReason = domain.ExitReasonMaxDuration
			break
		}
	}

	// If no exit triggered, use max duration at calculated time
	if exitReason == "" {
		maxExitTime := input.EntrySignalTime + s.MaxHoldDurationMs
		exitSignalTime = maxExitTime
		price, err := lookup.PriceAt(maxExitTime, input.PriceTimeseries)
		if err != nil {
			return nil, err
		}
		exitSignalPrice = price
		exitReason = domain.ExitReasonMaxDuration
	}

	minLiquidityPtr := &minLiquidity

	return buildTradeRecord(
		input.CandidateID,
		s.ID(),
		input.Scenario.ScenarioID,
		input.EntrySignalTime,
		input.EntrySignalPrice,
		entryLiquidityPtr,
		exitSignalTime,
		exitSignalPrice,
		exitReason,
		input.Scenario,
		nil, // no peak price tracking
		minLiquidityPtr,
	), nil
}

// Ensure LiquidityGuardStrategy implements Strategy
var _ Strategy = (*LiquidityGuardStrategy)(nil)
