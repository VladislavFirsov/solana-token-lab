package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestCandidateStore_InsertAndGetByID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	candidate := &domain.TokenCandidate{
		CandidateID:  "test-candidate-001",
		Source:       domain.SourceNewToken,
		Mint:         "MintAddress123",
		Pool:         ptr("PoolAddress123"),
		TxSignature:  "TxSig123",
		EventIndex:   0,
		Slot:         100,
		DiscoveredAt: 1700000000000,
	}

	// Insert
	err := store.Insert(ctx, candidate)
	require.NoError(t, err)

	// GetByID
	retrieved, err := store.GetByID(ctx, "test-candidate-001")
	require.NoError(t, err)

	assert.Equal(t, candidate.CandidateID, retrieved.CandidateID)
	assert.Equal(t, candidate.Source, retrieved.Source)
	assert.Equal(t, candidate.Mint, retrieved.Mint)
	assert.Equal(t, *candidate.Pool, *retrieved.Pool)
	assert.Equal(t, candidate.TxSignature, retrieved.TxSignature)
	assert.Equal(t, candidate.EventIndex, retrieved.EventIndex)
	assert.Equal(t, candidate.Slot, retrieved.Slot)
	assert.Equal(t, candidate.DiscoveredAt, retrieved.DiscoveredAt)
	assert.NotZero(t, retrieved.CreatedAt)
}

func TestCandidateStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	candidate := &domain.TokenCandidate{
		CandidateID:  "test-candidate-dup",
		Source:       domain.SourceNewToken,
		Mint:         "MintAddress123",
		TxSignature:  "TxSig123",
		EventIndex:   0,
		Slot:         100,
		DiscoveredAt: 1700000000000,
	}

	// First insert should succeed
	err := store.Insert(ctx, candidate)
	require.NoError(t, err)

	// Second insert should return ErrDuplicateKey
	err = store.Insert(ctx, candidate)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestCandidateStore_GetByIDNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestCandidateStore_GetByMint(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	mint := "SharedMint123"

	// Insert multiple candidates with same mint
	candidates := []*domain.TokenCandidate{
		{
			CandidateID:  "candidate-mint-1",
			Source:       domain.SourceNewToken,
			Mint:         mint,
			TxSignature:  "TxSig1",
			EventIndex:   0,
			Slot:         100,
			DiscoveredAt: 1700000000000,
		},
		{
			CandidateID:  "candidate-mint-2",
			Source:       domain.SourceActiveToken,
			Mint:         mint,
			TxSignature:  "TxSig2",
			EventIndex:   0,
			Slot:         200,
			DiscoveredAt: 1700000001000,
		},
		{
			CandidateID:  "candidate-other-mint",
			Source:       domain.SourceNewToken,
			Mint:         "OtherMint",
			TxSignature:  "TxSig3",
			EventIndex:   0,
			Slot:         300,
			DiscoveredAt: 1700000002000,
		},
	}

	for _, c := range candidates {
		err := store.Insert(ctx, c)
		require.NoError(t, err)
	}

	// GetByMint should return only candidates with matching mint
	result, err := store.GetByMint(ctx, mint)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "candidate-mint-1", result[0].CandidateID)
	assert.Equal(t, "candidate-mint-2", result[1].CandidateID)
}

func TestCandidateStore_GetByTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	// Insert candidates at different times
	candidates := []*domain.TokenCandidate{
		{
			CandidateID:  "candidate-time-1",
			Source:       domain.SourceNewToken,
			Mint:         "Mint1",
			TxSignature:  "TxSig1",
			EventIndex:   0,
			Slot:         100,
			DiscoveredAt: 1000,
		},
		{
			CandidateID:  "candidate-time-2",
			Source:       domain.SourceNewToken,
			Mint:         "Mint2",
			TxSignature:  "TxSig2",
			EventIndex:   0,
			Slot:         200,
			DiscoveredAt: 2000,
		},
		{
			CandidateID:  "candidate-time-3",
			Source:       domain.SourceNewToken,
			Mint:         "Mint3",
			TxSignature:  "TxSig3",
			EventIndex:   0,
			Slot:         300,
			DiscoveredAt: 3000,
		},
		{
			CandidateID:  "candidate-time-4",
			Source:       domain.SourceNewToken,
			Mint:         "Mint4",
			TxSignature:  "TxSig4",
			EventIndex:   0,
			Slot:         400,
			DiscoveredAt: 4000,
		},
	}

	for _, c := range candidates {
		err := store.Insert(ctx, c)
		require.NoError(t, err)
	}

	// GetByTimeRange [2000, 3000] should return candidates 2 and 3 (inclusive)
	result, err := store.GetByTimeRange(ctx, 2000, 3000)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "candidate-time-2", result[0].CandidateID)
	assert.Equal(t, "candidate-time-3", result[1].CandidateID)

	// GetByTimeRange with exact boundaries
	result, err = store.GetByTimeRange(ctx, 1000, 4000)
	require.NoError(t, err)
	assert.Len(t, result, 4)
}

