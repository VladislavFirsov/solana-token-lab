package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestSwapEventStore_InsertAndGetByTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	event := &domain.SwapEvent{
		Mint:        "EventMint1",
		Pool:        ptr("EventPool1"),
		TxSignature: "EventTx1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
		AmountOut:   100.5,
	}

	// Insert
	err := store.Insert(ctx, event)
	require.NoError(t, err)

	// GetByTimeRange [1000, 2000) - should include the event
	events, err := store.GetByTimeRange(ctx, 1000, 2000)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Equal(t, event.Mint, events[0].Mint)
	assert.Equal(t, *event.Pool, *events[0].Pool)
	assert.Equal(t, event.TxSignature, events[0].TxSignature)
	assert.Equal(t, event.EventIndex, events[0].EventIndex)
	assert.Equal(t, event.Slot, events[0].Slot)
	assert.Equal(t, event.Timestamp, events[0].Timestamp)
	assert.InDelta(t, event.AmountOut, events[0].AmountOut, 0.0001)
}

func TestSwapEventStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	event := &domain.SwapEvent{
		Mint:        "DupEventMint",
		TxSignature: "DupEventTx",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
		AmountOut:   50.0,
	}

	// First insert should succeed
	err := store.Insert(ctx, event)
	require.NoError(t, err)

	// Second insert with same (mint, tx_signature, event_index) should fail
	err = store.Insert(ctx, event)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestSwapEventStore_InsertBulk(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	events := []*domain.SwapEvent{
		{
			Mint:        "BulkEventMint",
			Pool:        ptr("Pool1"),
			TxSignature: "BulkEventTx1",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
		{
			Mint:        "BulkEventMint",
			Pool:        ptr("Pool1"),
			TxSignature: "BulkEventTx2",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   2000,
			AmountOut:   200.0,
		},
		{
			Mint:        "BulkEventMint2",
			TxSignature: "BulkEventTx3",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   3000,
			AmountOut:   300.0,
		},
	}

	// InsertBulk
	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// Verify all inserted
	result, err := store.GetByTimeRange(ctx, 0, 10000)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestSwapEventStore_InsertBulkAtomic(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	// First batch succeeds
	firstBatch := []*domain.SwapEvent{
		{
			Mint:        "AtomicEventMint",
			TxSignature: "AtomicEventTx1",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
	}

	err := store.InsertBulk(ctx, firstBatch)
	require.NoError(t, err)

	// Second batch has duplicate - should fail entirely
	secondBatch := []*domain.SwapEvent{
		{
			Mint:        "AtomicEventMint",
			TxSignature: "AtomicEventTx2",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   2000,
			AmountOut:   200.0,
		},
		{
			Mint:        "AtomicEventMint",
			TxSignature: "AtomicEventTx1", // duplicate!
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
	}

	err = store.InsertBulk(ctx, secondBatch)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)

	// Should still have only 1 event (atomic rollback)
	result, err := store.GetByTimeRange(ctx, 0, 10000)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestSwapEventStore_InsertBulkEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	// Empty bulk should succeed (no-op)
	err := store.InsertBulk(ctx, []*domain.SwapEvent{})
	require.NoError(t, err)
}

func TestSwapEventStore_GetByMintTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	events := []*domain.SwapEvent{
		{
			Mint:        "MintA",
			TxSignature: "MintRangeTx1",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
		{
			Mint:        "MintA",
			TxSignature: "MintRangeTx2",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   2000,
			AmountOut:   200.0,
		},
		{
			Mint:        "MintB",
			TxSignature: "MintRangeTx3",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   1500,
			AmountOut:   150.0,
		},
		{
			Mint:        "MintA",
			TxSignature: "MintRangeTx4",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   3000,
			AmountOut:   300.0,
		},
	}

	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// GetByMintTimeRange for MintA [1000, 2500) should return 2 events
	result, err := store.GetByMintTimeRange(ctx, "MintA", 1000, 2500)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Equal(t, int64(1000), result[0].Timestamp)
	assert.Equal(t, int64(2000), result[1].Timestamp)
}

func TestSwapEventStore_GetDistinctMintsByTimeRange(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	events := []*domain.SwapEvent{
		{
			Mint:        "DistinctMintA",
			TxSignature: "DistinctTx1",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
		{
			Mint:        "DistinctMintA",
			TxSignature: "DistinctTx2",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1500,
			AmountOut:   150.0,
		},
		{
			Mint:        "DistinctMintB",
			TxSignature: "DistinctTx3",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   2000,
			AmountOut:   200.0,
		},
		{
			Mint:        "DistinctMintC",
			TxSignature: "DistinctTx4",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   3000,
			AmountOut:   300.0,
		},
		{
			Mint:        "DistinctMintA",
			TxSignature: "DistinctTx5",
			EventIndex:  0,
			Slot:        104,
			Timestamp:   3500,
			AmountOut:   350.0,
		},
	}

	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// GetDistinctMintsByTimeRange [1000, 3000) should return MintA, MintB
	result, err := store.GetDistinctMintsByTimeRange(ctx, 1000, 3000)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	assert.Contains(t, result, "DistinctMintA")
	assert.Contains(t, result, "DistinctMintB")
	assert.NotContains(t, result, "DistinctMintC")
}

func TestSwapEventStore_NullPool(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	event := &domain.SwapEvent{
		Mint:        "NullPoolMint",
		Pool:        nil, // NULL
		TxSignature: "NullPoolTx",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   1000,
		AmountOut:   100.0,
	}

	err := store.Insert(ctx, event)
	require.NoError(t, err)

	events, err := store.GetByTimeRange(ctx, 0, 2000)
	require.NoError(t, err)

	assert.Len(t, events, 1)
	assert.Nil(t, events[0].Pool)
}

func TestSwapEventStore_TimeRangeBoundaries(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	events := []*domain.SwapEvent{
		{
			Mint:        "BoundaryMint",
			TxSignature: "BoundaryTx1",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1000, // inclusive start
			AmountOut:   100.0,
		},
		{
			Mint:        "BoundaryMint",
			TxSignature: "BoundaryTx2",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   2000, // exclusive end
			AmountOut:   200.0,
		},
	}

	err := store.InsertBulk(ctx, events)
	require.NoError(t, err)

	// [1000, 2000) should include only timestamp=1000 (exclusive end)
	result, err := store.GetByTimeRange(ctx, 1000, 2000)
	require.NoError(t, err)

	assert.Len(t, result, 1)
	assert.Equal(t, int64(1000), result[0].Timestamp)
}

func TestSwapEventStore_Ordering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	// Insert out of order
	events := []*domain.SwapEvent{
		{
			Mint:        "OrderMint",
			TxSignature: "OrderTx3",
			EventIndex:  0,
			Slot:        103,
			Timestamp:   3000,
			AmountOut:   300.0,
		},
		{
			Mint:        "OrderMint",
			TxSignature: "OrderTx1",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1000,
			AmountOut:   100.0,
		},
		{
			Mint:        "OrderMint",
			TxSignature: "OrderTx2",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   2000,
			AmountOut:   200.0,
		},
	}

	for _, e := range events {
		err := store.Insert(ctx, e)
		require.NoError(t, err)
	}

	// Results should be ordered by timestamp ASC
	result, err := store.GetByTimeRange(ctx, 0, 10000)
	require.NoError(t, err)

	assert.Len(t, result, 3)
	assert.Equal(t, int64(1000), result[0].Timestamp)
	assert.Equal(t, int64(2000), result[1].Timestamp)
	assert.Equal(t, int64(3000), result[2].Timestamp)
}

func TestSwapEventStore_EmptyResult(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewSwapEventStore(pool)

	// GetByTimeRange with no matching records
	result, err := store.GetByTimeRange(ctx, 0, 1000)
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetByMintTimeRange with no matching records
	result, err = store.GetByMintTimeRange(ctx, "NonexistentMint", 0, 1000)
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetDistinctMintsByTimeRange with no matching records
	mints, err := store.GetDistinctMintsByTimeRange(ctx, 0, 1000)
	require.NoError(t, err)
	assert.Empty(t, mints)
}
