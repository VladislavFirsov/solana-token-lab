package simulation

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
)

// Helper to create test price timeseries
func makePriceTimeseries(candidateID string, prices []float64, startMs, intervalMs int64) []*domain.PriceTimeseriesPoint {
	result := make([]*domain.PriceTimeseriesPoint, len(prices))
	for i, p := range prices {
		result[i] = &domain.PriceTimeseriesPoint{
			CandidateID: candidateID,
			TimestampMs: startMs + int64(i)*intervalMs,
			Slot:        int64(100 + i),
			Price:       p,
		}
	}
	return result
}

// Helper to create test liquidity timeseries
func makeLiquidityTimeseries(candidateID string, liquidity []float64, startMs, intervalMs int64) []*domain.LiquidityTimeseriesPoint {
	result := make([]*domain.LiquidityTimeseriesPoint, len(liquidity))
	for i, l := range liquidity {
		result[i] = &domain.LiquidityTimeseriesPoint{
			CandidateID: candidateID,
			TimestampMs: startMs + int64(i)*intervalMs,
			Slot:        int64(100 + i),
			Liquidity:   l,
		}
	}
	return result
}

func ptrInt64(i int64) *int64 {
	return &i
}

func TestRunner_Run_Deterministic(t *testing.T) {
	ctx := context.Background()
	candidateID := "test-candidate-1"

	// Run multiple times, verify same output
	for run := 0; run < 5; run++ {
		candidateStore := memory.NewCandidateStore()
		priceStore := memory.NewPriceTimeseriesStore()
		liqStore := memory.NewLiquidityTimeseriesStore()
		tradeStore := memory.NewTradeRecordStore()

		// Insert candidate
		candidate := &domain.TokenCandidate{
			CandidateID:  candidateID,
			Source:       domain.SourceNewToken,
			Mint:         "mint1",
			TxSignature:  "tx1",
			Slot:         100,
			DiscoveredAt: 1000000,
		}
		if err := candidateStore.Insert(ctx, candidate); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}

		// Insert price data
		prices := makePriceTimeseries(candidateID, []float64{1.0, 1.1, 1.2, 1.15}, 1000000, 30000)
		if err := priceStore.InsertBulk(ctx, prices); err != nil {
			t.Fatalf("Insert prices failed: %v", err)
		}

		// Insert liquidity data
		liq := makeLiquidityTimeseries(candidateID, []float64{1000, 950, 900}, 1000000, 60000)
		if err := liqStore.InsertBulk(ctx, liq); err != nil {
			t.Fatalf("Insert liquidity failed: %v", err)
		}

		runner := NewRunner(RunnerOptions{
			CandidateStore:       candidateStore,
			PriceTimeseriesStore: priceStore,
			LiqTimeseriesStore:   liqStore,
			TradeRecordStore:     tradeStore,
		})

		cfg := domain.StrategyConfig{
			StrategyType:   domain.StrategyTypeTimeExit,
			EntryEventType: "NEW_TOKEN",
			HoldDurationMs: ptrInt64(60000),
		}

		trade, err := runner.Run(ctx, candidateID, cfg, domain.ScenarioConfigRealistic)
		if err != nil {
			t.Fatalf("Run %d: Run failed: %v", run, err)
		}

		// Verify deterministic values
		if trade.EntrySignalTime != 1000000 {
			t.Errorf("Run %d: expected entry time 1000000, got %d", run, trade.EntrySignalTime)
		}
		if trade.EntrySignalPrice != 1.0 {
			t.Errorf("Run %d: expected entry price 1.0, got %f", run, trade.EntrySignalPrice)
		}
		if trade.ExitReason != domain.ExitReasonTimeExit {
			t.Errorf("Run %d: expected TIME_EXIT, got %s", run, trade.ExitReason)
		}
	}
}

