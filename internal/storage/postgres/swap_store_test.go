package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// createTestCandidate inserts a test candidate and returns its ID.
func createTestCandidate(t *testing.T, ctx context.Context, pool *Pool, id string) string {
	t.Helper()

	candidateStore := NewCandidateStore(pool)
	candidate := &domain.TokenCandidate{
		CandidateID:  id,
		Source:       domain.SourceNewToken,
		Mint:         "TestMint" + id,
		TxSignature:  "TxSig" + id,
		EventIndex:   0,
		Slot:         100,
		DiscoveredAt: 1700000000000,
	}

	err := candidateStore.Insert(ctx, candidate)
	require.NoError(t, err)
	return id
}

func TestSwapStore_InsertAndGetByCandidateID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-test-candidate-1")

	store := NewSwapStore(pool)

	swap := &domain.Swap{
		CandidateID: candidateID,
		TxSignature: "SwapTx1",
		EventIndex:  0,
		Slot:        101,
		Timestamp:   1700000001000,
		Side:        domain.SwapSideBuy,
		AmountIn:    1.0,
		AmountOut:   100.0,
		Price:       0.01,
	}

	// Insert
	err := store.Insert(ctx, swap)
	require.NoError(t, err)

	// GetByCandidateID
	swaps, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, swaps, 1)
	assert.Equal(t, swap.CandidateID, swaps[0].CandidateID)
	assert.Equal(t, swap.TxSignature, swaps[0].TxSignature)
	assert.Equal(t, swap.EventIndex, swaps[0].EventIndex)
	assert.Equal(t, swap.Slot, swaps[0].Slot)
	assert.Equal(t, swap.Timestamp, swaps[0].Timestamp)
	assert.Equal(t, swap.Side, swaps[0].Side)
	assert.InDelta(t, swap.AmountIn, swaps[0].AmountIn, 0.0001)
	assert.InDelta(t, swap.AmountOut, swaps[0].AmountOut, 0.0001)
	assert.InDelta(t, swap.Price, swaps[0].Price, 0.0001)
	assert.NotZero(t, swaps[0].ID)
	assert.NotZero(t, swaps[0].CreatedAt)
}

func TestSwapStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-dup-candidate")

	store := NewSwapStore(pool)

	swap := &domain.Swap{
		CandidateID: candidateID,
		TxSignature: "DupSwapTx",
		EventIndex:  0,
		Slot:        101,
		Timestamp:   1700000001000,
		Side:        domain.SwapSideBuy,
		AmountIn:    1.0,
		AmountOut:   100.0,
		Price:       0.01,
	}

	// First insert should succeed
	err := store.Insert(ctx, swap)
	require.NoError(t, err)

	// Second insert with same (candidate_id, tx_signature, event_index) should fail
	err = store.Insert(ctx, swap)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestSwapStore_InsertBulk(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-bulk-candidate")

	store := NewSwapStore(pool)

	swaps := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "BulkSwapTx1",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1700000001000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "BulkSwapTx2",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   1700000002000,
			Side:        domain.SwapSideSell,
			AmountIn:    50.0,
			AmountOut:   0.5,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "BulkSwapTx3",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   1700000003000,
			Side:        domain.SwapSideBuy,
			AmountIn:    2.0,
			AmountOut:   180.0,
			Price:       0.011,
		},
	}

	// InsertBulk
	err := store.InsertBulk(ctx, swaps)
	require.NoError(t, err)

	// Verify all inserted
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestSwapStore_InsertBulkAtomic(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-atomic-candidate")

	store := NewSwapStore(pool)

	// First batch succeeds
	firstBatch := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "AtomicTx1",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1700000001000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
	}

	err := store.InsertBulk(ctx, firstBatch)
	require.NoError(t, err)

	// Second batch has duplicate - should fail entirely
	secondBatch := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "AtomicTx2",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   1700000002000,
			Side:        domain.SwapSideSell,
			AmountIn:    50.0,
			AmountOut:   0.5,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "AtomicTx1", // duplicate!
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1700000001000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
	}

	err = store.InsertBulk(ctx, secondBatch)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)

	// Should still have only 1 swap (atomic rollback)
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestSwapStore_InsertBulkEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapStore(pool)

	// Empty bulk should succeed (no-op)
	err := store.InsertBulk(ctx, []*domain.Swap{})
	require.NoError(t, err)
}

func TestSwapStore_GetByTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-timerange-candidate")

	store := NewSwapStore(pool)

	swaps := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "TimeTx1",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "TimeTx2",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   2000,
			Side:        domain.SwapSideSell,
			AmountIn:    50.0,
			AmountOut:   0.5,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "TimeTx3",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   3000,
			Side:        domain.SwapSideBuy,
			AmountIn:    2.0,
			AmountOut:   180.0,
			Price:       0.011,
		},
		{
			CandidateID: candidateID,
			TxSignature: "TimeTx4",
			EventIndex:  0,
			Slot:        104,
			Timestamp:   4000,
			Side:        domain.SwapSideSell,
			AmountIn:    100.0,
			AmountOut:   1.1,
			Price:       0.011,
		},
	}

	err := store.InsertBulk(ctx, swaps)
	require.NoError(t, err)

	// GetByTimeRange [2000, 3000] should return 2 swaps (inclusive)
	result, err := store.GetByTimeRange(ctx, candidateID, 2000, 3000)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, int64(2000), result[0].Timestamp)
	assert.Equal(t, int64(3000), result[1].Timestamp)
}

func TestSwapStore_Ordering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-order-candidate")

	store := NewSwapStore(pool)

	// Insert in reverse timestamp order
	swaps := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "OrderTx3",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   3000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "OrderTx1",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "OrderTx2",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   2000,
			Side:        domain.SwapSideSell,
			AmountIn:    50.0,
			AmountOut:   0.5,
			Price:       0.01,
		},
	}

	for _, s := range swaps {
		err := store.Insert(ctx, s)
		require.NoError(t, err)
	}

	// Results should be ordered by timestamp ASC
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, result, 3)
	assert.Equal(t, int64(1000), result[0].Timestamp)
	assert.Equal(t, int64(2000), result[1].Timestamp)
	assert.Equal(t, int64(3000), result[2].Timestamp)
}

func TestSwapStore_EmptyResult(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapStore(pool)

	// GetByCandidateID with no matching records
	result, err := store.GetByCandidateID(ctx, "nonexistent-candidate")
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetByTimeRange with no matching records
	candidateID := createTestCandidate(t, ctx, pool, "swap-empty-candidate")
	result, err = store.GetByTimeRange(ctx, candidateID, 0, 100)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestSwapStore_SameTimestampDifferentEventIndex(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "swap-multievent-candidate")

	store := NewSwapStore(pool)

	// Multiple swaps in same transaction (same tx_signature, different event_index)
	swaps := []*domain.Swap{
		{
			CandidateID: candidateID,
			TxSignature: "MultiEventTx",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1000,
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: candidateID,
			TxSignature: "MultiEventTx",
			EventIndex:  1,
			Slot:        101,
			Timestamp:   1000,
			Side:        domain.SwapSideSell,
			AmountIn:    50.0,
			AmountOut:   0.5,
			Price:       0.01,
		},
	}

	err := store.InsertBulk(ctx, swaps)
	require.NoError(t, err)

	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 2)
}
