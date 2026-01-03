package replay

import (
	"context"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

// collectingEngine collects events for verification.
type collectingEngine struct {
	events []*Event
}

func (e *collectingEngine) OnEvent(_ context.Context, event *Event) error {
	e.events = append(e.events, event)
	return nil
}

// orderValidatingEngine validates that events are received in order.
type orderValidatingEngine struct {
	lastSlot   int64
	lastTxSig  string
	lastIndex  int
	firstEvent bool
	orderError error
}

func newOrderValidatingEngine() *orderValidatingEngine {
	return &orderValidatingEngine{firstEvent: true}
}

func (e *orderValidatingEngine) OnEvent(_ context.Context, event *Event) error {
	if e.firstEvent {
		e.lastSlot = event.Slot
		e.lastTxSig = event.TxSignature
		e.lastIndex = event.EventIndex
		e.firstEvent = false
		return nil
	}

	// Check ordering
	if event.Slot < e.lastSlot {
		e.orderError = ErrInvalidOrdering
		return e.orderError
	}
	if event.Slot == e.lastSlot {
		if event.TxSignature < e.lastTxSig {
			e.orderError = ErrInvalidOrdering
			return e.orderError
		}
		if event.TxSignature == e.lastTxSig && event.EventIndex <= e.lastIndex {
			e.orderError = ErrInvalidOrdering
			return e.orderError
		}
	}

	e.lastSlot = event.Slot
	e.lastTxSig = event.TxSignature
	e.lastIndex = event.EventIndex
	return nil
}

func TestRunner_OrdersEventsDeterministically(t *testing.T) {
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
		t.Fatalf("InsertBulk swaps failed: %v", err)
	}

	runner := NewRunner(swapStore, liquidityStore)
	engine := newOrderValidatingEngine()

	err := runner.Run(ctx, "c1", 0, 10000, engine)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if engine.orderError != nil {
		t.Error("Events were not received in order")
	}
}

func TestRunner_MergesSwapsAndLiquidity(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	// Insert swaps
	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000},
	}
	if err := swapStore.InsertBulk(ctx, swaps); err != nil {
		t.Fatalf("InsertBulk swaps failed: %v", err)
	}

	// Insert liquidity events
	liquidity := []*domain.LiquidityEvent{
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
		{CandidateID: "c1", Slot: 400, TxSignature: "tx4", EventIndex: 0, Timestamp: 4000},
	}
	if err := liquidityStore.InsertBulk(ctx, liquidity); err != nil {
		t.Fatalf("InsertBulk liquidity failed: %v", err)
	}

	runner := NewRunner(swapStore, liquidityStore)
	engine := &collectingEngine{}

	err := runner.Run(ctx, "c1", 0, 10000, engine)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(engine.events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(engine.events))
	}

	// Verify order: slot 100, 200, 300, 400
	expectedSlots := []int64{100, 200, 300, 400}
	for i, slot := range expectedSlots {
		if engine.events[i].Slot != slot {
			t.Errorf("Event %d: expected slot %d, got %d", i, slot, engine.events[i].Slot)
		}
	}

	// Verify types
	if engine.events[0].Type != EventTypeSwap {
		t.Error("Event 0 should be swap")
	}
	if engine.events[1].Type != EventTypeLiquidity {
		t.Error("Event 1 should be liquidity")
	}
	if engine.events[2].Type != EventTypeSwap {
		t.Error("Event 2 should be swap")
	}
	if engine.events[3].Type != EventTypeLiquidity {
		t.Error("Event 3 should be liquidity")
	}
}

func TestRunner_Deterministic(t *testing.T) {
	// Run multiple times and verify same order
	for run := 0; run < 5; run++ {
		swapStore := memory.NewSwapStore()
		liquidityStore := memory.NewLiquidityEventStore()
		ctx := context.Background()

		swaps := []*domain.Swap{
			{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000},
			{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		}
		_ = swapStore.InsertBulk(ctx, swaps)

		liquidity := []*domain.LiquidityEvent{
			{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
		}
		_ = liquidityStore.InsertBulk(ctx, liquidity)

		runner := NewRunner(swapStore, liquidityStore)
		engine := &collectingEngine{}

		_ = runner.Run(ctx, "c1", 0, 10000, engine)

		if len(engine.events) != 3 {
			t.Fatalf("Run %d: expected 3 events", run)
		}

		// First event should always be slot 100
		if engine.events[0].Slot != 100 {
			t.Errorf("Run %d: first event should be slot 100, got %d", run, engine.events[0].Slot)
		}
	}
}

func TestRunner_Empty(t *testing.T) {
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()
	ctx := context.Background()

	runner := NewRunner(swapStore, liquidityStore)
	engine := &collectingEngine{}

	err := runner.Run(ctx, "nonexistent", 0, 10000, engine)
	if err != nil {
		t.Errorf("Empty run should not error: %v", err)
	}

	if len(engine.events) != 0 {
		t.Errorf("Expected 0 events, got %d", len(engine.events))
	}
}

func TestMergeEvents(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
		{Slot: 300, TxSignature: "tx3", EventIndex: 0},
	}

	liquidity := []*domain.LiquidityEvent{
		{Slot: 200, TxSignature: "tx2", EventIndex: 0},
	}

	events := MergeEvents(swaps, liquidity)

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Should be sorted by slot
	if events[0].Slot != 100 || events[1].Slot != 200 || events[2].Slot != 300 {
		t.Error("Events not sorted correctly")
	}
}

func TestSortEvents(t *testing.T) {
	events := []*Event{
		{Slot: 200, TxSignature: "tx2", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
	}

	SortEvents(events)

	if events[0].Slot != 100 || events[0].EventIndex != 0 {
		t.Error("First event should be (100, tx1, 0)")
	}
	if events[1].Slot != 100 || events[1].EventIndex != 1 {
		t.Error("Second event should be (100, tx1, 1)")
	}
	if events[2].Slot != 200 {
		t.Error("Third event should be slot 200")
	}
}

func TestSortEvents_TieBreaker(t *testing.T) {
	// Events with same composite key but different types
	// EventType "liquidity" < "swap" alphabetically
	events := []*Event{
		{Slot: 100, TxSignature: "tx1", EventIndex: 0, Type: EventTypeSwap},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0, Type: EventTypeLiquidity},
	}

	// Run multiple times to verify determinism (sort.Slice is not stable)
	for run := 0; run < 10; run++ {
		// Shuffle order
		if run%2 == 0 {
			events[0], events[1] = events[1], events[0]
		}

		SortEvents(events)

		// Liquidity should always come first (alphabetically: "liquidity" < "swap")
		if events[0].Type != EventTypeLiquidity {
			t.Errorf("Run %d: first event should be liquidity, got %s", run, events[0].Type)
		}
		if events[1].Type != EventTypeSwap {
			t.Errorf("Run %d: second event should be swap, got %s", run, events[1].Type)
		}
	}
}
