package clickhouse

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestStrategyAggregateStore_Insert(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	outcomeRealistic := 0.15
	outcomePessimistic := 0.05
	outcomeDegraded := -0.02

	agg := &domain.StrategyAggregate{
		StrategyID:           "TIME_EXIT_5M",
		ScenarioID:           "OPTIMISTIC",
		EntryEventType:       "NEW_TOKEN",
		TotalTrades:          100,
		TotalTokens:          50,
		Wins:                 60,
		Losses:               40,
		WinRate:              0.6,
		TokenWinRate:         0.55,
		OutcomeMean:          0.12,
		OutcomeMedian:        0.10,
		OutcomeP10:           -0.15,
		OutcomeP25:           0.02,
		OutcomeP75:           0.20,
		OutcomeP90:           0.35,
		OutcomeMin:           -0.50,
		OutcomeMax:           1.5,
		OutcomeStddev:        0.25,
		MaxDrawdown:          -0.30,
		MaxConsecutiveLosses: 5,
		OutcomeRealistic:     &outcomeRealistic,
		OutcomePessimistic:   &outcomePessimistic,
		OutcomeDegraded:      &outcomeDegraded,
	}

	err := store.Insert(ctx, agg)
	require.NoError(t, err)

	// Verify insert
	got, err := store.GetByKey(ctx, "TIME_EXIT_5M", "OPTIMISTIC", "NEW_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "TIME_EXIT_5M", got.StrategyID)
	assert.Equal(t, "OPTIMISTIC", got.ScenarioID)
	assert.Equal(t, "NEW_TOKEN", got.EntryEventType)
	assert.Equal(t, 100, got.TotalTrades)
	assert.Equal(t, 50, got.TotalTokens)
	assert.Equal(t, 60, got.Wins)
	assert.Equal(t, 40, got.Losses)
	assert.Equal(t, 0.6, got.WinRate)
	assert.Equal(t, 0.55, got.TokenWinRate)
	assert.Equal(t, 0.12, got.OutcomeMean)
	assert.Equal(t, 0.10, got.OutcomeMedian)
	assert.Equal(t, -0.15, got.OutcomeP10)
	assert.Equal(t, 0.02, got.OutcomeP25)
	assert.Equal(t, 0.20, got.OutcomeP75)
	assert.Equal(t, 0.35, got.OutcomeP90)
	assert.Equal(t, -0.50, got.OutcomeMin)
	assert.Equal(t, 1.5, got.OutcomeMax)
	assert.Equal(t, 0.25, got.OutcomeStddev)
	assert.Equal(t, -0.30, got.MaxDrawdown)
	assert.Equal(t, 5, got.MaxConsecutiveLosses)
	assert.NotNil(t, got.OutcomeRealistic)
	assert.Equal(t, 0.15, *got.OutcomeRealistic)
	assert.NotNil(t, got.OutcomePessimistic)
	assert.Equal(t, 0.05, *got.OutcomePessimistic)
	assert.NotNil(t, got.OutcomeDegraded)
	assert.Equal(t, -0.02, *got.OutcomeDegraded)
}

func TestStrategyAggregateStore_Insert_DuplicateKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	agg := &domain.StrategyAggregate{
		StrategyID:     "TIME_EXIT_5M",
		ScenarioID:     "OPTIMISTIC",
		EntryEventType: "NEW_TOKEN",
		TotalTrades:    100,
		TotalTokens:    50,
		Wins:           60,
		Losses:         40,
		WinRate:        0.6,
		TokenWinRate:   0.55,
	}

	err := store.Insert(ctx, agg)
	require.NoError(t, err)

	// Try to insert duplicate
	err = store.Insert(ctx, agg)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestStrategyAggregateStore_InsertBulk(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Test empty insert
	err := store.InsertBulk(ctx, nil)
	assert.NoError(t, err)

	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100, Wins: 60, Losses: 40, WinRate: 0.6},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "REALISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100, Wins: 55, Losses: 45, WinRate: 0.55},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "PESSIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100, Wins: 50, Losses: 50, WinRate: 0.50},
	}

	err = store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// Verify all inserted
	got, err := store.GetByStrategy(ctx, "TIME_EXIT_5M")
	require.NoError(t, err)
	assert.Len(t, got, 3)
}

func TestStrategyAggregateStore_InsertBulk_DuplicateKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100, Wins: 60, Losses: 40, WinRate: 0.6},
	}

	err := store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// Try to insert duplicate
	err = store.InsertBulk(ctx, aggregates)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestStrategyAggregateStore_InsertBulk_IntraBatchDuplicate(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Same key twice in one batch
	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100, Wins: 60, Losses: 40, WinRate: 0.6},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 200, Wins: 120, Losses: 80, WinRate: 0.6},
	}

	err := store.InsertBulk(ctx, aggregates)
	assert.ErrorIs(t, err, storage.ErrDuplicateKey)
}

