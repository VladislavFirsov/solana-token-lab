package ingestion

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

// mockSwapEventSource implements a controllable swap event source for testing.
type mockSwapEventSource struct {
	ch chan *domain.SwapEvent
}

func newMockSwapEventSource() *mockSwapEventSource {
	return &mockSwapEventSource{
		ch: make(chan *domain.SwapEvent, 100),
	}
}

func (m *mockSwapEventSource) Subscribe(ctx context.Context) (<-chan *domain.SwapEvent, error) {
	return m.ch, nil
}

func (m *mockSwapEventSource) Send(event *domain.SwapEvent) {
	m.ch <- event
}

func (m *mockSwapEventSource) Close() {
	close(m.ch)
}

// mockLiquidityEventSource implements a controllable liquidity event source.
type mockLiquidityEventSource struct {
	ch chan *domain.LiquidityEvent
}

func newMockLiquidityEventSource() *mockLiquidityEventSource {
	return &mockLiquidityEventSource{
		ch: make(chan *domain.LiquidityEvent, 100),
	}
}

func (m *mockLiquidityEventSource) Subscribe(ctx context.Context) (<-chan *domain.LiquidityEvent, error) {
	return m.ch, nil
}

func (m *mockLiquidityEventSource) Send(event *domain.LiquidityEvent) {
	m.ch <- event
}

func (m *mockLiquidityEventSource) Close() {
	close(m.ch)
}

func TestRunner_SlotBasedOrdering(t *testing.T) {
	// Test that events are processed in slot order, not arrival order
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	progressStore := memory.NewDiscoveryProgressStore()

	newTokenDetector := discovery.NewDetector(candidateStore).WithProgressStore(progressStore)

	runner := NewRunner(RunnerOptions{
		SwapEventStore:   swapEventStore,
		CandidateStore:   candidateStore,
		NewTokenDetector: newTokenDetector,
		SlotLagWindow:    2,
		Logger:           log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Manually buffer events out of order
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint1", Slot: 5, TxSignature: "tx5", EventIndex: 0, Timestamp: 5000,
	})
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint2", Slot: 3, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000,
	})
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint3", Slot: 4, TxSignature: "tx4", EventIndex: 0, Timestamp: 4000,
	})

	// Trigger processing by sending a higher slot
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint4", Slot: 8, TxSignature: "tx8", EventIndex: 0, Timestamp: 8000,
	})

	// Slots 3, 4, 5 should be finalized (8 - 2 = 6, so slots <= 6 are finalized)
	// After processing, buffer should only contain slot 8
	assert.Len(t, runner.swapBuffer, 1, "Only slot 8 should remain in buffer")
	assert.Contains(t, runner.swapBuffer, int64(8))

	// Verify events were stored
	events, err := swapEventStore.GetByTimeRange(ctx, 0, 10000)
	require.NoError(t, err)
	assert.Len(t, events, 3, "3 events should have been processed")
}

func TestRunner_FlushOnShutdown(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()

	runner := NewRunner(RunnerOptions{
		SwapEventStore: swapEventStore,
		SlotLagWindow:  10, // High lag so nothing auto-processes
		Logger:         log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// Buffer some events that won't auto-finalize
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint1", Slot: 1, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000,
	})
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint2", Slot: 2, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000,
	})

	// Verify events are in buffer
	assert.Len(t, runner.swapBuffer, 2)

	// Flush all on shutdown
	runner.flushAllSlots(ctx)

	// Buffer should be empty
	assert.Empty(t, runner.swapBuffer)

	// Events should be stored
	events, err := swapEventStore.GetByTimeRange(ctx, 0, 10000)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestRunner_LateEventProcessing(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()

	runner := NewRunner(RunnerOptions{
		SwapEventStore: swapEventStore,
		SlotLagWindow:  3,
		Logger:         log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// First, advance the slot pointer
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint1", Slot: 10, TxSignature: "tx10", EventIndex: 0, Timestamp: 10000,
	})

	// Now send a "late" event for slot 5 (which is already finalized)
	// It should be processed immediately
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint2", Slot: 5, TxSignature: "tx5", EventIndex: 0, Timestamp: 5000,
	})

	// Late event should have been processed immediately
	events, err := swapEventStore.GetByTimeRange(ctx, 0, 6000)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Late event should be processed immediately")
}

