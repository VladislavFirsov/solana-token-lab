package ingestion

import (
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
)

func TestSortSwaps(t *testing.T) {
	// Intentionally unordered swaps
	swaps := []*domain.Swap{
		{Slot: 200, TxSignature: "tx2", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx2", EventIndex: 0},
		{Slot: 300, TxSignature: "tx1", EventIndex: 0},
	}

	SortSwaps(swaps)

	// Verify order: (slot ASC, tx_signature ASC, event_index ASC)
	expected := []struct {
		slot       int64
		txSig      string
		eventIndex int
	}{
		{100, "tx1", 0},
		{100, "tx1", 1},
		{100, "tx2", 0},
		{200, "tx2", 0},
		{300, "tx1", 0},
	}

	for i, exp := range expected {
		if swaps[i].Slot != exp.slot || swaps[i].TxSignature != exp.txSig || swaps[i].EventIndex != exp.eventIndex {
			t.Errorf("Index %d: got (%d, %s, %d), want (%d, %s, %d)",
				i, swaps[i].Slot, swaps[i].TxSignature, swaps[i].EventIndex,
				exp.slot, exp.txSig, exp.eventIndex)
		}
	}
}

func TestSortSwaps_Empty(t *testing.T) {
	var swaps []*domain.Swap
	SortSwaps(swaps) // Should not panic
}

func TestSortSwaps_SingleElement(t *testing.T) {
	swaps := []*domain.Swap{{Slot: 100, TxSignature: "tx1", EventIndex: 0}}
	SortSwaps(swaps)
	if swaps[0].Slot != 100 {
		t.Error("Single element should remain unchanged")
	}
}

func TestSortLiquidityEvents(t *testing.T) {
	events := []*domain.LiquidityEvent{
		{Slot: 200, TxSignature: "tx2", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
	}

	SortLiquidityEvents(events)

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

func TestValidateSwapOrdering_Valid(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx2", EventIndex: 0},
		{Slot: 200, TxSignature: "tx1", EventIndex: 0},
	}

	err := ValidateSwapOrdering(swaps)
	if err != nil {
		t.Errorf("Valid ordering should pass, got error: %v", err)
	}
}

func TestValidateSwapOrdering_Invalid_Slot(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 200, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0}, // slot goes backwards
	}

	err := ValidateSwapOrdering(swaps)
	if !errors.Is(err, ErrInvalidOrdering) {
		t.Errorf("Expected ErrInvalidOrdering, got %v", err)
	}
}

func TestValidateSwapOrdering_Invalid_TxSignature(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 100, TxSignature: "tx2", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0}, // tx_signature not ascending
	}

	err := ValidateSwapOrdering(swaps)
	if !errors.Is(err, ErrInvalidOrdering) {
		t.Errorf("Expected ErrInvalidOrdering, got %v", err)
	}
}

func TestValidateSwapOrdering_Invalid_EventIndex(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0}, // event_index goes backwards
	}

	err := ValidateSwapOrdering(swaps)
	if !errors.Is(err, ErrInvalidOrdering) {
		t.Errorf("Expected ErrInvalidOrdering, got %v", err)
	}
}

func TestValidateSwapOrdering_Invalid_Duplicate(t *testing.T) {
	swaps := []*domain.Swap{
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0}, // duplicate
	}

	err := ValidateSwapOrdering(swaps)
	if !errors.Is(err, ErrInvalidOrdering) {
		t.Errorf("Expected ErrInvalidOrdering for duplicates, got %v", err)
	}
}

func TestValidateSwapOrdering_Empty(t *testing.T) {
	err := ValidateSwapOrdering(nil)
	if err != nil {
		t.Errorf("Empty slice should be valid, got %v", err)
	}
}

func TestValidateLiquidityEventOrdering_Valid(t *testing.T) {
	events := []*domain.LiquidityEvent{
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 200, TxSignature: "tx1", EventIndex: 0},
	}

	err := ValidateLiquidityEventOrdering(events)
	if err != nil {
		t.Errorf("Valid ordering should pass, got error: %v", err)
	}
}

func TestValidateLiquidityEventOrdering_Invalid(t *testing.T) {
	events := []*domain.LiquidityEvent{
		{Slot: 200, TxSignature: "tx1", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
	}

	err := ValidateLiquidityEventOrdering(events)
	if !errors.Is(err, ErrInvalidOrdering) {
		t.Errorf("Expected ErrInvalidOrdering, got %v", err)
	}
}

func TestSortSwaps_Deterministic(t *testing.T) {
	// Run sorting multiple times and verify same result
	for run := 0; run < 10; run++ {
		swaps := []*domain.Swap{
			{Slot: 300, TxSignature: "tx3", EventIndex: 0},
			{Slot: 100, TxSignature: "tx1", EventIndex: 0},
			{Slot: 200, TxSignature: "tx2", EventIndex: 0},
		}

		SortSwaps(swaps)

		if swaps[0].Slot != 100 || swaps[1].Slot != 200 || swaps[2].Slot != 300 {
			t.Errorf("Run %d: sorting not deterministic", run)
		}
	}
}
