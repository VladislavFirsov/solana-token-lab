package ingestion

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/ingestion/stub"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
)

// orderValidatingSwapStore wraps a SwapStore and validates ordering in InsertBulk.
// Returns ErrInvalidOrdering if swaps are not properly ordered.
type orderValidatingSwapStore struct {
	storage.SwapStore
}

func (s *orderValidatingSwapStore) InsertBulk(ctx context.Context, swaps []*domain.Swap) error {
	if err := ValidateSwapOrdering(swaps); err != nil {
		return err
	}
	return s.SwapStore.InsertBulk(ctx, swaps)
}

// orderValidatingLiquidityStore wraps a LiquidityEventStore and validates ordering in InsertBulk.
type orderValidatingLiquidityStore struct {
	storage.LiquidityEventStore
}

func (s *orderValidatingLiquidityStore) InsertBulk(ctx context.Context, events []*domain.LiquidityEvent) error {
	if err := ValidateLiquidityEventOrdering(events); err != nil {
		return err
	}
	return s.LiquidityEventStore.InsertBulk(ctx, events)
}

func TestManager_IngestSwaps_Ordering(t *testing.T) {
	// Create unordered swaps (slot order differs from timestamp order)
	// Manager must sort these before InsertBulk, otherwise validating store fails
	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 1000}, // slot 300, ts 1000
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 3000}, // slot 100, ts 3000
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000}, // slot 200, ts 2000
	}

	source := stub.NewStubSwapSource(swaps)
	// Use validating store that returns error if InsertBulk receives unordered data
	store := &orderValidatingSwapStore{SwapStore: memory.NewSwapStore()}

	mgr := NewManager(ManagerOptions{
		SwapSource: source,
		SwapStore:  store,
	})

	ctx := context.Background()
	count, err := mgr.IngestSwaps(ctx, "c1", 0, 10000)
	if err != nil {
		t.Fatalf("IngestSwaps failed: %v (Manager must sort before InsertBulk)", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 swaps ingested, got %d", count)
	}
}

func TestManager_IngestSwaps_DuplicateRejection(t *testing.T) {
	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
	}

	source := stub.NewStubSwapSource(swaps)
	store := memory.NewSwapStore()

	mgr := NewManager(ManagerOptions{
		SwapSource: source,
		SwapStore:  store,
	})

	ctx := context.Background()

	// First ingest succeeds
	count, err := mgr.IngestSwaps(ctx, "c1", 0, 10000)
	if err != nil {
		t.Fatalf("First ingest failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 swap, got %d", count)
	}

	// Second ingest with same data fails (duplicate)
	_, err = mgr.IngestSwaps(ctx, "c1", 0, 10000)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey on duplicate, got %v", err)
	}
}

func TestManager_IngestSwaps_Deterministic(t *testing.T) {
	// Run multiple times and verify Manager always sorts (deterministic ordering)
	for run := 0; run < 5; run++ {
		swaps := []*domain.Swap{
			{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 1000},
			{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 3000},
			{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
		}

		source := stub.NewStubSwapSource(swaps)
		// Use validating store - if Manager doesn't sort, test fails
		store := &orderValidatingSwapStore{SwapStore: memory.NewSwapStore()}

		mgr := NewManager(ManagerOptions{
			SwapSource: source,
			SwapStore:  store,
		})

		ctx := context.Background()
		count, err := mgr.IngestSwaps(ctx, "c1", 0, 10000)
		if err != nil {
			t.Fatalf("Run %d: IngestSwaps failed: %v", run, err)
		}
		if count != 3 {
			t.Errorf("Run %d: Expected 3, got %d", run, count)
		}
	}
}

func TestManager_IngestSwaps_Empty(t *testing.T) {
	source := stub.NewStubSwapSource(nil)
	store := memory.NewSwapStore()

	mgr := NewManager(ManagerOptions{
		SwapSource: source,
		SwapStore:  store,
	})

	ctx := context.Background()
	count, err := mgr.IngestSwaps(ctx, "c1", 0, 10000)
	if err != nil {
		t.Errorf("Empty source should not error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 swaps, got %d", count)
	}
}

func TestManager_IngestSwaps_FilterByTimeRange(t *testing.T) {
	swaps := []*domain.Swap{
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 1000},
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000},
		{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 3000},
	}

	source := stub.NewStubSwapSource(swaps)
	store := memory.NewSwapStore()

	mgr := NewManager(ManagerOptions{
		SwapSource: source,
		SwapStore:  store,
	})

	ctx := context.Background()
	count, err := mgr.IngestSwaps(ctx, "c1", 1500, 2500)
	if err != nil {
		t.Fatalf("IngestSwaps failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 swap in time range, got %d", count)
	}
}

