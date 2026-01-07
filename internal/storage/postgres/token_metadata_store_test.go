package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestTokenMetadataStore_InsertAndGetByID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "meta-test-candidate-1")

	store := NewTokenMetadataStore(pool)

	metadata := &domain.TokenMetadata{
		CandidateID: candidateID,
		Mint:        "MetadataMint1",
		Name:        ptr("Test Token"),
		Symbol:      ptr("TST"),
		Decimals:    9,
		Supply:      ptr(1000000.0),
		FetchedAt:   1700000000000,
	}

	// Insert
	err := store.Insert(ctx, metadata)
	require.NoError(t, err)

	// GetByID
	retrieved, err := store.GetByID(ctx, candidateID)
	require.NoError(t, err)

	assert.Equal(t, metadata.CandidateID, retrieved.CandidateID)
	assert.Equal(t, metadata.Mint, retrieved.Mint)
	assert.NotNil(t, retrieved.Name)
	assert.Equal(t, *metadata.Name, *retrieved.Name)
	assert.NotNil(t, retrieved.Symbol)
	assert.Equal(t, *metadata.Symbol, *retrieved.Symbol)
	assert.Equal(t, metadata.Decimals, retrieved.Decimals)
	assert.NotNil(t, retrieved.Supply)
	assert.InDelta(t, *metadata.Supply, *retrieved.Supply, 0.0001)
	assert.Equal(t, metadata.FetchedAt, retrieved.FetchedAt)
	assert.NotZero(t, retrieved.CreatedAt)
}

func TestTokenMetadataStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "meta-dup-candidate")

	store := NewTokenMetadataStore(pool)

	metadata := &domain.TokenMetadata{
		CandidateID: candidateID,
		Mint:        "MetadataMintDup",
		Name:        ptr("Test Token"),
		Symbol:      ptr("TST"),
		Decimals:    9,
		FetchedAt:   1700000000000,
	}

	// First insert should succeed
	err := store.Insert(ctx, metadata)
	require.NoError(t, err)

	// Second insert with same candidate_id should fail
	err = store.Insert(ctx, metadata)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestTokenMetadataStore_GetByIDNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewTokenMetadataStore(pool)

	_, err := store.GetByID(ctx, "nonexistent-candidate")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestTokenMetadataStore_GetByMint(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "meta-mint-candidate")

	store := NewTokenMetadataStore(pool)

	metadata := &domain.TokenMetadata{
		CandidateID: candidateID,
		Mint:        "SharedMintAddress",
		Name:        ptr("Test Token"),
		Symbol:      ptr("TST"),
		Decimals:    9,
		Supply:      ptr(1000000.0),
		FetchedAt:   1700000000000,
	}

	err := store.Insert(ctx, metadata)
	require.NoError(t, err)

	// GetByMint
	retrieved, err := store.GetByMint(ctx, "SharedMintAddress")
	require.NoError(t, err)

	assert.Equal(t, metadata.CandidateID, retrieved.CandidateID)
	assert.Equal(t, metadata.Mint, retrieved.Mint)
}

func TestTokenMetadataStore_GetByMintNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewTokenMetadataStore(pool)

	_, err := store.GetByMint(ctx, "nonexistent-mint")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestTokenMetadataStore_GetByMintLatest(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create two candidates with same mint
	candidateID1 := createTestCandidate(t, ctx, pool, "meta-latest-candidate-1")
	candidateID2 := createTestCandidate(t, ctx, pool, "meta-latest-candidate-2")

	store := NewTokenMetadataStore(pool)

	// First metadata (older)
	metadata1 := &domain.TokenMetadata{
		CandidateID: candidateID1,
		Mint:        "SameMintForLatest",
		Name:        ptr("Old Name"),
		Symbol:      ptr("OLD"),
		Decimals:    9,
		FetchedAt:   1700000000000,
	}

	err := store.Insert(ctx, metadata1)
	require.NoError(t, err)

	// Second metadata (newer)
	metadata2 := &domain.TokenMetadata{
		CandidateID: candidateID2,
		Mint:        "SameMintForLatest",
		Name:        ptr("New Name"),
		Symbol:      ptr("NEW"),
		Decimals:    9,
		FetchedAt:   1700000001000,
	}

	err = store.Insert(ctx, metadata2)
	require.NoError(t, err)

	// GetByMint should return the latest (by fetched_at DESC)
	retrieved, err := store.GetByMint(ctx, "SameMintForLatest")
	require.NoError(t, err)

	assert.Equal(t, candidateID2, retrieved.CandidateID)
	assert.Equal(t, "New Name", *retrieved.Name)
	assert.Equal(t, "NEW", *retrieved.Symbol)
}

func TestTokenMetadataStore_NullableFields(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "meta-nullable-candidate")

	store := NewTokenMetadataStore(pool)

	// Insert with all nullable fields as nil
	metadata := &domain.TokenMetadata{
		CandidateID: candidateID,
		Mint:        "NullableMint",
		Name:        nil, // NULL
		Symbol:      nil, // NULL
		Decimals:    6,
		Supply:      nil, // NULL
		FetchedAt:   1700000000000,
	}

	err := store.Insert(ctx, metadata)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, candidateID)
	require.NoError(t, err)

	assert.Nil(t, retrieved.Name)
	assert.Nil(t, retrieved.Symbol)
	assert.Nil(t, retrieved.Supply)
}

func TestTokenMetadataStore_DifferentDecimals(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	store := NewTokenMetadataStore(pool)

	testCases := []struct {
		name     string
		decimals int
	}{
		{"zero_decimals", 0},
		{"six_decimals", 6},
		{"nine_decimals", 9},
		{"eighteen_decimals", 18},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidateID := createTestCandidate(t, ctx, pool, "meta-dec-"+tc.name)

			metadata := &domain.TokenMetadata{
				CandidateID: candidateID,
				Mint:        "Mint" + tc.name,
				Decimals:    tc.decimals,
				FetchedAt:   1700000000000,
			}

			err := store.Insert(ctx, metadata)
			require.NoError(t, err)

			retrieved, err := store.GetByID(ctx, candidateID)
			require.NoError(t, err)

			assert.Equal(t, tc.decimals, retrieved.Decimals)
		})
	}
}

func TestTokenMetadataStore_LargeSupply(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "meta-supply-candidate")

	store := NewTokenMetadataStore(pool)

	// Large supply value (typical for tokens with many decimals)
	largeSupply := 1000000000000000.0

	metadata := &domain.TokenMetadata{
		CandidateID: candidateID,
		Mint:        "LargeSupplyMint",
		Name:        ptr("Large Supply Token"),
		Symbol:      ptr("BIG"),
		Decimals:    18,
		Supply:      &largeSupply,
		FetchedAt:   1700000000000,
	}

	err := store.Insert(ctx, metadata)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, candidateID)
	require.NoError(t, err)

	assert.NotNil(t, retrieved.Supply)
	assert.InDelta(t, largeSupply, *retrieved.Supply, 1.0)
}
