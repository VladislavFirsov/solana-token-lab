package backtest

import (
	"context"

	"solana-token-lab/internal/replay"
)

// Action represents a trade action.
type Action string

// Action constants.
const (
	ActionEnter Action = "enter"
	ActionExit  Action = "exit"
)

// Signal represents a trade signal from a strategy.
type Signal struct {
	Action    Action
	Reason    string
	Timestamp int64
}

// Strategy defines hooks for backtest execution.
type Strategy interface {
	// OnEvent is called for each event in order.
	// Returns a trade signal or nil if no action.
	OnEvent(ctx context.Context, event *replay.Event) (*Signal, error)

	// Name returns the strategy identifier.
	Name() string
}

// Results holds backtest output.
type Results struct {
	StrategyName string
	CandidateID  string
	EventCount   int
	SignalCount  int
	Signals      []*Signal
}

// Engine orchestrates strategy execution during backtest.
// Implements replay.ReplayEngine.
type Engine struct {
	strategy Strategy
	results  *Results
}

// NewEngine creates a new backtest engine.
func NewEngine(strategy Strategy, candidateID string) *Engine {
	return &Engine{
		strategy: strategy,
		results: &Results{
			StrategyName: strategy.Name(),
			CandidateID:  candidateID,
			Signals:      make([]*Signal, 0),
		},
	}
}

// OnEvent processes an event through the strategy.
// Implements replay.ReplayEngine.
func (e *Engine) OnEvent(ctx context.Context, event *replay.Event) error {
	e.results.EventCount++

	signal, err := e.strategy.OnEvent(ctx, event)
	if err != nil {
		return err
	}

	if signal != nil {
		e.results.SignalCount++
		e.results.Signals = append(e.results.Signals, signal)
	}

	return nil
}

// Results returns the backtest results.
func (e *Engine) Results() *Results {
	return e.results
}

// Ensure Engine implements replay.ReplayEngine
var _ replay.ReplayEngine = (*Engine)(nil)