func TestManager_IngestLiquidityEvents_Ordering(t *testing.T) {
	// Create unordered events (slot order differs from timestamp order)
	// Manager must sort these before InsertBulk, otherwise validating store fails
	events := []*domain.LiquidityEvent{
		{CandidateID: "c1", Slot: 300, TxSignature: "tx3", EventIndex: 0, Timestamp: 1000}, // slot 300, ts 1000
		{CandidateID: "c1", Slot: 100, TxSignature: "tx1", EventIndex: 0, Timestamp: 3000}, // slot 100, ts 3000
		{CandidateID: "c1", Slot: 200, TxSignature: "tx2", EventIndex: 0, Timestamp: 2000}, // slot 200, ts 2000
	}

	source := stub.NewStubLiquidityEventSource(events)
	// Use validating store that returns error if InsertBulk receives unordered data
	store := &orderValidatingLiquidityStore{LiquidityEventStore: memory.NewLiquidityEventStore()}

	mgr := NewManager(ManagerOptions{
		LiquiditySource: source,
		LiquidityStore:  store,
	})

	ctx := context.Background()
	count, err := mgr.IngestLiquidityEvents(ctx, "c1", 0, 10000)
	if err != nil {
		t.Fatalf("IngestLiquidityEvents failed: %v (Manager must sort before InsertBulk)", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 events, got %d", count)
	}
}

func TestManager_IngestMetadata(t *testing.T) {
	name := "TestToken"
	symbol := "TT"
	metadata := []*domain.TokenMetadata{
		{Mint: "mint1", Name: &name, Symbol: &symbol, Decimals: 9},
	}

	source := stub.NewStubMetadataSource(metadata)
	store := memory.NewTokenMetadataStore()

	mgr := NewManager(ManagerOptions{
		MetadataSource: source,
		MetadataStore:  store,
	})

	ctx := context.Background()
	err := mgr.IngestMetadata(ctx, "cand1", "mint1")
	if err != nil {
		t.Fatalf("IngestMetadata failed: %v", err)
	}

	result, err := store.GetByID(ctx, "cand1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if result.Mint != "mint1" {
		t.Errorf("Mint mismatch: got %s, want mint1", result.Mint)
	}
	if *result.Name != "TestToken" {
		t.Errorf("Name mismatch: got %s, want TestToken", *result.Name)
	}
}

func TestManager_IngestMetadata_DuplicateRejection(t *testing.T) {
	metadata := []*domain.TokenMetadata{
		{Mint: "mint1", Decimals: 9},
	}

	source := stub.NewStubMetadataSource(metadata)
	store := memory.NewTokenMetadataStore()

	mgr := NewManager(ManagerOptions{
		MetadataSource: source,
		MetadataStore:  store,
	})

	ctx := context.Background()

	// First ingest succeeds
	err := mgr.IngestMetadata(ctx, "cand1", "mint1")
	if err != nil {
		t.Fatalf("First ingest failed: %v", err)
	}

	// Second ingest with same candidate fails
	err = mgr.IngestMetadata(ctx, "cand1", "mint1")
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestManager_IngestMetadata_NotFound(t *testing.T) {
	source := stub.NewStubMetadataSource(nil)
	store := memory.NewTokenMetadataStore()

	mgr := NewManager(ManagerOptions{
		MetadataSource: source,
		MetadataStore:  store,
	})

	ctx := context.Background()
	err := mgr.IngestMetadata(ctx, "cand1", "nonexistent")
	if err != nil {
		t.Errorf("Missing metadata should not error: %v", err)
	}
}

func TestManager_NilSources(t *testing.T) {
	mgr := NewManager(ManagerOptions{})

	ctx := context.Background()

	count, err := mgr.IngestSwaps(ctx, "c1", 0, 1000)
	if err != nil || count != 0 {
		t.Error("Nil swap source should return 0, nil")
	}

	count, err = mgr.IngestLiquidityEvents(ctx, "c1", 0, 1000)
	if err != nil || count != 0 {
		t.Error("Nil liquidity source should return 0, nil")
	}

	err = mgr.IngestMetadata(ctx, "c1", "mint1")
	if err != nil {
		t.Error("Nil metadata source should return nil")
	}
}
