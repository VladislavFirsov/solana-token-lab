package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestLiquidityEventStore_InsertAndGetByCandidateID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-test-candidate-1")

	store := NewLiquidityEventStore(pool)

	event := &domain.LiquidityEvent{
		CandidateID:    candidateID,
		Pool:           "PoolAddress1",
		Mint:           "MintAddress1",
		TxSignature:    "LiqTx1",
		EventIndex:     0,
		Slot:           101,
		Timestamp:      1700000001000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1010.0,
	}

	// Insert
	err := store.Insert(ctx, event)
	require.NoError(t, err)

	// GetByCandidateID
	events, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, event.CandidateID, events[0].CandidateID)
	assert.Equal(t, event.Pool, events[0].Pool)
	assert.Equal(t, event.Mint, events[0].Mint)
	assert.Equal(t, event.TxSignature, events[0].TxSignature)
	assert.Equal(t, event.EventIndex, events[0].EventIndex)
	assert.Equal(t, event.Slot, events[0].Slot)
	assert.Equal(t, event.Timestamp, events[0].Timestamp)
	assert.Equal(t, event.EventType, events[0].EventType)
	assert.InDelta(t, event.AmountToken, events[0].AmountToken, 0.0001)
	assert.InDelta(t, event.AmountQuote, events[0].AmountQuote, 0.0001)
	assert.InDelta(t, event.LiquidityAfter, events[0].LiquidityAfter, 0.0001)
	assert.NotZero(t, events[0].ID)
	assert.NotZero(t, events[0].CreatedAt)
}

func TestLiquidityEventStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-dup-candidate")

	store := NewLiquidityEventStore(pool)

	event := &domain.LiquidityEvent{
		CandidateID:    candidateID,
		Pool:           "PoolDup",
		Mint:           "MintDup",
		TxSignature:    "DupLiqTx",
		EventIndex:     0,
		Slot:           101,
		Timestamp:      1700000001000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1010.0,
	}

	// First insert should succeed
	err := store.Insert(ctx, event)
	require.NoError(t, err)

	// Second insert with same (tx_signature, event_index, mint/pool) should fail
	err = store.Insert(ctx, event)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestLiquidityEventStore_InsertBulk(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-bulk-candidate")

	store := NewLiquidityEventStore(pool)

	events := []*domain.LiquidityEvent{
		{
			CandidateID:    candidateID,
			Pool:           "PoolBulk",
			Mint:           "MintBulk",
			TxSignature:    "BulkLiqTx1",
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1700000001000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    1000.0,
			AmountQuote:    10.0,
			LiquidityAfter: 1010.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolBulk",
			Mint:           "MintBulk",
			TxSignature:    "BulkLiqTx2",
			EventIndex:     0,
			Slot:           102,
			Timestamp:      1700000002000,
			EventType:      domain.LiquidityEventRemove,
			AmountToken:    500.0,
			AmountQuote:    5.0,
			LiquidityAfter: 505.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolBulk",
			Mint:           "MintBulk",
			TxSignature:    "BulkLiqTx3",
			EventIndex:     0,
			Slot:           103,
			Timestamp:      1700000003000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    200.0,
			AmountQuote:    2.0,
			LiquidityAfter: 707.0,
		},
	}

	// InsertBulk
	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// Verify all inserted
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestLiquidityEventStore_InsertBulkAtomic(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-atomic-candidate")

	store := NewLiquidityEventStore(pool)

	// First batch succeeds
	firstBatch := []*domain.LiquidityEvent{
		{
			CandidateID:    candidateID,
			Pool:           "PoolAtomic",
			Mint:           "MintAtomic",
			TxSignature:    "AtomicLiqTx1",
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1700000001000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    1000.0,
			AmountQuote:    10.0,
			LiquidityAfter: 1010.0,
		},
	}

	err := store.InsertBulk(ctx, firstBatch)
	require.NoError(t, err)

	// Second batch has duplicate - should fail entirely
	secondBatch := []*domain.LiquidityEvent{
		{
			CandidateID:    candidateID,
			Pool:           "PoolAtomic",
			Mint:           "MintAtomic",
			TxSignature:    "AtomicLiqTx2",
			EventIndex:     0,
			Slot:           102,
			Timestamp:      1700000002000,
			EventType:      domain.LiquidityEventRemove,
			AmountToken:    500.0,
			AmountQuote:    5.0,
			LiquidityAfter: 505.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolAtomic",
			Mint:           "MintAtomic",
			TxSignature:    "AtomicLiqTx1", // duplicate!
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1700000001000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    1000.0,
			AmountQuote:    10.0,
			LiquidityAfter: 1010.0,
		},
	}

	err = store.InsertBulk(ctx, secondBatch)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)

	// Should still have only 1 event (atomic rollback)
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestLiquidityEventStore_InsertBulkEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewLiquidityEventStore(pool)

	// Empty bulk should succeed (no-op)
	err := store.InsertBulk(ctx, []*domain.LiquidityEvent{})
	require.NoError(t, err)
}