func TestRunner_DeterministicOrdering(t *testing.T) {
	// Run multiple times and verify order is always the same
	for run := 0; run < 5; run++ {
		swapEventStore := memory.NewSwapEventStore()

		runner := NewRunner(RunnerOptions{
			SwapEventStore: swapEventStore,
			SlotLagWindow:  1,
			Logger:         log.New(os.Stderr, "[test] ", log.LstdFlags),
		})

		ctx := context.Background()

		// Send events in random arrival order, but same slot
		runner.bufferSwapEvent(ctx, &domain.SwapEvent{
			Mint: "mint1", Slot: 1, TxSignature: "txC", EventIndex: 0, Timestamp: 1000,
		})
		runner.bufferSwapEvent(ctx, &domain.SwapEvent{
			Mint: "mint2", Slot: 1, TxSignature: "txA", EventIndex: 0, Timestamp: 1000,
		})
		runner.bufferSwapEvent(ctx, &domain.SwapEvent{
			Mint: "mint3", Slot: 1, TxSignature: "txB", EventIndex: 0, Timestamp: 1000,
		})

		// Trigger finalization
		runner.bufferSwapEvent(ctx, &domain.SwapEvent{
			Mint: "mint4", Slot: 5, TxSignature: "tx5", EventIndex: 0, Timestamp: 5000,
		})

		events, err := swapEventStore.GetByTimeRange(ctx, 0, 2000)
		require.NoError(t, err)
		require.Len(t, events, 3)

		// Should be sorted by (tx_signature, event_index) within the slot
		assert.Equal(t, "txA", events[0].TxSignature, "Run %d: first should be txA", run)
		assert.Equal(t, "txB", events[1].TxSignature, "Run %d: second should be txB", run)
		assert.Equal(t, "txC", events[2].TxSignature, "Run %d: third should be txC", run)
	}
}

