package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func createTestTradeRecord(candidateID, tradeID, strategyID, scenarioID string) *domain.TradeRecord {
	return &domain.TradeRecord{
		TradeID:          tradeID,
		CandidateID:      candidateID,
		StrategyID:       strategyID,
		ScenarioID:       scenarioID,
		EntrySignalTime:  1000,
		EntrySignalPrice: 0.01,
		EntryActualTime:  1100,
		EntryActualPrice: 0.0105,
		EntryLiquidity:   ptr(1000.0),
		PositionSize:     1.0,
		PositionValue:    0.0105,
		ExitSignalTime:   2000,
		ExitSignalPrice:  0.02,
		ExitActualTime:   2100,
		ExitActualPrice:  0.019,
		ExitReason:       "TIME_EXIT",
		EntryCostSOL:     0.0001,
		ExitCostSOL:      0.0001,
		MEVCostSOL:       0.0,
		TotalCostSOL:     0.0002,
		TotalCostPct:     0.019,
		GrossReturn:      0.81,
		Outcome:          0.79,
		OutcomeClass:     "WIN",
		HoldDurationMs:   1100,
		PeakPrice:        ptr(0.025),
		MinLiquidity:     ptr(950.0),
	}
}

func TestTradeRecordStore_InsertAndGetByID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-test-candidate-1")

	store := NewTradeRecordStore(pool)

	trade := createTestTradeRecord(candidateID, "trade-001", "TIME_EXIT_5min", "REALISTIC")

	// Insert
	err := store.Insert(ctx, trade)
	require.NoError(t, err)

	// GetByID
	retrieved, err := store.GetByID(ctx, "trade-001")
	require.NoError(t, err)

	assert.Equal(t, trade.TradeID, retrieved.TradeID)
	assert.Equal(t, trade.CandidateID, retrieved.CandidateID)
	assert.Equal(t, trade.StrategyID, retrieved.StrategyID)
	assert.Equal(t, trade.ScenarioID, retrieved.ScenarioID)
	assert.Equal(t, trade.EntrySignalTime, retrieved.EntrySignalTime)
	assert.InDelta(t, trade.EntrySignalPrice, retrieved.EntrySignalPrice, 0.0001)
	assert.Equal(t, trade.EntryActualTime, retrieved.EntryActualTime)
	assert.InDelta(t, trade.EntryActualPrice, retrieved.EntryActualPrice, 0.0001)
	assert.NotNil(t, retrieved.EntryLiquidity)
	assert.InDelta(t, *trade.EntryLiquidity, *retrieved.EntryLiquidity, 0.0001)
	assert.InDelta(t, trade.PositionSize, retrieved.PositionSize, 0.0001)
	assert.InDelta(t, trade.PositionValue, retrieved.PositionValue, 0.0001)
	assert.Equal(t, trade.ExitSignalTime, retrieved.ExitSignalTime)
	assert.InDelta(t, trade.ExitSignalPrice, retrieved.ExitSignalPrice, 0.0001)
	assert.Equal(t, trade.ExitActualTime, retrieved.ExitActualTime)
	assert.InDelta(t, trade.ExitActualPrice, retrieved.ExitActualPrice, 0.0001)
	assert.Equal(t, trade.ExitReason, retrieved.ExitReason)
	assert.InDelta(t, trade.EntryCostSOL, retrieved.EntryCostSOL, 0.0001)
	assert.InDelta(t, trade.ExitCostSOL, retrieved.ExitCostSOL, 0.0001)
	assert.InDelta(t, trade.MEVCostSOL, retrieved.MEVCostSOL, 0.0001)
	assert.InDelta(t, trade.TotalCostSOL, retrieved.TotalCostSOL, 0.0001)
	assert.InDelta(t, trade.TotalCostPct, retrieved.TotalCostPct, 0.0001)
	assert.InDelta(t, trade.GrossReturn, retrieved.GrossReturn, 0.0001)
	assert.InDelta(t, trade.Outcome, retrieved.Outcome, 0.0001)
	assert.Equal(t, trade.OutcomeClass, retrieved.OutcomeClass)
	assert.Equal(t, trade.HoldDurationMs, retrieved.HoldDurationMs)
	assert.NotNil(t, retrieved.PeakPrice)
	assert.InDelta(t, *trade.PeakPrice, *retrieved.PeakPrice, 0.0001)
	assert.NotNil(t, retrieved.MinLiquidity)
	assert.InDelta(t, *trade.MinLiquidity, *retrieved.MinLiquidity, 0.0001)
}

