package backtest

import (
	"context"

	"solana-token-lab/internal/replay"
)

// StubStrategy is a no-op strategy for testing.
// It collects events for verification without generating signals.
type StubStrategy struct {
	events []*replay.Event
}

// NewStubStrategy creates a new stub strategy.
func NewStubStrategy() *StubStrategy {
	return &StubStrategy{
		events: make([]*replay.Event, 0),
	}
}

// OnEvent collects events for verification.
// Always returns nil signal (no action).
func (s *StubStrategy) OnEvent(_ context.Context, event *replay.Event) (*Signal, error) {
	s.events = append(s.events, event)
	return nil, nil
}

// Name returns the strategy identifier.
func (s *StubStrategy) Name() string {
	return "stub"
}

// Events returns collected events for test verification.
func (s *StubStrategy) Events() []*replay.Event {
	return s.events
}

// Ensure StubStrategy implements Strategy
var _ Strategy = (*StubStrategy)(nil)
