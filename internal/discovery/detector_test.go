package discovery

import (
	"context"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
)

func TestDetector_DeterministicID(t *testing.T) {
	store := memory.NewCandidateStore()
	detector := NewDetector(store)
	ctx := context.Background()

	pool := "PoolAddr123"
	event := &SwapEvent{
		Mint:        "MintAddr456",
		Pool:        &pool,
		TxSignature: "TxSig789",
		EventIndex:  0,
		Slot:        12345678,
		Timestamp:   1704067200000,
	}

	// Process same event multiple times (with reset)
	candidate1, err := detector.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if candidate1 == nil {
		t.Fatal("Expected candidate to be created")
	}

	// Compute expected ID
	expectedID := idhash.ComputeCandidateID(
		event.Mint,
		event.Pool,
		domain.SourceNewToken,
		event.TxSignature,
		event.EventIndex,
		event.Slot,
	)

	if candidate1.CandidateID != expectedID {
		t.Errorf("CandidateID mismatch: expected %s, got %s", expectedID, candidate1.CandidateID)
	}

	// Reset and try with fresh store — should produce same ID
	store2 := memory.NewCandidateStore()
	detector2 := NewDetector(store2)

	candidate2, err := detector2.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent (2) failed: %v", err)
	}
	if candidate2 == nil {
		t.Fatal("Expected candidate to be created (2)")
	}

	if candidate1.CandidateID != candidate2.CandidateID {
		t.Error("Same input should produce same candidate_id")
	}
}

func TestDetector_FirstSwapOnly(t *testing.T) {
	store := memory.NewCandidateStore()
	detector := NewDetector(store)
	ctx := context.Background()

	// Two swaps for same mint
	event1 := &SwapEvent{
		Mint:        "MintAddr456",
		TxSignature: "TxSig1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
	}
	event2 := &SwapEvent{
		Mint:        "MintAddr456", // Same mint
		TxSignature: "TxSig2",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   2000,
	}

	// First swap should create candidate
	candidate1, err := detector.ProcessEvent(ctx, event1)
	if err != nil {
		t.Fatalf("ProcessEvent (1) failed: %v", err)
	}
	if candidate1 == nil {
		t.Fatal("First swap should create candidate")
	}

	// Second swap for same mint should NOT create candidate
	candidate2, err := detector.ProcessEvent(ctx, event2)
	if err != nil {
		t.Fatalf("ProcessEvent (2) failed: %v", err)
	}
	if candidate2 != nil {
		t.Error("Second swap for same mint should not create candidate")
	}

	// Verify only one candidate in store
	candidates, _ := store.GetByMint(ctx, "MintAddr456")
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate in store, got %d", len(candidates))
	}
}

func TestDetector_NoDuplicates_GetByMint(t *testing.T) {
	store := memory.NewCandidateStore()
	ctx := context.Background()

	// Pre-insert a candidate
	existingCandidate := &domain.TokenCandidate{
		CandidateID:  "existing123",
		Source:       domain.SourceNewToken,
		Mint:         "ExistingMint",
		TxSignature:  "TxExisting",
		Slot:         50,
		DiscoveredAt: 500,
	}
	_ = store.Insert(ctx, existingCandidate)

	// Create detector after candidate exists
	detector := NewDetector(store)

	// Try to process swap for existing mint
	event := &SwapEvent{
		Mint:        "ExistingMint", // Already exists
		TxSignature: "TxSig2",
		EventIndex:  0,
		Slot:        200,
		Timestamp:   2000,
	}

	candidate, err := detector.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if candidate != nil {
		t.Error("Should not create candidate for already existing mint")
	}

	// Verify still only one candidate
	candidates, _ := store.GetByMint(ctx, "ExistingMint")
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate, got %d", len(candidates))
	}
}