func TestTradeRecordStore_InsertDuplicate(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-dup-candidate")

	store := NewTradeRecordStore(pool)

	trade := createTestTradeRecord(candidateID, "trade-dup-001", "TIME_EXIT_5min", "REALISTIC")

	// First insert should succeed
	err := store.Insert(ctx, trade)
	require.NoError(t, err)

	// Second insert with same trade_id should fail
	err = store.Insert(ctx, trade)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestTradeRecordStore_GetByIDNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewTradeRecordStore(pool)

	_, err := store.GetByID(ctx, "nonexistent-trade")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestTradeRecordStore_InsertBulk(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-bulk-candidate")

	store := NewTradeRecordStore(pool)

	trades := []*domain.TradeRecord{
		createTestTradeRecord(candidateID, "bulk-trade-001", "TIME_EXIT_5min", "REALISTIC"),
		createTestTradeRecord(candidateID, "bulk-trade-002", "TIME_EXIT_5min", "PESSIMISTIC"),
		createTestTradeRecord(candidateID, "bulk-trade-003", "TRAILING_STOP", "REALISTIC"),
	}

	// InsertBulk
	err := store.InsertBulk(ctx, trades)
	require.NoError(t, err)

	// Verify all inserted
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestTradeRecordStore_InsertBulkAtomic(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-atomic-candidate")

	store := NewTradeRecordStore(pool)

	// First batch succeeds
	firstBatch := []*domain.TradeRecord{
		createTestTradeRecord(candidateID, "atomic-trade-001", "TIME_EXIT_5min", "REALISTIC"),
	}

	err := store.InsertBulk(ctx, firstBatch)
	require.NoError(t, err)

	// Second batch has duplicate - should fail entirely
	secondBatch := []*domain.TradeRecord{
		createTestTradeRecord(candidateID, "atomic-trade-002", "TIME_EXIT_5min", "PESSIMISTIC"),
		createTestTradeRecord(candidateID, "atomic-trade-001", "TIME_EXIT_5min", "REALISTIC"), // duplicate!
	}

	err = store.InsertBulk(ctx, secondBatch)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)

	// Should still have only 1 trade (atomic rollback)
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestTradeRecordStore_InsertBulkEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewTradeRecordStore(pool)

	// Empty bulk should succeed (no-op)
	err := store.InsertBulk(ctx, []*domain.TradeRecord{})
	require.NoError(t, err)
}

func TestTradeRecordStore_GetByCandidateID(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID1 := createTestCandidate(t, ctx, pool, "trade-bycandidate-1")
	candidateID2 := createTestCandidate(t, ctx, pool, "trade-bycandidate-2")

	store := NewTradeRecordStore(pool)

	trades := []*domain.TradeRecord{
		createTestTradeRecord(candidateID1, "candidate-trade-001", "TIME_EXIT", "REALISTIC"),
		createTestTradeRecord(candidateID1, "candidate-trade-002", "TRAILING_STOP", "REALISTIC"),
		createTestTradeRecord(candidateID2, "candidate-trade-003", "TIME_EXIT", "REALISTIC"),
	}

	err := store.InsertBulk(ctx, trades)
	require.NoError(t, err)

	// GetByCandidateID should return only trades for candidate1
	result, err := store.GetByCandidateID(ctx, candidateID1)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	for _, tr := range result {
		assert.Equal(t, candidateID1, tr.CandidateID)
	}
}

func TestTradeRecordStore_GetByStrategyScenario(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID1 := createTestCandidate(t, ctx, pool, "trade-strategy-1")
	candidateID2 := createTestCandidate(t, ctx, pool, "trade-strategy-2")

	store := NewTradeRecordStore(pool)

	trades := []*domain.TradeRecord{
		createTestTradeRecord(candidateID1, "strategy-trade-001", "TIME_EXIT_5min", "REALISTIC"),
		createTestTradeRecord(candidateID1, "strategy-trade-002", "TIME_EXIT_5min", "PESSIMISTIC"),
		createTestTradeRecord(candidateID2, "strategy-trade-003", "TIME_EXIT_5min", "REALISTIC"),
		createTestTradeRecord(candidateID1, "strategy-trade-004", "TRAILING_STOP", "REALISTIC"),
	}

	err := store.InsertBulk(ctx, trades)
	require.NoError(t, err)

	// GetByStrategyScenario TIME_EXIT_5min + REALISTIC should return 2 trades
	result, err := store.GetByStrategyScenario(ctx, "TIME_EXIT_5min", "REALISTIC")
	require.NoError(t, err)

	assert.Len(t, result, 2)
	for _, tr := range result {
		assert.Equal(t, "TIME_EXIT_5min", tr.StrategyID)
		assert.Equal(t, "REALISTIC", tr.ScenarioID)
	}
}

func TestTradeRecordStore_GetAll(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-all-candidate")

	store := NewTradeRecordStore(pool)

	trades := []*domain.TradeRecord{
		createTestTradeRecord(candidateID, "all-trade-001", "TIME_EXIT", "REALISTIC"),
		createTestTradeRecord(candidateID, "all-trade-002", "TRAILING_STOP", "REALISTIC"),
		createTestTradeRecord(candidateID, "all-trade-003", "LIQUIDITY_GUARD", "PESSIMISTIC"),
	}

	err := store.InsertBulk(ctx, trades)
	require.NoError(t, err)

	// GetAll should return all trades
	result, err := store.GetAll(ctx)
	require.NoError(t, err)

	assert.Len(t, result, 3)
}

func TestTradeRecordStore_Ordering(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-order-candidate")

	store := NewTradeRecordStore(pool)

	// Insert trades with different entry times
	trade1 := createTestTradeRecord(candidateID, "order-trade-003", "TIME_EXIT", "REALISTIC")
	trade1.EntrySignalTime = 3000

	trade2 := createTestTradeRecord(candidateID, "order-trade-001", "TIME_EXIT", "REALISTIC")
	trade2.EntrySignalTime = 1000

	trade3 := createTestTradeRecord(candidateID, "order-trade-002", "TIME_EXIT", "REALISTIC")
	trade3.EntrySignalTime = 2000

	// Insert in reverse order
	for _, tr := range []*domain.TradeRecord{trade1, trade2, trade3} {
		err := store.Insert(ctx, tr)
		require.NoError(t, err)
	}

	// Results should be ordered by entry_signal_time ASC
	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, result, 3)
	assert.Equal(t, int64(1000), result[0].EntrySignalTime)
	assert.Equal(t, int64(2000), result[1].EntrySignalTime)
	assert.Equal(t, int64(3000), result[2].EntrySignalTime)
}

