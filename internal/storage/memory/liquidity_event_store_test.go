package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestLiquidityEventStore_InsertAndGet(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	event := &domain.LiquidityEvent{
		ID:             1,
		CandidateID:    "cand1",
		TxSignature:    "sig1",
		EventIndex:     0,
		Slot:           100,
		Timestamp:      1704067200000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1000.0,
	}

	err := store.Insert(ctx, event)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	result, err := store.GetByCandidateID(ctx, "cand1")
	if err != nil {
		t.Fatalf("GetByCandidateID failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 event, got %d", len(result))
	}

	if result[0].EventType != domain.LiquidityEventAdd {
		t.Errorf("EventType mismatch: got %s, want %s", result[0].EventType, domain.LiquidityEventAdd)
	}
}

func TestLiquidityEventStore_DuplicateKey(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	event := &domain.LiquidityEvent{
		CandidateID: "cand1",
		TxSignature: "sig1",
		EventIndex:  0,
		Timestamp:   1000,
	}

	if err := store.Insert(ctx, event); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.Insert(ctx, event)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestLiquidityEventStore_InsertBulk(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 1, Timestamp: 1001},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 1002},
	}

	err := store.InsertBulk(ctx, events)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 3 {
		t.Errorf("Expected 3 events, got %d", len(result))
	}
}

func TestLiquidityEventStore_InsertBulkIntraBatchDuplicate(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	// Batch with duplicate within itself
	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000}, // duplicate
	}

	err := store.InsertBulk(ctx, events)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey for intra-batch duplicate, got %v", err)
	}

	// Verify nothing was inserted
	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 0 {
		t.Errorf("Expected 0 events (rollback), got %d", len(result))
	}
}

func TestLiquidityEventStore_InsertBulkExistingDuplicate(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	first := &domain.LiquidityEvent{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000}
	if err := store.Insert(ctx, first); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 1, Timestamp: 1001},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000}, // duplicate
	}

	err := store.InsertBulk(ctx, events)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 1 {
		t.Errorf("Expected 1 event (no partial insert), got %d", len(result))
	}
}

func TestLiquidityEventStore_GetByTimeRange(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 2000},
		{CandidateID: "c1", TxSignature: "s3", EventIndex: 0, Timestamp: 3000},
		{CandidateID: "c2", TxSignature: "s4", EventIndex: 0, Timestamp: 2500}, // different candidate
	}

	if err := store.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByTimeRange(ctx, "c1", 1500, 2500)
	if err != nil {
		t.Fatalf("GetByTimeRange failed: %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 event in range, got %d", len(result))
	}

	if result[0].Timestamp != 2000 {
		t.Errorf("Expected timestamp 2000, got %d", result[0].Timestamp)
	}
}

func TestLiquidityEventStore_OrderByTimestamp(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", TxSignature: "s3", EventIndex: 0, Timestamp: 3000},
		{CandidateID: "c1", TxSignature: "s1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", TxSignature: "s2", EventIndex: 0, Timestamp: 2000},
	}

	if err := store.InsertBulk(ctx, events); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")

	for i := 1; i < len(result); i++ {
		if result[i].Timestamp < result[i-1].Timestamp {
			t.Errorf("Results not ordered: %d < %d", result[i].Timestamp, result[i-1].Timestamp)
		}
	}
}

func TestLiquidityEventStore_InvalidInput(t *testing.T) {
	store := NewLiquidityEventStore()
	ctx := context.Background()

	err := store.Insert(ctx, nil)
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil, got %v", err)
	}

	err = store.Insert(ctx, &domain.LiquidityEvent{CandidateID: ""})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty CandidateID, got %v", err)
	}
}