func TestCandidateStore_GetBySource(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	// Insert candidates with different sources
	candidates := []*domain.TokenCandidate{
		{
			CandidateID:  "candidate-new-1",
			Source:       domain.SourceNewToken,
			Mint:         "Mint1",
			TxSignature:  "TxSig1",
			EventIndex:   0,
			Slot:         100,
			DiscoveredAt: 1000,
		},
		{
			CandidateID:  "candidate-new-2",
			Source:       domain.SourceNewToken,
			Mint:         "Mint2",
			TxSignature:  "TxSig2",
			EventIndex:   0,
			Slot:         200,
			DiscoveredAt: 2000,
		},
		{
			CandidateID:  "candidate-active-1",
			Source:       domain.SourceActiveToken,
			Mint:         "Mint3",
			TxSignature:  "TxSig3",
			EventIndex:   0,
			Slot:         300,
			DiscoveredAt: 3000,
		},
	}

	for _, c := range candidates {
		err := store.Insert(ctx, c)
		require.NoError(t, err)
	}

	// GetBySource NEW_TOKEN
	newTokens, err := store.GetBySource(ctx, domain.SourceNewToken)
	require.NoError(t, err)
	assert.Len(t, newTokens, 2)

	// GetBySource ACTIVE_TOKEN
	activeTokens, err := store.GetBySource(ctx, domain.SourceActiveToken)
	require.NoError(t, err)
	assert.Len(t, activeTokens, 1)
	assert.Equal(t, "candidate-active-1", activeTokens[0].CandidateID)
}

func TestCandidateStore_NullPool(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	// Insert candidate with nil Pool
	candidate := &domain.TokenCandidate{
		CandidateID:  "candidate-nil-pool",
		Source:       domain.SourceNewToken,
		Mint:         "MintNilPool",
		Pool:         nil, // NULL
		TxSignature:  "TxSig123",
		EventIndex:   0,
		Slot:         100,
		DiscoveredAt: 1700000000000,
	}

	err := store.Insert(ctx, candidate)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, "candidate-nil-pool")
	require.NoError(t, err)

	assert.Nil(t, retrieved.Pool)
}

func TestCandidateStore_Ordering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	// Insert candidates in random order
	candidates := []*domain.TokenCandidate{
		{
			CandidateID:  "z-candidate", // later in alphabetical order
			Source:       domain.SourceNewToken,
			Mint:         "SameMint",
			TxSignature:  "TxSig1",
			EventIndex:   0,
			Slot:         100,
			DiscoveredAt: 1000, // same time
		},
		{
			CandidateID:  "a-candidate", // earlier in alphabetical order
			Source:       domain.SourceNewToken,
			Mint:         "SameMint",
			TxSignature:  "TxSig2",
			EventIndex:   1,
			Slot:         200,
			DiscoveredAt: 1000, // same time
		},
	}

	// Insert in reverse order
	for i := len(candidates) - 1; i >= 0; i-- {
		err := store.Insert(ctx, candidates[i])
		require.NoError(t, err)
	}

	// Results should be ordered by discovered_at ASC, candidate_id ASC
	result, err := store.GetByMint(ctx, "SameMint")
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, "a-candidate", result[0].CandidateID)
	assert.Equal(t, "z-candidate", result[1].CandidateID)
}

func TestCandidateStore_EmptyResult(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewCandidateStore(pool)
	ctx := context.Background()

	// GetByMint with no matching records
	result, err := store.GetByMint(ctx, "NonexistentMint")
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetByTimeRange with no matching records
	result, err = store.GetByTimeRange(ctx, 9999999, 9999999999)
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetBySource with no matching records
	result, err = store.GetBySource(ctx, domain.SourceNewToken)
	require.NoError(t, err)
	assert.Empty(t, result)
}