func TestTradeRecordStore_EmptyResult(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewTradeRecordStore(pool)

	// GetByCandidateID with no matching records
	result, err := store.GetByCandidateID(ctx, "nonexistent-candidate")
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetByStrategyScenario with no matching records
	result, err = store.GetByStrategyScenario(ctx, "NONEXISTENT", "NONEXISTENT")
	require.NoError(t, err)
	assert.Empty(t, result)

	// GetAll with empty database
	result, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestTradeRecordStore_NullableFields(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-nullable-candidate")

	store := NewTradeRecordStore(pool)

	trade := createTestTradeRecord(candidateID, "nullable-trade-001", "TIME_EXIT", "REALISTIC")
	trade.EntryLiquidity = nil
	trade.PeakPrice = nil
	trade.MinLiquidity = nil

	err := store.Insert(ctx, trade)
	require.NoError(t, err)

	retrieved, err := store.GetByID(ctx, "nullable-trade-001")
	require.NoError(t, err)

	assert.Nil(t, retrieved.EntryLiquidity)
	assert.Nil(t, retrieved.PeakPrice)
	assert.Nil(t, retrieved.MinLiquidity)
}

func TestTradeRecordStore_OutcomeClasses(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	candidateID := createTestCandidate(t, ctx, pool, "trade-outcome-candidate")

	store := NewTradeRecordStore(pool)

	// WIN trade
	winTrade := createTestTradeRecord(candidateID, "outcome-win-001", "TIME_EXIT", "REALISTIC")
	winTrade.Outcome = 0.5
	winTrade.OutcomeClass = "WIN"

	err := store.Insert(ctx, winTrade)
	require.NoError(t, err)

	// LOSS trade
	lossTrade := createTestTradeRecord(candidateID, "outcome-loss-001", "TIME_EXIT", "REALISTIC")
	lossTrade.Outcome = -0.3
	lossTrade.OutcomeClass = "LOSS"

	err = store.Insert(ctx, lossTrade)
	require.NoError(t, err)

	result, err := store.GetByCandidateID(ctx, candidateID)
	require.NoError(t, err)

	assert.Len(t, result, 2)
	winFound := false
	lossFound := false
	for _, tr := range result {
		if tr.OutcomeClass == "WIN" {
			winFound = true
			assert.Greater(t, tr.Outcome, 0.0)
		}
		if tr.OutcomeClass == "LOSS" {
			lossFound = true
			assert.Less(t, tr.Outcome, 0.0)
		}
	}
	assert.True(t, winFound, "WIN trade not found")
	assert.True(t, lossFound, "LOSS trade not found")
}
