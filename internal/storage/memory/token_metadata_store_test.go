package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestTokenMetadataStore_InsertAndGetByID(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	name := "TestToken"
	symbol := "TT"
	supply := 1000000.0

	meta := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "mint1",
		Name:        &name,
		Symbol:      &symbol,
		Decimals:    9,
		Supply:      &supply,
		FetchedAt:   1704067200000,
		CreatedAt:   1704067200000,
	}

	err := store.Insert(ctx, meta)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
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

func TestTokenMetadataStore_GetByMint(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	meta := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "mint1",
		Decimals:    9,
	}

	if err := store.Insert(ctx, meta); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	result, err := store.GetByMint(ctx, "mint1")
	if err != nil {
		t.Fatalf("GetByMint failed: %v", err)
	}

	if result.CandidateID != "cand1" {
		t.Errorf("CandidateID mismatch: got %s, want cand1", result.CandidateID)
	}
}

func TestTokenMetadataStore_DuplicateCandidateID(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	meta := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "mint1",
		Decimals:    9,
	}

	if err := store.Insert(ctx, meta); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Same candidate ID, different mint - should fail
	meta2 := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "mint2",
		Decimals:    6,
	}

	err := store.Insert(ctx, meta2)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestTokenMetadataStore_DuplicateMint(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	// First candidate with mint
	meta1 := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "shared_mint",
		Decimals:    9,
		FetchedAt:   1000,
	}

	// Second candidate with same mint - should fail
	meta2 := &domain.TokenMetadata{
		CandidateID: "cand2",
		Mint:        "shared_mint",
		Decimals:    9,
		FetchedAt:   2000,
	}

	if err := store.Insert(ctx, meta1); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.Insert(ctx, meta2)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey for duplicate mint, got %v", err)
	}

	// Original should still be retrievable
	result, err := store.GetByMint(ctx, "shared_mint")
	if err != nil {
		t.Fatalf("GetByMint failed: %v", err)
	}

	if result.CandidateID != "cand1" {
		t.Errorf("Expected cand1, got %s", result.CandidateID)
	}
}

func TestTokenMetadataStore_NotFound(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}

	_, err = store.GetByMint(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestTokenMetadataStore_InvalidInput(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	err := store.Insert(ctx, nil)
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil, got %v", err)
	}

	err = store.Insert(ctx, &domain.TokenMetadata{CandidateID: ""})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty ID, got %v", err)
	}
}

func TestTokenMetadataStore_ReturnsCopy(t *testing.T) {
	store := NewTokenMetadataStore()
	ctx := context.Background()

	meta := &domain.TokenMetadata{
		CandidateID: "cand1",
		Mint:        "mint1",
		Decimals:    9,
	}

	if err := store.Insert(ctx, meta); err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Modify original
	meta.Decimals = 6

	// Should return original value
	result, _ := store.GetByID(ctx, "cand1")
	if result.Decimals != 9 {
		t.Error("Store should return copy, not reference")
	}
}
