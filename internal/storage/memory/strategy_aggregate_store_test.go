package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestStrategyAggregateStore_InsertAndGet(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	agg := &domain.StrategyAggregate{
		StrategyID:     "TIME_EXIT_300",
		ScenarioID:     "realistic",
		EntryEventType: "NEW_TOKEN",
		TotalTrades:    100,
		Wins:           60,
		Losses:         40,
		WinRate:        0.6,
		OutcomeMedian:  0.05,
	}

	err := store.Insert(ctx, agg)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	got, err := store.GetByKey(ctx, "TIME_EXIT_300", "realistic", "NEW_TOKEN")
	if err != nil {
		t.Fatalf("GetByKey failed: %v", err)
	}

	if got.WinRate != 0.6 {
		t.Errorf("WinRate mismatch: got %f, want %f", got.WinRate, 0.6)
	}
}

func TestStrategyAggregateStore_DuplicateKey(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	agg := &domain.StrategyAggregate{
		StrategyID:     "TIME_EXIT",
		ScenarioID:     "realistic",
		EntryEventType: "NEW_TOKEN",
	}

	if err := store.Insert(ctx, agg); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.Insert(ctx, agg)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestStrategyAggregateStore_NotFound(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	_, err := store.GetByKey(ctx, "nonexistent", "realistic", "NEW_TOKEN")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestStrategyAggregateStore_InsertBulk(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	aggs := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"},
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "ACTIVE_TOKEN"},
		{StrategyID: "TIME_EXIT", ScenarioID: "pessimistic", EntryEventType: "NEW_TOKEN"},
	}

	err := store.InsertBulk(ctx, aggs)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByStrategy(ctx, "TIME_EXIT")
	if len(result) != 3 {
		t.Errorf("Expected 3 aggregates, got %d", len(result))
	}
}

func TestStrategyAggregateStore_InsertBulkPartialDuplicate(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	// Insert first
	first := &domain.StrategyAggregate{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"}
	if err := store.Insert(ctx, first); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Bulk with duplicate
	aggs := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "ACTIVE_TOKEN"},
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"}, // duplicate
	}

	err := store.InsertBulk(ctx, aggs)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}

	// Verify no partial insert
	all, _ := store.GetAll(ctx)
	if len(all) != 1 {
		t.Errorf("Expected 1 aggregate (no partial insert), got %d", len(all))
	}
}

func TestStrategyAggregateStore_GetByStrategy(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	aggs := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"},
		{StrategyID: "TRAILING_STOP", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"},
		{StrategyID: "TIME_EXIT", ScenarioID: "pessimistic", EntryEventType: "NEW_TOKEN"},
	}

	if err := store.InsertBulk(ctx, aggs); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByStrategy(ctx, "TIME_EXIT")
	if err != nil {
		t.Fatalf("GetByStrategy failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 aggregates for TIME_EXIT, got %d", len(result))
	}
}

func TestStrategyAggregateStore_GetAll(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	aggs := []*domain.StrategyAggregate{
		{StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"},
		{StrategyID: "TRAILING_STOP", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN"},
		{StrategyID: "LIQUIDITY_GUARD", ScenarioID: "pessimistic", EntryEventType: "ACTIVE_TOKEN"},
	}

	if err := store.InsertBulk(ctx, aggs); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 aggregates, got %d", len(result))
	}

	// Should be sorted by strategy, scenario, entry event type
	// LIQUIDITY_GUARD < TIME_EXIT < TRAILING_STOP
	if result[0].StrategyID != "LIQUIDITY_GUARD" {
		t.Errorf("First should be LIQUIDITY_GUARD, got %s", result[0].StrategyID)
	}
}

func TestStrategyAggregateStore_InvalidInput(t *testing.T) {
	store := NewStrategyAggregateStore()
	ctx := context.Background()

	err := store.Insert(ctx, nil)
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil, got %v", err)
	}

	err = store.Insert(ctx, &domain.StrategyAggregate{StrategyID: "", ScenarioID: "r", EntryEventType: "n"})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty strategy, got %v", err)
	}

	err = store.Insert(ctx, &domain.StrategyAggregate{StrategyID: "s", ScenarioID: "", EntryEventType: "n"})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty scenario, got %v", err)
	}

	err = store.Insert(ctx, &domain.StrategyAggregate{StrategyID: "s", ScenarioID: "r", EntryEventType: ""})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty entry event type, got %v", err)
	}
}