func TestDetector_MultipleMintsOrdered(t *testing.T) {
	store := memory.NewCandidateStore()
	detector := NewDetector(store)
	ctx := context.Background()

	// Unordered events for different mints
	events := []*SwapEvent{
		{Mint: "MintC", TxSignature: "Tx3", EventIndex: 0, Slot: 300, Timestamp: 3000},
		{Mint: "MintA", TxSignature: "Tx1", EventIndex: 0, Slot: 100, Timestamp: 1000},
		{Mint: "MintB", TxSignature: "Tx2", EventIndex: 0, Slot: 200, Timestamp: 2000},
		{Mint: "MintA", TxSignature: "Tx4", EventIndex: 0, Slot: 400, Timestamp: 4000}, // Duplicate mint
	}

	candidates, err := detector.ProcessEvents(ctx, events)
	if err != nil {
		t.Fatalf("ProcessEvents failed: %v", err)
	}

	// Should have 3 candidates (MintA, MintB, MintC - each once)
	if len(candidates) != 3 {
		t.Fatalf("Expected 3 candidates, got %d", len(candidates))
	}

	// Candidates should be ordered by (slot, tx_signature, event_index)
	// First should be MintA (slot 100)
	if candidates[0].Mint != "MintA" {
		t.Errorf("First candidate should be MintA, got %s", candidates[0].Mint)
	}
	if candidates[0].Slot != 100 {
		t.Errorf("First candidate slot should be 100, got %d", candidates[0].Slot)
	}

	// Second should be MintB (slot 200)
	if candidates[1].Mint != "MintB" {
		t.Errorf("Second candidate should be MintB, got %s", candidates[1].Mint)
	}

	// Third should be MintC (slot 300)
	if candidates[2].Mint != "MintC" {
		t.Errorf("Third candidate should be MintC, got %s", candidates[2].Mint)
	}
}

func TestParser_Deterministic(t *testing.T) {
	parser := NewParser()

	logs := []string{
		"Program log: Initialize",
		"Program log: Swap mint=TokenMint123 pool=PoolAddr456",
		"Program log: Success",
	}

	// Parse multiple times
	for i := 0; i < 5; i++ {
		events := parser.ParseSwapEvents(logs, "TxSig789", 12345, 1704067200000)

		if len(events) != 1 {
			t.Fatalf("Run %d: Expected 1 event, got %d", i, len(events))
		}

		if events[0].Mint != "TokenMint123" {
			t.Errorf("Run %d: Expected mint TokenMint123, got %s", i, events[0].Mint)
		}
		if events[0].Pool == nil || *events[0].Pool != "PoolAddr456" {
			t.Errorf("Run %d: Expected pool PoolAddr456", i)
		}
		if events[0].EventIndex != 1 {
			t.Errorf("Run %d: Expected event_index 1, got %d", i, events[0].EventIndex)
		}
	}
}

func TestParser_MultipleEventsInTx(t *testing.T) {
	parser := NewParser()

	logs := []string{
		"Program log: Initialize",
		"Program log: Swap mint=TokenMint1 pool=Pool1",
		"Program log: Process",
		"Program log: Swap mint=TokenMint2 pool=Pool2",
		"Program log: Success",
	}

	events := parser.ParseSwapEvents(logs, "TxSig", 100, 1000)

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Should be sorted by event_index (order of appearance)
	if events[0].EventIndex != 1 {
		t.Errorf("First event should have index 1, got %d", events[0].EventIndex)
	}
	if events[0].Mint != "TokenMint1" {
		t.Errorf("First event should be TokenMint1, got %s", events[0].Mint)
	}

	if events[1].EventIndex != 3 {
		t.Errorf("Second event should have index 3, got %d", events[1].EventIndex)
	}
	if events[1].Mint != "TokenMint2" {
		t.Errorf("Second event should be TokenMint2, got %s", events[1].Mint)
	}
}

func TestParser_NoPool(t *testing.T) {
	parser := NewParser()

	logs := []string{
		"Program log: Swap mint=TokenMint123",
	}

	events := parser.ParseSwapEvents(logs, "TxSig", 100, 1000)

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].Pool != nil {
		t.Error("Pool should be nil when not present in log")
	}
}

