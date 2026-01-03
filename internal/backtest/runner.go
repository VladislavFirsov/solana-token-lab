package backtest

import (
	"context"

	"solana-token-lab/internal/replay"
)

// Runner executes backtest with strategy hooks.
type Runner struct {
	replayRunner *replay.Runner
}

// NewRunner creates a new backtest runner.
func NewRunner(replayRunner *replay.Runner) *Runner {
	return &Runner{
		replayRunner: replayRunner,
	}
}

// Run executes backtest for a candidate within time range.
// Returns backtest results after processing all events.
func (r *Runner) Run(ctx context.Context, candidateID string, from, to int64, strategy Strategy) (*Results, error) {
	engine := NewEngine(strategy, candidateID)

	if err := r.replayRunner.Run(ctx, candidateID, from, to, engine); err != nil {
		return nil, err
	}

	return engine.Results(), nil
}

// RunAll executes backtest for all events of a candidate.
func (r *Runner) RunAll(ctx context.Context, candidateID string, strategy Strategy) (*Results, error) {
	engine := NewEngine(strategy, candidateID)

	if err := r.replayRunner.RunAll(ctx, candidateID, engine); err != nil {
		return nil, err
	}

	return engine.Results(), nil
}
