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

func TestDerivedFeatureStore_InsertBulk(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Test empty insert
	err := store.InsertBulk(ctx, nil)
	assert.NoError(t, err)

	// Test single insert with all fields
	priceDelta := 0.5
	priceVelocity := 0.01
	priceAccel := 0.001
	liqDelta := 100.0
	liqVelocity := 1.0
	lastSwap := int64(500)
	lastLiq := int64(1000)

	points := []*domain.DerivedFeaturePoint{
		{
			CandidateID:            "cand-1",
			TimestampMs:            1000,
			PriceDelta:             &priceDelta,
			PriceVelocity:          &priceVelocity,
			PriceAcceleration:      &priceAccel,
			LiquidityDelta:         &liqDelta,
			LiquidityVelocity:      &liqVelocity,
			TokenLifetimeMs:        5000,
			LastSwapIntervalMs:     &lastSwap,
			LastLiqEventIntervalMs: &lastLiq,
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
	assert.NotNil(t, got[0].PriceDelta)
	assert.Equal(t, 0.5, *got[0].PriceDelta)
	assert.NotNil(t, got[0].PriceVelocity)
	assert.Equal(t, 0.01, *got[0].PriceVelocity)
	assert.NotNil(t, got[0].PriceAcceleration)
	assert.InDelta(t, 0.001, *got[0].PriceAcceleration, 0.0001)
	assert.NotNil(t, got[0].LiquidityDelta)
	assert.Equal(t, 100.0, *got[0].LiquidityDelta)
	assert.NotNil(t, got[0].LiquidityVelocity)
	assert.Equal(t, 1.0, *got[0].LiquidityVelocity)
	assert.Equal(t, int64(5000), got[0].TokenLifetimeMs)
	assert.NotNil(t, got[0].LastSwapIntervalMs)
	assert.Equal(t, int64(500), *got[0].LastSwapIntervalMs)
	assert.NotNil(t, got[0].LastLiqEventIntervalMs)
	assert.Equal(t, int64(1000), *got[0].LastLiqEventIntervalMs)
}

func TestDerivedFeatureStore_InsertBulk_NullableFields(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Insert point with all nullable fields as nil (first row in timeseries)
	points := []*domain.DerivedFeaturePoint{
		{
			CandidateID:            "cand-1",
			TimestampMs:            1000,
			PriceDelta:             nil,
			PriceVelocity:          nil,
			PriceAcceleration:      nil,
			LiquidityDelta:         nil,
			LiquidityVelocity:      nil,
			TokenLifetimeMs:        0,
			LastSwapIntervalMs:     nil,
			LastLiqEventIntervalMs: nil,
		},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Verify insert
	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Nil(t, got[0].PriceDelta)
	assert.Nil(t, got[0].PriceVelocity)
	assert.Nil(t, got[0].PriceAcceleration)
	assert.Nil(t, got[0].LiquidityDelta)
	assert.Nil(t, got[0].LiquidityVelocity)
	assert.Nil(t, got[0].LastSwapIntervalMs)
	assert.Nil(t, got[0].LastLiqEventIntervalMs)
}

func TestDerivedFeatureStore_InsertBulk_DuplicateKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 0},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	// Try to insert duplicate
	err = store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestDerivedFeatureStore_InsertBulk_IntraBatchDuplicate(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Same key twice in one batch
	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 0},
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 1000},
	}

	err := store.InsertBulk(ctx, points)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestDerivedFeatureStore_GetByCandidateID(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Insert multiple points
	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 0},
		{CandidateID: "cand-1", TimestampMs: 2000, TokenLifetimeMs: 1000},
		{CandidateID: "cand-2", TimestampMs: 1500, TokenLifetimeMs: 0},
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

	// Get non-existent
	got, err = store.GetByCandidateID(ctx, "cand-999")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDerivedFeatureStore_GetByTimeRange(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Insert points
	points := []*domain.DerivedFeaturePoint{
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 0},
		{CandidateID: "cand-1", TimestampMs: 2000, TokenLifetimeMs: 1000},
		{CandidateID: "cand-1", TimestampMs: 3000, TokenLifetimeMs: 2000},
		{CandidateID: "cand-1", TimestampMs: 4000, TokenLifetimeMs: 3000},
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

func TestDerivedFeatureStore_FeatureProgression(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Simulate feature progression: first row has all null, then progressively filled
	priceDelta1 := 0.1
	priceVelocity1 := 0.001
	liqDelta1 := 10.0
	liqVelocity1 := 0.1
	lastSwap1 := int64(1000)

	priceDelta2 := 0.2
	priceVelocity2 := 0.002
	priceAccel2 := 0.001
	liqDelta2 := 20.0
	liqVelocity2 := 0.2
	lastSwap2 := int64(1000)
	lastLiq2 := int64(500)

	points := []*domain.DerivedFeaturePoint{
		// First row - no previous data
		{CandidateID: "cand-1", TimestampMs: 1000, TokenLifetimeMs: 0},
		// Second row - has delta/velocity
		{
			CandidateID:        "cand-1",
			TimestampMs:        2000,
			TokenLifetimeMs:    1000,
			PriceDelta:         &priceDelta1,
			PriceVelocity:      &priceVelocity1,
			LiquidityDelta:     &liqDelta1,
			LiquidityVelocity:  &liqVelocity1,
			LastSwapIntervalMs: &lastSwap1,
		},
		// Third row - has acceleration too
		{
			CandidateID:            "cand-1",
			TimestampMs:            3000,
			TokenLifetimeMs:        2000,
			PriceDelta:             &priceDelta2,
			PriceVelocity:          &priceVelocity2,
			PriceAcceleration:      &priceAccel2,
			LiquidityDelta:         &liqDelta2,
			LiquidityVelocity:      &liqVelocity2,
			LastSwapIntervalMs:     &lastSwap2,
			LastLiqEventIntervalMs: &lastLiq2,
		},
	}

	err := store.InsertBulk(ctx, points)
	require.NoError(t, err)

	got, err := store.GetByCandidateID(ctx, "cand-1")
	require.NoError(t, err)
	require.Len(t, got, 3)

	// First row - all nullable are nil
	assert.Nil(t, got[0].PriceDelta)
	assert.Nil(t, got[0].PriceAcceleration)

	// Second row - has delta/velocity but no acceleration
	assert.NotNil(t, got[1].PriceDelta)
	assert.Nil(t, got[1].PriceAcceleration)

	// Third row - has everything
	assert.NotNil(t, got[2].PriceDelta)
	assert.NotNil(t, got[2].PriceAcceleration)
	assert.NotNil(t, got[2].LastLiqEventIntervalMs)
}

func TestDerivedFeatureStore_MultipleCandidates(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewDerivedFeatureStore(conn)
	ctx := context.Background()

	// Insert many points for many candidates
	var points []*domain.DerivedFeaturePoint
	for i := 0; i < 10; i++ {
		for j := 0; j < 5; j++ {
			priceDelta := float64(i*10 + j)
			points = append(points, &domain.DerivedFeaturePoint{
				CandidateID:     fmt.Sprintf("cand-%d", i),
				TimestampMs:     int64(j * 1000),
				TokenLifetimeMs: int64(j * 1000),
				PriceDelta:      &priceDelta,
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
