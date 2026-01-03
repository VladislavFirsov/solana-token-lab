package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestCandidateStore_InsertAndGet(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	c := &domain.TokenCandidate{
		CandidateID:  "abc123",
		Source:       domain.SourceNewToken,
		Mint:         "mint123",
		TxSignature:  "sig123",
		Slot:         100,
		DiscoveredAt: 1704067200000,
		CreatedAt:    1704067200000,
	}

	// Insert
	err := store.Insert(ctx, c)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Get
	got, err := store.GetByID(ctx, "abc123")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.CandidateID != c.CandidateID {
		t.Errorf("CandidateID mismatch: got %s, want %s", got.CandidateID, c.CandidateID)
	}
	if got.Mint != c.Mint {
		t.Errorf("Mint mismatch: got %s, want %s", got.Mint, c.Mint)
	}
}

func TestCandidateStore_DuplicateKey(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	c := &domain.TokenCandidate{
		CandidateID:  "abc123",
		Source:       domain.SourceNewToken,
		Mint:         "mint123",
		TxSignature:  "sig123",
		Slot:         100,
		DiscoveredAt: 1704067200000,
	}

	// First insert
	err := store.Insert(ctx, c)
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Second insert should fail
	err = store.Insert(ctx, c)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestCandidateStore_NotFound(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestCandidateStore_GetByTimeRange(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	candidates := []*domain.TokenCandidate{
		{CandidateID: "c1", Mint: "m1", DiscoveredAt: 1000, Source: domain.SourceNewToken, TxSignature: "s1", Slot: 1},
		{CandidateID: "c2", Mint: "m2", DiscoveredAt: 2000, Source: domain.SourceNewToken, TxSignature: "s2", Slot: 2},
		{CandidateID: "c3", Mint: "m3", DiscoveredAt: 3000, Source: domain.SourceNewToken, TxSignature: "s3", Slot: 3},
		{CandidateID: "c4", Mint: "m4", DiscoveredAt: 4000, Source: domain.SourceNewToken, TxSignature: "s4", Slot: 4},
	}

	for _, c := range candidates {
		if err := store.Insert(ctx, c); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	// Query range [2000, 3000]
	result, err := store.GetByTimeRange(ctx, 2000, 3000)
	if err != nil {
		t.Fatalf("GetByTimeRange failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 results, got %d", len(result))
	}

	// Verify order
	if result[0].CandidateID != "c2" {
		t.Errorf("First result should be c2, got %s", result[0].CandidateID)
	}
	if result[1].CandidateID != "c3" {
		t.Errorf("Second result should be c3, got %s", result[1].CandidateID)
	}
}

func TestCandidateStore_GetBySource(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	candidates := []*domain.TokenCandidate{
		{CandidateID: "c1", Mint: "m1", Source: domain.SourceNewToken, TxSignature: "s1", Slot: 1, DiscoveredAt: 1000},
		{CandidateID: "c2", Mint: "m2", Source: domain.SourceActiveToken, TxSignature: "s2", Slot: 2, DiscoveredAt: 2000},
		{CandidateID: "c3", Mint: "m3", Source: domain.SourceNewToken, TxSignature: "s3", Slot: 3, DiscoveredAt: 3000},
	}

	for _, c := range candidates {
		if err := store.Insert(ctx, c); err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
	}

	result, err := store.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		t.Fatalf("GetBySource failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 NEW_TOKEN results, got %d", len(result))
	}
}

func TestCandidateStore_ConcurrentInserts(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c := &domain.TokenCandidate{
				CandidateID:  string(rune('a' + id%26)) + string(rune('0'+id)),
				Source:       domain.SourceNewToken,
				Mint:         "mint",
				TxSignature:  "sig",
				Slot:         int64(id),
				DiscoveredAt: int64(id * 1000),
			}
			// Ignore errors; some may be duplicates due to key collision
			_ = store.Insert(ctx, c)
		}(i)
	}

	wg.Wait()
	// Basic smoke test: should not panic
}

func TestCandidateStore_InvalidInput(t *testing.T) {
	store := NewCandidateStore()
	ctx := context.Background()

	// Nil input
	err := store.Insert(ctx, nil)
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil, got %v", err)
	}

	// Empty candidate_id
	err = store.Insert(ctx, &domain.TokenCandidate{CandidateID: ""})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty ID, got %v", err)
	}
}