func TestRunner_LiquidityEventBuffering(t *testing.T) {
	liquidityStore := memory.NewLiquidityEventStore()

	runner := NewRunner(RunnerOptions{
		LiquidityStore: liquidityStore,
		SlotLagWindow:  2,
		Logger:         log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// Buffer liquidity events
	runner.bufferLiquidityEvent(ctx, &domain.LiquidityEvent{
		CandidateID: "c1", Slot: 3, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000,
	})
	runner.bufferLiquidityEvent(ctx, &domain.LiquidityEvent{
		CandidateID: "c1", Slot: 1, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000,
	})

	// Trigger finalization
	runner.bufferLiquidityEvent(ctx, &domain.LiquidityEvent{
		CandidateID: "c1", Slot: 5, TxSignature: "tx5", EventIndex: 0, Timestamp: 5000,
	})

	// Slots 1, 3 should be finalized (5 - 2 = 3, so slots <= 3 are finalized)
	events, err := liquidityStore.GetByCandidateID(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

func TestRunner_MixedEventTypes(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	liquidityStore := memory.NewLiquidityEventStore()

	runner := NewRunner(RunnerOptions{
		SwapEventStore: swapEventStore,
		LiquidityStore: liquidityStore,
		SlotLagWindow:  2,
		Logger:         log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// Buffer both types of events at the same slot
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint1", Slot: 3, TxSignature: "swap_tx", EventIndex: 0, Timestamp: 3000,
	})
	runner.bufferLiquidityEvent(ctx, &domain.LiquidityEvent{
		CandidateID: "c1", Slot: 3, TxSignature: "liq_tx", EventIndex: 0, Timestamp: 3000,
	})

	// Trigger finalization
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "mint2", Slot: 10, TxSignature: "tx10", EventIndex: 0, Timestamp: 10000,
	})

	// Both should be processed
	swapEvents, err := swapEventStore.GetByTimeRange(ctx, 0, 5000)
	require.NoError(t, err)
	assert.Len(t, swapEvents, 1)

	liqEvents, err := liquidityStore.GetByCandidateID(ctx, "c1")
	require.NoError(t, err)
	assert.Len(t, liqEvents, 1)
}

func TestRunner_SortInt64s(t *testing.T) {
	tests := []struct {
		name  string
		input []int64
		want  []int64
	}{
		{"empty", []int64{}, []int64{}},
		{"single", []int64{5}, []int64{5}},
		{"already_sorted", []int64{1, 2, 3}, []int64{1, 2, 3}},
		{"reverse", []int64{3, 2, 1}, []int64{1, 2, 3}},
		{"random", []int64{5, 1, 3, 2, 4}, []int64{1, 2, 3, 4, 5}},
		{"duplicates", []int64{3, 1, 3, 2, 1}, []int64{1, 1, 2, 3, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortInt64s(tt.input)
			assert.Equal(t, tt.want, tt.input)
		})
	}
}

func TestRunner_NewTokenDiscovery(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	progressStore := memory.NewDiscoveryProgressStore()

	newTokenDetector := discovery.NewDetector(candidateStore).WithProgressStore(progressStore)

	runner := NewRunner(RunnerOptions{
		SwapEventStore:   swapEventStore,
		CandidateStore:   candidateStore,
		NewTokenDetector: newTokenDetector,
		SlotLagWindow:    1,
		Logger:           log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// Send first swap for a new mint
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "new_mint_xyz", Slot: 1, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000,
	})

	// Trigger finalization
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "other_mint", Slot: 5, TxSignature: "tx5", EventIndex: 0, Timestamp: 5000,
	})

	// Verify candidate was discovered
	candidates, err := candidateStore.GetBySource(ctx, domain.SourceNewToken)
	require.NoError(t, err)
	assert.Len(t, candidates, 1)
	assert.Equal(t, "new_mint_xyz", candidates[0].Mint)
}

func TestRunner_NoDuplicateCandidates(t *testing.T) {
	swapEventStore := memory.NewSwapEventStore()
	candidateStore := memory.NewCandidateStore()
	progressStore := memory.NewDiscoveryProgressStore()

	newTokenDetector := discovery.NewDetector(candidateStore).WithProgressStore(progressStore)

	runner := NewRunner(RunnerOptions{
		SwapEventStore:   swapEventStore,
		CandidateStore:   candidateStore,
		NewTokenDetector: newTokenDetector,
		SlotLagWindow:    1,
		Logger:           log.New(os.Stderr, "[test] ", log.LstdFlags),
	})

	ctx := context.Background()

	// Send multiple swaps for the same mint
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "same_mint", Slot: 1, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000,
	})
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "same_mint", Slot: 2, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000,
	})
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "same_mint", Slot: 3, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000,
	})

	// Trigger finalization
	runner.bufferSwapEvent(ctx, &domain.SwapEvent{
		Mint: "trigger", Slot: 10, TxSignature: "tx10", EventIndex: 0, Timestamp: 10000,
	})

	// Only one candidate should be created
	candidates, err := candidateStore.GetByMint(ctx, "same_mint")
	require.NoError(t, err)
	assert.Len(t, candidates, 1, "Should only have one candidate per mint")
}

func TestRunner_DefaultValues(t *testing.T) {
	runner := NewRunner(RunnerOptions{})

	assert.Equal(t, 1*time.Hour, runner.checkInterval, "Default check interval should be 1 hour")
	assert.Equal(t, int64(5), runner.slotLagWindow, "Default slot lag window should be 5")
	assert.NotNil(t, runner.logger, "Logger should not be nil")
}
