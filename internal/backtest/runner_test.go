package backtest

import (
	"context"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/storage/memory"
)

func TestRunner_CallsStrategyInOrder(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	// Insert unordered swaps
	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000},
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
	}
	if err := swapStore.InsertBulk(ctx, swaps); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	replayRunner := replay.NewRunner(swapStore, liquidityStore)
	runner := NewRunner(replayRunner)
	strategy := NewStubStrategy()

	results, err := runner.Run(ctx, "c1", 0, 10000, strategy)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify events were received in order
	events := strategy.Events()
	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Should be sorted by slot
	if events[0].Slot != 100 || events[1].Slot != 200 || events[2].Slot != 300 {
		t.Error("Strategy did not receive events in order")
	}

	// Verify results
	if results.EventCount != 3 {
		t.Errorf("Expected EventCount 3, got %d", results.EventCount)
	}
	if results.StrategyName != "stub" {
		t.Errorf("Expected StrategyName 'stub', got '%s'", results.StrategyName)
	}
}

func TestRunner_CollectsResults(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
	}
	_ = swapStore.InsertBulk(ctx, swaps)

	liquidity := []*domain.LiquidityEvent{
		{CandidateID: "c1", Slot: 150, TxSignature: "tx1.5", EventIndex: 0, Timestamp: 1500},
	}
	_ = liquidityStore.InsertBulk(ctx, liquidity)

	replayRunner := replay.NewRunner(swapStore, liquidityStore)
	runner := NewRunner(replayRunner)
	strategy := NewStubStrategy()

	results, err := runner.Run(ctx, "c1", 0, 10000, strategy)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if results.EventCount != 3 {
		t.Errorf("Expected EventCount 3, got %d", results.EventCount)
	}
	if results.CandidateID != "c1" {
		t.Errorf("Expected CandidateID 'c1', got '%s'", results.CandidateID)
	}
	if results.SignalCount != 0 {
		t.Errorf("Expected SignalCount 0 (stub strategy), got %d", results.SignalCount)
	}
}

func TestRunner_WithSignalingStrategy(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
	}
	_ = swapStore.InsertBulk(ctx, swaps)

	replayRunner := replay.NewRunner(swapStore, liquidityStore)
	runner := NewRunner(replayRunner)

	// Use a strategy that always signals
	strategy := &alwaysSignalStrategy{}

	results, err := runner.Run(ctx, "c1", 0, 10000, strategy)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if results.EventCount != 2 {
		t.Errorf("Expected EventCount 2, got %d", results.EventCount)
	}
	if results.SignalCount != 2 {
		t.Errorf("Expected SignalCount 2, got %d", results.SignalCount)
	}
	if len(results.Signals) != 2 {
		t.Errorf("Expected 2 signals, got %d", len(results.Signals))
	}
}

func TestRunner_Empty(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	replayRunner := replay.NewRunner(swapStore, liquidityStore)
	runner := NewRunner(replayRunner)
	strategy := NewStubStrategy()

	results, err := runner.Run(ctx, "nonexistent", 0, 10000, strategy)
	if err != nil {
		t.Errorf("Empty run should not error: %v", err)
	}

	if results.EventCount != 0 {
		t.Errorf("Expected EventCount 0, got %d", results.EventCount)
	}
}

// alwaysSignalStrategy is a test strategy that always returns a signal.
type alwaysSignalStrategy struct{}

func (s *alwaysSignalStrategy) OnEvent(_ context.Context, event *replay.Event) (*Signal, error) {
	return &Signal{
		Action:    ActionEnter,
		Reason:    "test",
		Timestamp: event.Timestamp,
	}, nil
}

func (s *alwaysSignalStrategy) Name() string {
	return "always_signal"
}
