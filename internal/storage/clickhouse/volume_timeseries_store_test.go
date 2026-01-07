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

func TestVolumeTimeseriesStore_InsertBulk(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Test empty insert
	err := store.InsertBulk(ctx, nil)
	assert.NoError(t, err)

	// Test single insert
	points := []*domain.VolumeTimeseriesPoint{
		{
			CandidateID:     "cand-1",
			TimestampMs:     1000,
			IntervalSeconds: 60,
			Volume:          1000.0,
			SwapCount:       10,
			BuyVolume:       600.0,
			SellVolume:      400.0,
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
	assert.Equal(t, 60, got[0].IntervalSeconds)
	assert.Equal(t, 1000.0, got[0].Volume)
	assert.Equal(t, 10, got[0].SwapCount)
	assert.Equal(t, 600.0, got[0].BuyVolume)
	assert.Equal(t, 400.0, got[0].SellVolume)
}

func TestVolumeTimeseriesStore_InsertBulk_DuplicateKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 1000.0, SwapCount: 10, BuyVolume: 600.0, SellVolume: 400.0},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Try to insert duplicate (same candidate_id, timestamp_ms, interval_seconds)
	err = store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestVolumeTimeseriesStore_InsertBulk_SameTimeDifferentInterval(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Different intervals for same timestamp should work
	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 100.0, SwapCount: 1, BuyVolume: 60.0, SellVolume: 40.0},
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 300, Volume: 500.0, SwapCount: 5, BuyVolume: 300.0, SellVolume: 200.0},
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 3600, Volume: 3600.0, SwapCount: 36, BuyVolume: 2000.0, SellVolume: 1600.0},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Verify all inserted
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestVolumeTimeseriesStore_InsertBulk_IntraBatchDuplicate(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Same key twice in one batch
	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 1000.0, SwapCount: 10, BuyVolume: 600.0, SellVolume: 400.0},
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 2000.0, SwapCount: 20, BuyVolume: 1200.0, SellVolume: 800.0},
	}

	err := store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestVolumeTimeseriesStore_GetByCandidateID(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Insert points with different intervals
	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 100.0, SwapCount: 1, BuyVolume: 60.0, SellVolume: 40.0},
		{CandidateID: "cand-1", TimestampMs: 2000, IntervalSeconds: 60, Volume: 200.0, SwapCount: 2, BuyVolume: 120.0, SellVolume: 80.0},
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 300, Volume: 500.0, SwapCount: 5, BuyVolume: 300.0, SellVolume: 200.0},
		{CandidateID: "cand-2", TimestampMs: 1500, IntervalSeconds: 60, Volume: 150.0, SwapCount: 3, BuyVolume: 90.0, SellVolume: 60.0},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Get only cand-1 (should be ordered by interval_seconds ASC, timestamp_ms ASC)
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	require.Len(t, got, 3)

	// Verify ordering: 60s interval first, then 300s
	assert.Equal(t, 60, got[0].IntervalSeconds)
	assert.Equal(t, int64(1000), got[0].TimestampMs)
	assert.Equal(t, 60, got[1].IntervalSeconds)
	assert.Equal(t, int64(2000), got[1].TimestampMs)
	assert.Equal(t, 300, got[2].IntervalSeconds)

	// Get cand-2
	got, err = store.GetByCandidateID(ctx, "cand-2")
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Get non-existent
	got, err = store.GetByCandidateID(ctx, "cand-999")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestVolumeTimeseriesStore_GetByTimeRange(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Insert points
	points := []*domain.VolumeTimeseriesPoint{
		{CandidateID: "cand-1", TimestampMs: 1000, IntervalSeconds: 60, Volume: 100.0, SwapCount: 1, BuyVolume: 60.0, SellVolume: 40.0},
		{CandidateID: "cand-1", TimestampMs: 2000, IntervalSeconds: 60, Volume: 200.0, SwapCount: 2, BuyVolume: 120.0, SellVolume: 80.0},
		{CandidateID: "cand-1", TimestampMs: 3000, IntervalSeconds: 60, Volume: 300.0, SwapCount: 3, BuyVolume: 180.0, SellVolume: 120.0},
		{CandidateID: "cand-1", TimestampMs: 4000, IntervalSeconds: 60, Volume: 400.0, SwapCount: 4, BuyVolume: 240.0, SellVolume: 160.0},
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

func TestVolumeTimeseriesStore_MultipleIntervals(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Insert points with all three supported intervals
	intervals := []int{60, 300, 3600}
	var points []*domain.VolumeTimeseriesPoint
	for _, interval := range intervals {
		for ts := int64(0); ts < 5000; ts += 1000 {
			points = append(points, &domain.VolumeTimeseriesPoint{
				CandidateID:     "cand-1",
				TimestampMs:     ts,
				IntervalSeconds: interval,
				Volume:          float64(interval) + float64(ts),
				SwapCount:       interval/60 + int(ts/1000),
				BuyVolume:       float64(interval)*0.6 + float64(ts)*0.6,
				SellVolume:      float64(interval)*0.4 + float64(ts)*0.4,
			})
		}
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Verify all inserted
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	assert.Len(t, got, 15) // 3 intervals * 5 timestamps
}

func TestVolumeTimeseriesStore_MultipleCandidates(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewVolumeTimeseriesStore(conn)
	ctx := context.Background()

	// Insert many points for many candidates
	var points []*domain.VolumeTimeseriesPoint
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			points = append(points, &domain.VolumeTimeseriesPoint{
				CandidateID:     fmt.Sprintf("cand-%d", i),
				TimestampMs:     int64(j * 60000),
				IntervalSeconds: 60,
				Volume:          float64(i*100 + j*10),
				SwapCount:       i + j,
				BuyVolume:       float64(i*60 + j*6),
				SellVolume:      float64(i*40 + j*4),
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
