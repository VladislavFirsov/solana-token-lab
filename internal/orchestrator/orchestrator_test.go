// Package orchestrator provides E2E pipeline orchestration tests.
package orchestrator

import (
	"context"
	"testing"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func TestOrchestrator_Run_EmptyCandidates(t *testing.T) {
	ctx := context.Background()
	stores := createTestStores()

	orch := New(Options{
		CandidateStore:           stores.candidateStore,
		SwapStore:                stores.swapStore,
		LiquidityEventStore:      stores.liquidityEventStore,
		PriceTimeseriesStore:     stores.priceTimeseriesStore,
		LiquidityTimeseriesStore: stores.liquidityTimeseriesStore,
		VolumeTimeseriesStore:    stores.volumeTimeseriesStore,
		DerivedFeatureStore:      stores.derivedFeatureStore,
		TradeRecordStore:         stores.tradeRecordStore,
		StrategyAggregateStore:   stores.strategyAggregateStore,
		StrategyConfigs:          []domain.StrategyConfig{},
		ScenarioConfigs:          []domain.ScenarioConfig{},
	})

	result, err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.CandidatesProcessed != 0 {
		t.Errorf("expected 0 candidates, got %d", result.CandidatesProcessed)
	}
	if result.TradesCreated != 0 {
		t.Errorf("expected 0 trades, got %d", result.TradesCreated)
	}
	if result.AggregatesCreated != 0 {
		t.Errorf("expected 0 aggregates, got %d", result.AggregatesCreated)
	}
}

func TestOrchestrator_Run_WithCandidates(t *testing.T) {
	ctx := context.Background()
	stores := createTestStores()

	now := time.Now().UnixMilli()

	// Add test candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  "test-candidate-001",
		Source:       domain.SourceNewToken,
		Mint:         "TestMint123",
		TxSignature:  "sig123",
		EventIndex:   0,
		Slot:         100,
		DiscoveredAt: now,
		CreatedAt:    now,
	}
	if err := stores.candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("insert candidate: %v", err)
	}

	// Add swap for normalization
	swap := &domain.Swap{
		CandidateID: "test-candidate-001",
		TxSignature: "swap-sig-1",
		EventIndex:  0,
		Slot:        100,
		Timestamp:   now,
		Side:        domain.SwapSideBuy,
		AmountIn:    100,
		AmountOut:   1000,
		Price:       1.0,
		CreatedAt:   now,
	}
	if err := stores.swapStore.InsertBulk(ctx, []*domain.Swap{swap}); err != nil {
		t.Fatalf("insert swap: %v", err)
	}

	// Strategy config
	holdDuration := int64(300000)
	strategyConfigs := []domain.StrategyConfig{
		{
			StrategyType:   domain.StrategyTypeTimeExit,
			EntryEventType: "NEW_TOKEN",
			HoldDurationMs: &holdDuration,
		},
	}

	scenarioConfigs := []domain.ScenarioConfig{
		domain.ScenarioConfigRealistic,
	}

	orch := New(Options{
		CandidateStore:           stores.candidateStore,
		SwapStore:                stores.swapStore,
		LiquidityEventStore:      stores.liquidityEventStore,
		PriceTimeseriesStore:     stores.priceTimeseriesStore,
		LiquidityTimeseriesStore: stores.liquidityTimeseriesStore,
		VolumeTimeseriesStore:    stores.volumeTimeseriesStore,
		DerivedFeatureStore:      stores.derivedFeatureStore,
		TradeRecordStore:         stores.tradeRecordStore,
		StrategyAggregateStore:   stores.strategyAggregateStore,
		StrategyConfigs:          strategyConfigs,
		ScenarioConfigs:          scenarioConfigs,
	})

	result, err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.CandidatesProcessed != 1 {
		t.Errorf("expected 1 candidate, got %d", result.CandidatesProcessed)
	}

	// Note: Trade creation depends on having sufficient timeseries data.
	// Full E2E testing is done via cmd/pipeline with fixtures.
	// This test verifies orchestrator runs phases without panicking.
}

