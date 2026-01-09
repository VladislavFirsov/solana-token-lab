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

func TestOrchestrator_CreatesTradesFromScratch(t *testing.T) {
	ctx := context.Background()
	stores := createTestStores()

	now := time.Now().UnixMilli()
	baseTime := now - 600000 // 10 minutes ago

	// Add test candidate with NEW_TOKEN source
	candidate := &domain.TokenCandidate{
		CandidateID:  "fresh-trades-candidate",
		Source:       domain.SourceNewToken,
		Mint:         "FreshMint123",
		TxSignature:  "fresh-sig",
		EventIndex:   0,
		Slot:         1000,
		DiscoveredAt: baseTime,
		CreatedAt:    baseTime,
	}
	if err := stores.candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("insert candidate: %v", err)
	}

	// Add multiple swaps to create price timeseries
	swaps := []*domain.Swap{
		{
			CandidateID: "fresh-trades-candidate",
			TxSignature: "swap-fresh-1",
			EventIndex:  0,
			Slot:        1000,
			Timestamp:   baseTime,
			Side:        domain.SwapSideBuy,
			AmountIn:    100,
			AmountOut:   1000,
			Price:       1.0,
		},
		{
			CandidateID: "fresh-trades-candidate",
			TxSignature: "swap-fresh-2",
			EventIndex:  0,
			Slot:        1100,
			Timestamp:   baseTime + 60000, // +1 min
			Side:        domain.SwapSideBuy,
			AmountIn:    100,
			AmountOut:   900,
			Price:       1.1,
		},
		{
			CandidateID: "fresh-trades-candidate",
			TxSignature: "swap-fresh-3",
			EventIndex:  0,
			Slot:        1200,
			Timestamp:   baseTime + 120000, // +2 min
			Side:        domain.SwapSideSell,
			AmountIn:    500,
			AmountOut:   50,
			Price:       1.05,
		},
		{
			CandidateID: "fresh-trades-candidate",
			TxSignature: "swap-fresh-4",
			EventIndex:  0,
			Slot:        1500,
			Timestamp:   baseTime + 300000, // +5 min
			Side:        domain.SwapSideBuy,
			AmountIn:    100,
			AmountOut:   850,
			Price:       1.08,
		},
		{
			CandidateID: "fresh-trades-candidate",
			TxSignature: "swap-fresh-5",
			EventIndex:  0,
			Slot:        1800,
			Timestamp:   baseTime + 360000, // +6 min
			Side:        domain.SwapSideSell,
			AmountIn:    400,
			AmountOut:   45,
			Price:       1.12,
		},
	}
	if err := stores.swapStore.InsertBulk(ctx, swaps); err != nil {
		t.Fatalf("insert swaps: %v", err)
	}

	// Add liquidity events (required for liquidity timeseries normalization)
	liquidityEvents := []*domain.LiquidityEvent{
		{
			CandidateID:    "fresh-trades-candidate",
			TxSignature:    "liq-fresh-1",
			EventIndex:     0,
			Slot:           1000,
			Timestamp:      baseTime,
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    10000,
			AmountQuote:    100,
			LiquidityAfter: 10100,
		},
		{
			CandidateID:    "fresh-trades-candidate",
			TxSignature:    "liq-fresh-2",
			EventIndex:     0,
			Slot:           1500,
			Timestamp:      baseTime + 300000, // +5 min
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    5000,
			AmountQuote:    50,
			LiquidityAfter: 15150,
		},
	}
	if err := stores.liquidityEventStore.InsertBulk(ctx, liquidityEvents); err != nil {
		t.Fatalf("insert liquidity events: %v", err)
	}

	// Strategy config - TIME_EXIT with 5 min hold
	holdDuration := int64(300000) // 5 minutes
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

	// Verify trade store is empty before running
	allTrades, _ := stores.tradeRecordStore.GetAll(ctx)
	if len(allTrades) != 0 {
		t.Fatalf("expected empty trade store, got %d trades", len(allTrades))
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
		t.Errorf("expected 1 candidate processed, got %d", result.CandidatesProcessed)
	}

	// Verify no errors occurred
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got: %v", result.Errors)
	}

	// Verify trades were created (this is the main assertion)
	if result.TradesCreated == 0 {
		t.Errorf("expected TradesCreated > 0, got 0")
	}

	// Verify trade store contains the created trades
	allTrades, _ = stores.tradeRecordStore.GetAll(ctx)
	if len(allTrades) != result.TradesCreated {
		t.Errorf("trade store count (%d) doesn't match TradesCreated (%d)", len(allTrades), result.TradesCreated)
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
