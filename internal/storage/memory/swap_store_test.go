package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestSwapStore_InsertAndGet(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	swap := &domain.Swap{
		ID:          1,
		CandidateID: "cand1",
		TxSignature: "sig1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1704067200000,
		Side:        domain.SwapSideBuy,
		AmountIn:    1.0,
		AmountOut:   100.0,
		Price:       100.0,
	}

	err := store.Insert(ctx, swap)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	result, err := store.GetByCandidateID(ctx, "cand1")
	if err != nil {
		t.Fatalf("GetByCandidateID failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 swap, got %d", len(result))
	}

	if result[0].Price != 100.0 {
		t.Errorf("Price mismatch: got %f, want %f", result[0].Price, 100.0)
	}
}

func TestSwapStore_DuplicateKey(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	swap := &domain.Swap{
		CandidateID: "cand1",
		TxSignature: "sig1",
		EventIndex:  0,
		Timestamp:   1000,
	}

	if err := store.Insert(ctx, swap); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.Insert(ctx, swap)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestSwapStore_InsertBulk(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	swaps := []*domain.Swap{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 1, Timestamp: 1001},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 1002},
	}

	err := store.InsertBulk(ctx, swaps)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 3 {
		t.Errorf("Expected 3 swaps, got %d", len(result))
	}
}

func TestSwapStore_InsertBulkPartialDuplicate(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	// Insert first
	first := &domain.Swap{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000}
	if err := store.Insert(ctx, first); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Bulk insert with duplicate
	swaps := []*domain.Swap{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 1, Timestamp: 1001}, // new
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000}, // duplicate
	}

	err := store.InsertBulk(ctx, swaps)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}

	// Verify no partial insert
	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 1 {
		t.Errorf("Expected 1 swap (rollback), got %d", len(result))
	}
}

func TestSwapStore_GetByTimeRange(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	swaps := []*domain.Swap{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000, Slot: 1},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 2000, Slot: 2},
		{CandidateID: "c1", TxSignature: "s3", EventIndex: 0, Timestamp: 3000, Slot: 3},
		{CandidateID: "c2", TxSignature: "s4", EventIndex: 0, Timestamp: 2500, Slot: 4}, // different candidate
	}

	if err := store.InsertBulk(ctx, swaps); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByTimeRange(ctx, "c1", 1500, 2500)
	if err != nil {
		t.Fatalf("GetByTimeRange failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 swap in range, got %d", len(result))
	}

	if result[0].Timestamp != 2000 {
		t.Errorf("Expected timestamp 2000, got %d", result[0].Timestamp)
	}
}

func TestSwapStore_OrderByTimestamp(t *testing.T) {
	store := NewSwapStore()
	ctx := context.Background()

	// Insert in random order
	swaps := []*domain.Swap{
		{CandidateID: "c1", TxSignature: "s3", EventIndex: 0, Timestamp: 3000, Slot: 3},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000, Slot: 1},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 2000, Slot: 2},
	}

	if err := store.InsertBulk(ctx, swaps); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")

	// Should be ordered by timestamp ASC
	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("Results not ordered: %d < %d", result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}
