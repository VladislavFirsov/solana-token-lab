package replay

import (
	"context"

	"solana-token-lab/internal/storage"
)

// Runner loads events from storage and replays them in deterministic order.
type Runner struct {
	swapStore      storage.SwapStore
	liquidityStore storage.LiquidityEventStore
}

// NewRunner creates a new replay runner.
func NewRunner(swapStore storage.SwapStore, liquidityStore storage.LiquidityEventStore) *Runner {
	return &Runner{
		swapStore:      swapStore,
		liquidityStore: liquidityStore,
	}
}

// Run loads events for a candidate within time range and replays them through the engine.
// Events are ordered by (slot, tx_signature, event_index) before replay.
func (r *Runner) Run(ctx context.Context, candidateID string, from, to int64, engine ReplayEngine) error {
	// Load swaps
	swapData, err := r.swapStore.GetByTimeRange(ctx, candidateID, from, to)
	if err != nil {
		return err
	}

	// Load liquidity events
	liquidityData, err := r.liquidityStore.GetByTimeRange(ctx, candidateID, from, to)
	if err != nil {
		return err
	}

	// Merge and sort events
	events := MergeEvents(swapData, liquidityData)

	// Replay through engine
	for _, event := range events {
		if err := engine.OnEvent(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// RunAll loads all events for a candidate and replays them through the engine.
func (r *Runner) RunAll(ctx context.Context, candidateID string, engine ReplayEngine) error {
	// Load all swaps
	swapData, err := r.swapStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return err
	}

	// Load all liquidity events
	liquidityData, err := r.liquidityStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return err
	}

	// Merge and sort events
	events := MergeEvents(swapData, liquidityData)

	// Replay through engine
	for _, event := range events {
		if err := engine.OnEvent(ctx, event); err != nil {
			return err
		}
	}

	return nil
}
