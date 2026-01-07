package clickhouse

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestPriceTimeseriesStore_InsertBulk(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Test empty insert
	err := store.InsertBulk(ctx, nil)
	assert.NoError(t, err)

	// Test single insert
	points := []*domain.PriceTimeseriesPoint{
		{
			CandidateID: "cand-1",
			TimestampMs: 1000,
			Slot:        100,
			Price:       1.5,
			Volume:      1000.0,
			SwapCount:   10,
		},
	}

	err = store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Verify insert
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cand-1", got[0].CandidateID)
	assert.Equal(t, int64(1000), got[0].TimestampMs)
	assert.Equal(t, int64(100), got[0].Slot)
	assert.Equal(t, 1.5, got[0].Price)
	assert.Equal(t, 1000.0, got[0].Volume)
	assert.Equal(t, 10, got[0].SwapCount)
}

func TestPriceTimeseriesStore_InsertBulk_DuplicateKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, Slot: 100, Price: 1.5, Volume: 1000.0, SwapCount: 10},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Try to insert duplicate
	err = store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestPriceTimeseriesStore_InsertBulk_IntraBatchDuplicate(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Same key twice in one batch
	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, Slot: 100, Price: 1.5, Volume: 1000.0, SwapCount: 10},
		{CandidateID: "cand-1", TimestampMs: 1000, Slot: 100, Price: 2.0, Volume: 2000.0, SwapCount: 20},
	}

	err := store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestPriceTimeseriesStore_GetByCandidateID(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Insert multiple points for multiple candidates
	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, Slot: 100, Price: 1.0, Volume: 100.0, SwapCount: 1},
		{CandidateID: "cand-1", TimestampMs: 2000, Slot: 200, Price: 2.0, Volume: 200.0, SwapCount: 2},
		{CandidateID: "cand-2", TimestampMs: 1500, Slot: 150, Price: 1.5, Volume: 150.0, SwapCount: 3},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Get only cand-1
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify ordering by timestamp
	assert.Equal(t, int64(1000), got[0].TimestampMs)
	assert.Equal(t, int64(2000), got[1].TimestampMs)

	// Get cand-2
	got, err = store.GetByCandidateID(ctx, "cand-2")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "cand-2", got[0].CandidateID)

	// Get non-existent
	got, err = store.GetByCandidateID(ctx, "cand-999")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPriceTimeseriesStore_GetByTimeRange(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Insert points
	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, Slot: 100, Price: 1.0, Volume: 100.0, SwapCount: 1},
		{CandidateID: "cand-1", TimestampMs: 2000, Slot: 200, Price: 2.0, Volume: 200.0, SwapCount: 2},
		{CandidateID: "cand-1", TimestampMs: 3000, Slot: 300, Price: 3.0, Volume: 300.0, SwapCount: 3},
		{CandidateID: "cand-1", TimestampMs: 4000, Slot: 400, Price: 4.0, Volume: 400.0, SwapCount: 4},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Get range [2000, 3000] inclusive
	got, err := store.GetByTimeRange(ctx, "cand-1", 2000, 3000)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(2000), got[0].TimestampMs)
	assert.Equal(t, int64(3000), got[1].TimestampMs)

	// Get exact boundary
	got, err = store.GetByTimeRange(ctx, "cand-1", 1000, 1000)
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Get empty range
	got, err = store.GetByTimeRange(ctx, "cand-1", 5000, 6000)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestPriceTimeseriesStore_GetGlobalTimeRange(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Insert points
	points := []*domain.PriceTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 2000, Slot: 200, Price: 2.0, Volume: 200.0, SwapCount: 2},
		{CandidateID: "cand-2", TimestampMs: 1000, Slot: 100, Price: 1.0, Volume: 100.0, SwapCount: 1},
		{CandidateID: "cand-3", TimestampMs: 5000, Slot: 500, Price: 5.0, Volume: 500.0, SwapCount: 5},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	minTs, maxTs, err := store.GetGlobalTimeRange(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), minTs)
	assert.Equal(t, int64(5000), maxTs)
}

func TestPriceTimeseriesStore_MultipleCandidates(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewPriceTimeseriesStore(conn)
	ctx := context.Background()

	// Insert many points for many candidates
	var points []*domain.PriceTimeseriesPoint
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			points = append(points, &domain.PriceTimeseriesPoint{
				CandidateID: fmt.Sprintf("cand-%d", i),
				TimestampMs: int64(j * 1000),
				Slot:        int64(j * 100),
				Price:       float64(i*10 + j),
				Volume:      float64(i*100 + j*10),
				SwapCount:   i + j,
			})
		}
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Verify each candidate
	for i := 0; i < 10; i++ {
		got, err := store.GetByCandidateID(ctx, fmt.Sprintf("cand-%d", i))
		require.NoError(t, err)
		assert.Len(t, got, 5)
	}
}
