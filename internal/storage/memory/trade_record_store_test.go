package memory

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

func TestTradeRecordStore_InsertAndGet(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	trade := &domain.TradeRecord{
		TradeID:         "trade1",
		CandidateID:     "cand1",
		StrategyID:      "TIME_EXIT_300",
		ScenarioID:      "realistic",
		EntrySignalTime: 1000,
		Outcome:         0.05,
		OutcomeClass:    domain.OutcomeClassWin,
	}

	err := store.Insert(ctx, trade)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	got, err := store.GetByID(ctx, "trade1")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if got.Outcome != 0.05 {
		t.Errorf("Outcome mismatch: got %f, want %f", got.Outcome, 0.05)
	}
}

func TestTradeRecordStore_DuplicateKey(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	trade := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "cand1",
		StrategyID:  "strat1",
		ScenarioID:  "realistic",
	}

	if err := store.Insert(ctx, trade); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	err := store.Insert(ctx, trade)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}
}

func TestTradeRecordStore_NotFound(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestTradeRecordStore_InsertBulk(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "c1", StrategyID: "s1", ScenarioID: "realistic", EntrySignalTime: 1000},
		{TradeID: "t2", CandidateID: "c1", StrategyID: "s1", ScenarioID: "realistic", EntrySignalTime: 2000},
		{TradeID: "t3", CandidateID: "c2", StrategyID: "s1", ScenarioID: "realistic", EntrySignalTime: 3000},
	}

	err := store.InsertBulk(ctx, trades)
	if err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, _ := store.GetByCandidateID(ctx, "c1")
	if len(result) != 2 {
		t.Errorf("Expected 2 trades for c1, got %d", len(result))
	}
}

func TestTradeRecordStore_InsertBulkPartialDuplicate(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	// Insert first
	first := &domain.TradeRecord{TradeID: "t1", CandidateID: "c1", StrategyID: "s1", ScenarioID: "realistic"}
	if err := store.Insert(ctx, first); err != nil {
		t.Fatalf("First insert failed: %v", err)
	}

	// Bulk with duplicate
	trades := []*domain.TradeRecord{
		{TradeID: "t2", CandidateID: "c1", StrategyID: "s1", ScenarioID: "realistic"},
		{TradeID: "t1", CandidateID: "c1", StrategyID: "s1", ScenarioID: "realistic"}, // duplicate
	}

	err := store.InsertBulk(ctx, trades)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("Expected ErrDuplicateKey, got %v", err)
	}

	// Verify all-or-nothing
	all, _ := store.GetByCandidateID(ctx, "c1")
	if len(all) != 1 {
		t.Errorf("Expected 1 trade (no partial insert), got %d", len(all))
	}
}

func TestTradeRecordStore_GetByStrategyScenario(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "c1", StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntrySignalTime: 1000},
		{TradeID: "t2", CandidateID: "c2", StrategyID: "TIME_EXIT", ScenarioID: "realistic", EntrySignalTime: 2000},
		{TradeID: "t3", CandidateID: "c3", StrategyID: "TIME_EXIT", ScenarioID: "pessimistic", EntrySignalTime: 3000},
		{TradeID: "t4", CandidateID: "c4", StrategyID: "TRAILING_STOP", ScenarioID: "realistic", EntrySignalTime: 4000},
	}

	if err := store.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	result, err := store.GetByStrategyScenario(ctx, "TIME_EXIT", "realistic")
	if err != nil {
		t.Fatalf("GetByStrategyScenario failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 trades, got %d", len(result))
	}

	// Should be ordered by entry_signal_time
	if result[0].EntrySignalTime > result[1].EntrySignalTime {
		t.Error("Results not ordered by entry_signal_time")
	}
}

func TestTradeRecordStore_InvalidInput(t *testing.T) {
	store := NewTradeRecordStore()
	ctx := context.Background()

	err := store.Insert(ctx, nil)
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for nil, got %v", err)
	}

	err = store.Insert(ctx, &domain.TradeRecord{TradeID: ""})
	if !errors.Is(err, storage.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for empty ID, got %v", err)
	}
}