func TestRunner_Run_SourceMismatch(t *testing.T) {
	ctx := context.Background()
	candidateID := "test-candidate-mismatch"

	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liqStore := memory.NewLiquidityTimeseriesStore()

	// Insert candidate as NEW_TOKEN
	candidate := &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceNewToken, // NEW_TOKEN
		Mint:         "mint1",
		TxSignature:  "tx1",
		Slot:         100,
		DiscoveredAt: 1000000,
	}
	if err := candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("Insert candidate failed: %v", err)
	}

	// Insert price data
	prices := makePriceTimeseries(candidateID, []float64{1.0, 1.1}, 1000000, 30000)
	if err := priceStore.InsertBulk(ctx, prices); err != nil {
		t.Fatalf("Insert prices failed: %v", err)
	}

	runner := NewRunner(RunnerOptions{
		CandidateStore:       candidateStore,
		PriceTimeseriesStore: priceStore,
		LiqTimeseriesStore:   liqStore,
	})

	// Strategy expects ACTIVE_TOKEN but candidate is NEW_TOKEN
	cfg := domain.StrategyConfig{
		StrategyType:   domain.StrategyTypeTimeExit,
		EntryEventType: "ACTIVE_TOKEN", // mismatch!
		HoldDurationMs: ptrInt64(60000),
	}

	_, err := runner.Run(ctx, candidateID, cfg, domain.ScenarioConfigRealistic)
	if !errors.Is(err, ErrSourceMismatch) {
		t.Errorf("expected ErrSourceMismatch, got %v", err)
	}
}

func TestRunner_Run_StoresTradeRecord(t *testing.T) {
	ctx := context.Background()
	candidateID := "test-candidate-store"

	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liqStore := memory.NewLiquidityTimeseriesStore()
	tradeStore := memory.NewTradeRecordStore()

	// Insert candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceNewToken,
		Mint:         "mint1",
		TxSignature:  "tx1",
		Slot:         100,
		DiscoveredAt: 1000000,
	}
	if err := candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("Insert candidate failed: %v", err)
	}

	// Insert price data
	prices := makePriceTimeseries(candidateID, []float64{1.0, 1.1, 1.2}, 1000000, 30000)
	if err := priceStore.InsertBulk(ctx, prices); err != nil {
		t.Fatalf("Insert prices failed: %v", err)
	}

	// Insert liquidity data
	liq := makeLiquidityTimeseries(candidateID, []float64{1000, 1100, 1200}, 1000000, 30000)
	if err := liqStore.InsertBulk(ctx, liq); err != nil {
		t.Fatalf("Insert liquidity failed: %v", err)
	}

	runner := NewRunner(RunnerOptions{
		CandidateStore:       candidateStore,
		PriceTimeseriesStore: priceStore,
		LiqTimeseriesStore:   liqStore,
		TradeRecordStore:     tradeStore,
	})

	cfg := domain.StrategyConfig{
		StrategyType:   domain.StrategyTypeTimeExit,
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: ptrInt64(60000),
	}

	trade, err := runner.Run(ctx, candidateID, cfg, domain.ScenarioConfigRealistic)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify trade was stored
	stored, err := tradeStore.GetByID(ctx, trade.TradeID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}

	if stored.TradeID != trade.TradeID {
		t.Errorf("stored trade ID mismatch")
	}
	if stored.CandidateID != candidateID {
		t.Errorf("stored candidate ID mismatch")
	}
}

func TestRunner_Run_CandidateNotFound(t *testing.T) {
	ctx := context.Background()

	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liqStore := memory.NewLiquidityTimeseriesStore()

	runner := NewRunner(RunnerOptions{
		CandidateStore:       candidateStore,
		PriceTimeseriesStore: priceStore,
		LiqTimeseriesStore:   liqStore,
	})

	cfg := domain.StrategyConfig{
		StrategyType:   domain.StrategyTypeTimeExit,
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: ptrInt64(60000),
	}

	_, err := runner.Run(ctx, "nonexistent", cfg, domain.ScenarioConfigRealistic)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("expected storage.ErrNotFound, got %v", err)
	}
}

func TestRunner_Run_NoPriceData(t *testing.T) {
	ctx := context.Background()
	candidateID := "test-candidate-no-price"

	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liqStore := memory.NewLiquidityTimeseriesStore()

	// Insert candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceNewToken,
		Mint:         "mint1",
		TxSignature:  "tx1",
		Slot:         100,
		DiscoveredAt: 1000000,
	}
	if err := candidateStore.Insert(ctx, candidate); err != nil {
		t.Fatalf("Insert candidate failed: %v", err)
	}

	// No price data inserted

	runner := NewRunner(RunnerOptions{
		CandidateStore:       candidateStore,
		PriceTimeseriesStore: priceStore,
		LiqTimeseriesStore:   liqStore,
	})

	cfg := domain.StrategyConfig{
		StrategyType:   domain.StrategyTypeTimeExit,
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: ptrInt64(60000),
	}

	_, err := runner.Run(ctx, candidateID, cfg, domain.ScenarioConfigRealistic)
	if !errors.Is(err, lookup.ErrNoPriceData) {
		t.Errorf("expected ErrNoPriceData, got %v", err)
	}
}