func TestOrchestrator_Run_SkipNormalization(t *testing.T) {
	ctx := context.Background()
	stores := createTestStores()

	now := time.Now().UnixMilli()

	// Add test candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  "test-candidate-002",
		Source:       domain.SourceActiveToken,
		Mint:         "TestMint456",
		TxSignature:  "sig456",
		EventIndex:   0,
		Slot:         200,
		DiscoveredAt: now,
		CreatedAt:    now,
	}
	if err := stores.candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("insert candidate: %v", err)
	}

	// Pre-populate timeseries (skip normalization)
	ts := &domain.PriceTimeseriesPoint{
		CandidateID: "test-candidate-002",
		Slot:        200,
		Price:       1.0,
		Volume:      1000,
		SwapCount:   1,
		TimestampMs: now,
	}
	if err := stores.priceTimeseriesStore.InsertBulk(ctx, []*domain.PriceTimeseriesPoint{ts}); err != nil {
		t.Fatalf("save timeseries: %v", err)
	}

	orch := New(Options{
		CandidateStore:           stores.candidateStore,
		SwapStore:                stores.swapStore,
		LiquidityEventStore:      stores.liquidityEventStore,
		PriceTimeseriesStore:     stores.priceTimeseriesStore,
		LiquidityTimeseriesStore: stores.liquidityTimeseriesStore,
		VolumeTimeseriesStore:    stores.volumeTimeseriesStore,
		DerivedFeatureStore:      stores.derivedFeatureStore,
		TradeRecordStore:         stores.tradeRecordStore,
		StrategyAggregateStore:   stores.strategyAggregateStore,
		StrategyConfigs:          []domain.StrategyConfig{},
		ScenarioConfigs:          []domain.ScenarioConfig{},
		SkipNormalization:        true,
	})

	result, err := orch.Run(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.CandidatesProcessed != 1 {
		t.Errorf("expected 1 candidate, got %d", result.CandidatesProcessed)
	}
}

func TestSourceMatches(t *testing.T) {
	tests := []struct {
		source    domain.Source
		entryType string
		expected  bool
	}{
		{domain.SourceNewToken, "NEW_TOKEN", true},
		{domain.SourceNewToken, "ACTIVE_TOKEN", false},
		{domain.SourceActiveToken, "ACTIVE_TOKEN", true},
		{domain.SourceActiveToken, "NEW_TOKEN", false},
		{domain.SourceNewToken, "UNKNOWN", false},
	}

	for _, tt := range tests {
		got := sourceMatches(tt.source, tt.entryType)
		if got != tt.expected {
			t.Errorf("sourceMatches(%v, %q) = %v, want %v",
				tt.source, tt.entryType, got, tt.expected)
		}
	}
}

// testStores holds all memory stores for testing.
type testStores struct {
	candidateStore           *memory.CandidateStore
	swapStore                *memory.SwapStore
	liquidityEventStore      *memory.LiquidityEventStore
	priceTimeseriesStore     *memory.PriceTimeseriesStore
	liquidityTimeseriesStore *memory.LiquidityTimeseriesStore
	volumeTimeseriesStore    *memory.VolumeTimeseriesStore
	derivedFeatureStore      *memory.DerivedFeatureStore
	tradeRecordStore         *memory.TradeRecordStore
	strategyAggregateStore   *memory.StrategyAggregateStore
}

func createTestStores() *testStores {
	return &testStores{
		candidateStore:           memory.NewCandidateStore(),
		swapStore:                memory.NewSwapStore(),
		liquidityEventStore:      memory.NewLiquidityEventStore(),
		priceTimeseriesStore:     memory.NewPriceTimeseriesStore(),
		liquidityTimeseriesStore: memory.NewLiquidityTimeseriesStore(),
		volumeTimeseriesStore:    memory.NewVolumeTimeseriesStore(),
		derivedFeatureStore:      memory.NewDerivedFeatureStore(),
		tradeRecordStore:         memory.NewTradeRecordStore(),
		strategyAggregateStore:   memory.NewStrategyAggregateStore(),
	}
}