func TestStrategyAggregateStore_GetByKey(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Insert
	agg := &domain.StrategyAggregate{
		StrategyID:     "TIME_EXIT_5M",
		ScenarioID:     "OPTIMISTIC",
		EntryEventType: "NEW_TOKEN",
		TotalTrades:    100,
	}
	err := store.Insert(ctx, agg)
	require.NoError(t, err)

	// Get existing
	got, err := store.GetByKey(ctx, "TIME_EXIT_5M", "OPTIMISTIC", "NEW_TOKEN")
	require.NoError(t, err)
	assert.Equal(t, "TIME_EXIT_5M", got.StrategyID)

	// Get non-existent
	_, err = store.GetByKey(ctx, "NON_EXISTENT", "OPTIMISTIC", "NEW_TOKEN")
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestStrategyAggregateStore_GetByStrategy(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Insert multiple aggregates for different strategies
	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "REALISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "ACTIVE_TOKEN", TotalTrades: 100},
		{StrategyID: "TRAILING_STOP", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
	}

	err := store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// Get TIME_EXIT_5M strategy
	got, err := store.GetByStrategy(ctx, "TIME_EXIT_5M")
	require.NoError(t, err)
	assert.Len(t, got, 3)

	// Verify all are TIME_EXIT_5M
	for _, a := range got {
		assert.Equal(t, "TIME_EXIT_5M", a.StrategyID)
	}

	// Get TRAILING_STOP strategy
	got, err = store.GetByStrategy(ctx, "TRAILING_STOP")
	require.NoError(t, err)
	assert.Len(t, got, 1)

	// Get non-existent strategy
	got, err = store.GetByStrategy(ctx, "NON_EXISTENT")
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestStrategyAggregateStore_GetAll(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Empty at first
	got, err := store.GetAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)

	// Insert multiple aggregates
	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "TIME_EXIT_5M", ScenarioID: "REALISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "TRAILING_STOP", ScenarioID: "OPTIMISTIC", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "LIQUIDITY_GUARD", ScenarioID: "PESSIMISTIC", EntryEventType: "ACTIVE_TOKEN", TotalTrades: 100},
	}

	err = store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// Get all
	got, err = store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 4)
}

func TestStrategyAggregateStore_NullableFields(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Insert without sensitivity fields (they are nullable)
	agg := &domain.StrategyAggregate{
		StrategyID:           "TIME_EXIT_5M",
		ScenarioID:           "OPTIMISTIC",
		EntryEventType:       "NEW_TOKEN",
		TotalTrades:          100,
		OutcomeRealistic:     nil,
		OutcomePessimistic:   nil,
		OutcomeDegraded:      nil,
	}

	err := store.Insert(ctx, agg)
	require.NoError(t, err)

	got, err := store.GetByKey(ctx, "TIME_EXIT_5M", "OPTIMISTIC", "NEW_TOKEN")
	require.NoError(t, err)
	assert.Nil(t, got.OutcomeRealistic)
	assert.Nil(t, got.OutcomePessimistic)
	assert.Nil(t, got.OutcomeDegraded)
}

func TestStrategyAggregateStore_AllStrategiesAndScenarios(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	strategies := []string{"TIME_EXIT_5M", "TIME_EXIT_15M", "TIME_EXIT_30M", "TRAILING_STOP_5", "TRAILING_STOP_10", "LIQUIDITY_GUARD"}
	scenarios := []string{"OPTIMISTIC", "REALISTIC", "PESSIMISTIC", "DEGRADED"}
	entryTypes := []string{"NEW_TOKEN", "ACTIVE_TOKEN"}

	var aggregates []*domain.StrategyAggregate
	for _, s := range strategies {
		for _, sc := range scenarios {
			for _, et := range entryTypes {
				aggregates = append(aggregates, &domain.StrategyAggregate{
					StrategyID:     s,
					ScenarioID:     sc,
					EntryEventType: et,
					TotalTrades:    100,
					WinRate:        0.5,
				})
			}
		}
	}

	err := store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// Verify total count
	got, err := store.GetAll(ctx)
	require.NoError(t, err)
	assert.Len(t, got, 6*4*2) // 48 total

	// Verify each strategy has correct count
	for _, s := range strategies {
		got, err := store.GetByStrategy(ctx, s)
		require.NoError(t, err)
		assert.Len(t, got, 4*2) // 4 scenarios * 2 entry types
	}
}

func TestStrategyAggregateStore_Ordering(t *testing.T) {
	conn, cleanup := setupTestDB(t)
	defer cleanup()

	store := NewStrategyAggregateStore(conn)
	ctx := context.Background()

	// Insert in reverse order
	aggregates := []*domain.StrategyAggregate{
		{StrategyID: "Z_STRATEGY", ScenarioID: "Z_SCENARIO", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "A_STRATEGY", ScenarioID: "A_SCENARIO", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
		{StrategyID: "A_STRATEGY", ScenarioID: "Z_SCENARIO", EntryEventType: "NEW_TOKEN", TotalTrades: 100},
	}

	err := store.InsertBulk(ctx, aggregates)
	require.NoError(t, err)

	// GetAll should return ordered by strategy_id, scenario_id, entry_event_type
	got, err := store.GetAll(ctx)
	require.NoError(t, err)
	require.Len(t, got, 3)

	assert.Equal(t, "A_STRATEGY", got[0].StrategyID)
	assert.Equal(t, "A_SCENARIO", got[0].ScenarioID)
	assert.Equal(t, "A_STRATEGY", got[1].StrategyID)
	assert.Equal(t, "Z_SCENARIO", got[1].ScenarioID)
	assert.Equal(t, "Z_STRATEGY", got[2].StrategyID)
}