func TestSortSwapEvents(t *testing.T) {
	events := []*SwapEvent{
		{Slot: 200, TxSignature: "tx2", EventIndex: 0},
		{Slot: 100, TxSignature: "tx1", EventIndex: 1},
		{Slot: 100, TxSignature: "tx1", EventIndex: 0},
	}

	SortSwapEvents(events)

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

// Tests for persistence functionality

func TestDetector_WithProgressStore(t *testing.T) {
	candidateStore := memory.NewCandidateStore()
	progressStore := memory.NewDiscoveryProgressStore()
	ctx := context.Background()

	// Create detector with persistence
	detector := NewDetector(candidateStore).WithProgressStore(progressStore)

	// Process first event
	event1 := &SwapEvent{
		Mint:        "MintA",
		TxSignature: "Tx1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
	}

	candidate, err := detector.ProcessEvent(ctx, event1)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if candidate == nil {
		t.Fatal("Expected candidate to be created")
	}

	// Verify mint is persisted
	seen, err := progressStore.IsMintSeen(ctx, "MintA")
	if err != nil {
		t.Fatalf("IsMintSeen failed: %v", err)
	}
	if !seen {
		t.Error("Mint should be marked as seen in progress store")
	}

	// Save progress
	if err := detector.SaveProgress(ctx, 100, "Tx1"); err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	// Verify progress is saved
	progress, err := progressStore.GetLastProcessed(ctx)
	if err != nil {
		t.Fatalf("GetLastProcessed failed: %v", err)
	}
	if progress.Slot != 100 {
		t.Errorf("Expected slot 100, got %d", progress.Slot)
	}
	if progress.Signature != "Tx1" {
		t.Errorf("Expected signature Tx1, got %s", progress.Signature)
	}
}

func TestDetector_LoadState_ResumesFromPersistence(t *testing.T) {
	candidateStore := memory.NewCandidateStore()
	progressStore := memory.NewDiscoveryProgressStore()
	ctx := context.Background()

	// Pre-populate progress store (simulating previous run)
	_ = progressStore.MarkMintSeen(ctx, "SeenMint1")
	_ = progressStore.MarkMintSeen(ctx, "SeenMint2")
	_ = progressStore.SetLastProcessed(ctx, &storage.DiscoveryProgress{
		Slot:      500,
		Signature: "TxPrevious",
	})

	// Create new detector and load state
	detector := NewDetector(candidateStore).WithProgressStore(progressStore)
	if err := detector.LoadState(ctx); err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	// Verify in-memory cache is populated
	if detector.SeenCount() != 2 {
		t.Errorf("Expected 2 seen mints, got %d", detector.SeenCount())
	}

	// Processing an already-seen mint should return nil
	event := &SwapEvent{
		Mint:        "SeenMint1",
		TxSignature: "TxNew",
		EventIndex:  0,
		Slot:        600,
		Timestamp:   6000,
	}

	candidate, err := detector.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if candidate != nil {
		t.Error("Already-seen mint should not create new candidate")
	}

	// Check progress
	progress, err := detector.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress failed: %v", err)
	}
	if progress.Slot != 500 {
		t.Errorf("Expected previous slot 500, got %d", progress.Slot)
	}
}

func TestDetector_NoPersistence_Fallback(t *testing.T) {
	candidateStore := memory.NewCandidateStore()
	ctx := context.Background()

	// Create detector WITHOUT persistence
	detector := NewDetector(candidateStore)

	// LoadState should be no-op
	if err := detector.LoadState(ctx); err != nil {
		t.Fatalf("LoadState should not fail without persistence: %v", err)
	}

	// SaveProgress should be no-op
	if err := detector.SaveProgress(ctx, 100, "Tx1"); err != nil {
		t.Fatalf("SaveProgress should not fail without persistence: %v", err)
	}

	// GetProgress should return nil
	progress, err := detector.GetProgress(ctx)
	if err != nil {
		t.Fatalf("GetProgress should not fail without persistence: %v", err)
	}
	if progress != nil {
		t.Error("GetProgress should return nil without persistence")
	}

	// ProcessEvent should still work
	event := &SwapEvent{
		Mint:        "MintA",
		TxSignature: "Tx1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
	}

	candidate, err := detector.ProcessEvent(ctx, event)
	if err != nil {
		t.Fatalf("ProcessEvent failed: %v", err)
	}
	if candidate == nil {
		t.Fatal("Expected candidate to be created")
	}
}