func TestLiquidityEventStore_GetByTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-timerange-candidate")

	store := NewLiquidityEventStore(pool)

	events := []*domain.LiquidityEvent{
		{
			CandidateID:    candidateID,
			Pool:           "PoolTimeRange",
			Mint:           "MintTimeRange",
			TxSignature:    "TimeLiqTx1",
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    1000.0,
			AmountQuote:    10.0,
			LiquidityAfter: 1010.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolTimeRange",
			Mint:           "MintTimeRange",
			TxSignature:    "TimeLiqTx2",
			EventIndex:     0,
			Slot:           102,
			Timestamp:      2000,
			EventType:      domain.LiquidityEventRemove,
			AmountToken:    500.0,
			AmountQuote:    5.0,
			LiquidityAfter: 505.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolTimeRange",
			Mint:           "MintTimeRange",
			TxSignature:    "TimeLiqTx3",
			EventIndex:     0,
			Slot:           103,
			Timestamp:      3000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    200.0,
			AmountQuote:    2.0,
			LiquidityAfter: 707.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolTimeRange",
			Mint:           "MintTimeRange",
			TxSignature:    "TimeLiqTx4",
			EventIndex:     0,
			Slot:           104,
			Timestamp:      4000,
			EventType:      domain.LiquidityEventRemove,
			AmountToken:    300.0,
			AmountQuote:    3.0,
			LiquidityAfter: 404.0,
		},
	}

	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// GetByTimeRange [2000, 3000] should return 2 events (inclusive)
	result, err := store.GetByTimeRange(ctx, candidateID, 2000, 3000)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, int64(2000), result[0].Timestamp)
	assert.Equal(t, int64(3000), result[1].Timestamp)
}

func TestLiquidityEventStore_Ordering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-order-candidate")

	store := NewLiquidityEventStore(pool)

	// Insert in reverse timestamp order
	events := []*domain.LiquidityEvent{
		{
			CandidateID:    candidateID,
			Pool:           "PoolOrder",
			Mint:           "MintOrder",
			TxSignature:    "OrderLiqTx3",
			EventIndex:     0,
			Slot:           103,
			Timestamp:      3000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    300.0,
			AmountQuote:    3.0,
			LiquidityAfter: 303.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolOrder",
			Mint:           "MintOrder",
			TxSignature:    "OrderLiqTx1",
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1000,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    100.0,
			AmountQuote:    1.0,
			LiquidityAfter: 101.0,
		},
		{
			CandidateID:    candidateID,
			Pool:           "PoolOrder",
			Mint:           "MintOrder",
			TxSignature:    "OrderLiqTx2",
			EventIndex:     0,
			Slot:           102,
			Timestamp:      2000,
			EventType:      domain.LiquidityEventRemove,
			AmountToken:    50.0,
			AmountQuote:    0.5,
			LiquidityAfter: 50.5,
		},
	}

	for _, e := range events {
		err := store.Insert(ctx, e)
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

func TestLiquidityEventStore_EmptyResult(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewLiquidityEventStore(pool)

	// GetByCandidateID with no matching records
	result, err := store.GetByCandidateID(ctx, "nonexistent-candidate")
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetByTimeRange with no matching records
	candidateID := createTestCandidate(t, ctx, pool, "liq-empty-candidate")
	result, err = store.GetByTimeRange(ctx, candidateID, 0, 100)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLiquidityEventStore_EventTypes(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-eventtype-candidate")

	store := NewLiquidityEventStore(pool)

	// Insert add event
	addEvent := &domain.LiquidityEvent{
		CandidateID:    candidateID,
		Pool:           "PoolEventType",
		Mint:           "MintEventType",
		TxSignature:    "EventTypeTx1",
		EventIndex:     0,
		Slot:           101,
		Timestamp:      1000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1010.0,
	}

	err := store.Insert(ctx, addEvent)
	require.NoError(t, err)

	// Insert remove event
	removeEvent := &domain.LiquidityEvent{
		CandidateID:    candidateID,
		Pool:           "PoolEventType",
		Mint:           "MintEventType",
		TxSignature:    "EventTypeTx2",
		EventIndex:     0,
		Slot:           102,
		Timestamp:      2000,
		EventType:      domain.LiquidityEventRemove,
		AmountToken:    500.0,
		AmountQuote:    5.0,
		LiquidityAfter: 505.0,
	}

	err = store.Insert(ctx, removeEvent)
	require.NoError(t, err)

	// Verify both types stored correctly
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, domain.LiquidityEventAdd, result[0].EventType)
	assert.Equal(t, domain.LiquidityEventRemove, result[1].EventType)
}

func TestLiquidityEventStore_NullableCandidateID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewLiquidityEventStore(pool)

	// Insert event without candidate_id (deferred association)
	event := &domain.LiquidityEvent{
		CandidateID:    "", // null
		Pool:           "PoolNoCandidate",
		Mint:           "MintNoCandidate",
		TxSignature:    "NoCandidateTx",
		EventIndex:     0,
		Slot:           101,
		Timestamp:      1000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1010.0,
	}

	err := store.Insert(ctx, event)
	require.NoError(t, err)

	// Cannot retrieve by GetByCandidateID since it's null
	// But shouldn't error - just empty result
	result, err := store.GetByCandidateID(ctx, "")
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestLiquidityEventStore_EmptyPoolAndMint(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "liq-empty-pool-candidate")

	store := NewLiquidityEventStore(pool)

	// Insert event with empty pool and mint
	event := &domain.LiquidityEvent{
		CandidateID:    candidateID,
		Pool:           "", // empty -> NULL
		Mint:           "", // empty -> NULL
		TxSignature:    "EmptyPoolMintTx",
		EventIndex:     0,
		Slot:           101,
		Timestamp:      1000,
		EventType:      domain.LiquidityEventAdd,
		AmountToken:    1000.0,
		AmountQuote:    10.0,
		LiquidityAfter: 1010.0,
	}

	err := store.Insert(ctx, event)
	require.NoError(t, err)

	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, result, 1)
	assert.Empty(t, result[0].Pool)
	assert.Empty(t, result[0].Mint)
}
