package verification

import (
	"context"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
	"solana-token-lab/internal/strategy"
)

func TestCompareTradeRecords_ExactMatch(t *testing.T) {
	trade := &domain.TradeRecord{
		TradeID:          "trade1",
		CandidateID:      "c1",
		StrategyID:       "TIME_EXIT",
		ScenarioID:       "REALISTIC",
		EntrySignalTime:  1000,
		EntrySignalPrice: 1.0,
		EntryActualTime:  1050,
		EntryActualPrice: 1.005,
		EntryLiquidity:   ptrFloat64(1000.0),
		PositionSize:     1.0,
		PositionValue:    1.005,
		ExitSignalTime:   2000,
		ExitSignalPrice:  1.2,
		ExitActualTime:   2050,
		ExitActualPrice:  1.194,
		ExitReason:       "TIME_EXIT",
		EntryCostSOL:     0.0001,
		ExitCostSOL:      0.0001,
		MEVCostSOL:       0.001,
		TotalCostSOL:     0.0012,
		TotalCostPct:     0.12,
		GrossReturn:      0.188,
		Outcome:          0.068,
		OutcomeClass:     "WIN",
		HoldDurationMs:   1000,
		PeakPrice:        ptrFloat64(1.25),
		MinLiquidity:     ptrFloat64(900.0),
	}

	// Copy for replayed
	replayed := &domain.TradeRecord{
		TradeID:          "trade1",
		CandidateID:      "c1",
		StrategyID:       "TIME_EXIT",
		ScenarioID:       "REALISTIC",
		EntrySignalTime:  1000,
		EntrySignalPrice: 1.0,
		EntryActualTime:  1050,
		EntryActualPrice: 1.005,
		EntryLiquidity:   ptrFloat64(1000.0),
		PositionSize:     1.0,
		PositionValue:    1.005,
		ExitSignalTime:   2000,
		ExitSignalPrice:  1.2,
		ExitActualTime:   2050,
		ExitActualPrice:  1.194,
		ExitReason:       "TIME_EXIT",
		EntryCostSOL:     0.0001,
		ExitCostSOL:      0.0001,
		MEVCostSOL:       0.001,
		TotalCostSOL:     0.0012,
		TotalCostPct:     0.12,
		GrossReturn:      0.188,
		Outcome:          0.068,
		OutcomeClass:     "WIN",
		HoldDurationMs:   1000,
		PeakPrice:        ptrFloat64(1.25),
		MinLiquidity:     ptrFloat64(900.0),
	}

	divergences := CompareTradeRecords(trade, replayed)

	if len(divergences) != 0 {
		t.Errorf("Expected 0 divergences, got %d: %v", len(divergences), divergences)
	}
}

func TestCompareTradeRecords_OutcomeDivergence(t *testing.T) {
	stored := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		Outcome:     0.05,
	}

	replayed := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		Outcome:     0.06, // Different outcome
	}

	divergences := CompareTradeRecords(stored, replayed)

	if len(divergences) != 1 {
		t.Errorf("Expected 1 divergence, got %d", len(divergences))
	}

	if divergences[0].Field != "Outcome" {
		t.Errorf("Expected Outcome divergence, got %s", divergences[0].Field)
	}
}

func TestCompareTradeRecords_ExitReasonDivergence(t *testing.T) {
	stored := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TRAILING_STOP",
		ScenarioID:  "REALISTIC",
		ExitReason:  "TRAILING_STOP",
	}

	replayed := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TRAILING_STOP",
		ScenarioID:  "REALISTIC",
		ExitReason:  "INITIAL_STOP", // Different exit reason
	}

	divergences := CompareTradeRecords(stored, replayed)

	foundExitReason := false
	for _, d := range divergences {
		if d.Field == "ExitReason" {
			foundExitReason = true
			break
		}
	}

	if !foundExitReason {
		t.Error("Expected ExitReason divergence")
	}
}

func TestCompareTradeRecords_WithinTolerance(t *testing.T) {
	stored := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		Outcome:     0.123456789,
	}

	replayed := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		Outcome:     0.123456789 + FloatTolerance/2, // Within tolerance
	}

	divergences := CompareTradeRecords(stored, replayed)

	for _, d := range divergences {
		if d.Field == "Outcome" {
			t.Errorf("Outcome should not diverge within tolerance: stored=%v, replayed=%v",
				d.Expected, d.Actual)
		}
	}
}

func TestCompareTradeRecords_NullableFields(t *testing.T) {
	// Both nil
	stored := &domain.TradeRecord{
		TradeID:      "trade1",
		CandidateID:  "c1",
		StrategyID:   "TIME_EXIT",
		ScenarioID:   "REALISTIC",
		PeakPrice:    nil,
		MinLiquidity: nil,
	}

	replayed := &domain.TradeRecord{
		TradeID:      "trade1",
		CandidateID:  "c1",
		StrategyID:   "TIME_EXIT",
		ScenarioID:   "REALISTIC",
		PeakPrice:    nil,
		MinLiquidity: nil,
	}

	divergences := CompareTradeRecords(stored, replayed)

	for _, d := range divergences {
		if d.Field == "PeakPrice" || d.Field == "MinLiquidity" {
			t.Errorf("Should not diverge when both nil: %s", d.Field)
		}
	}
}

func TestCompareTradeRecords_NullVsValue(t *testing.T) {
	stored := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		PeakPrice:   nil,
	}

	replayed := &domain.TradeRecord{
		TradeID:     "trade1",
		CandidateID: "c1",
		StrategyID:  "TIME_EXIT",
		ScenarioID:  "REALISTIC",
		PeakPrice:   ptrFloat64(1.5),
	}

	divergences := CompareTradeRecords(stored, replayed)

	foundPeakPrice := false
	for _, d := range divergences {
		if d.Field == "PeakPrice" {
			foundPeakPrice = true
			break
		}
	}

	if !foundPeakPrice {
		t.Error("Expected PeakPrice divergence when nil vs value")
	}
}

func TestReplayVerifier_VerifyTrade_ExactMatch(t *testing.T) {
	ctx := context.Background()

	// Setup stores
	tradeStore := memory.NewTradeRecordStore()
	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liquidityStore := memory.NewLiquidityTimeseriesStore()

	// Create candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  "c1",
		Mint:         "mint1",
		Source:       domain.SourceNewToken,
		DiscoveredAt: 1000,
		Slot:         100,
		TxSignature:  "tx1",
		EventIndex:   0,
	}
	_ = candidateStore.Insert(ctx, candidate)

	// Create price timeseries
	prices := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0, Slot: 100},
		{CandidateID: "c1", TimestampMs: 2000, Price: 1.1, Slot: 200},
		{CandidateID: "c1", TimestampMs: 300001000, Price: 1.2, Slot: 300000100}, // For TIME_EXIT (5 min hold)
	}
	_ = priceStore.InsertBulk(ctx, prices)

	// Create liquidity timeseries (required to avoid ErrNoLiquidityData)
	liquidity := []*domain.LiquidityTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Liquidity: 1000.0, Slot: 100},
	}
	_ = liquidityStore.InsertBulk(ctx, liquidity)

	// Strategy config for TIME_EXIT (5 minute hold)
	holdDuration := int64(300000000) // 5 minutes
	strategyConfig := domain.StrategyConfig{
		StrategyType:   "TIME_EXIT",
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: &holdDuration,
	}

	// Scenario config (REALISTIC)
	scenarioConfig := domain.ScenarioConfig{
		ScenarioID:     domain.ScenarioRealistic,
		DelayMs:        50,
		SlippagePct:    1.0,
		FeeSOL:         0.000005,
		PriorityFeeSOL: 0.00005,
		MEVPenaltyPct:  0.1,
	}

	// Run simulation once to get a trade
	strat, _ := strategy.FromConfig(strategyConfig)

	// Get the actual strategyID from the strategy
	strategyID := strat.ID()

	// Create verifier with the correct strategyID key
	verifier := NewReplayVerifier(ReplayVerifierOptions{
		TradeStore:     tradeStore,
		CandidateStore: candidateStore,
		PriceStore:     priceStore,
		LiquidityStore: liquidityStore,
		StrategyConfigs: map[string]domain.StrategyConfig{
			strategyID: strategyConfig,
		},
		ScenarioConfigs: map[string]domain.ScenarioConfig{
			domain.ScenarioRealistic: scenarioConfig,
		},
	})

	// Get entry liquidity from the store (same as replay will do)
	entryLiq := ptrFloat64(1000.0) // matches the liquidity data at timestamp 1000

	input := &strategy.StrategyInput{
		CandidateID:         "c1",
		EntrySignalTime:     1000,
		EntrySignalPrice:    1.0,
		EntryLiquidity:      entryLiq,
		PriceTimeseries:     prices,
		LiquidityTimeseries: liquidity,
		Scenario:            scenarioConfig,
	}
	originalTrade, err := strat.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Failed to execute strategy: %v", err)
	}

	// Store the trade
	_ = tradeStore.Insert(ctx, originalTrade)

	// Verify
	result, err := verifier.VerifyTrade(ctx, originalTrade.TradeID)
	if err != nil {
		t.Fatalf("VerifyTrade failed: %v", err)
	}

	if !result.Match {
		t.Errorf("Expected match, got divergences: %v", result.Divergences)
	}
}

func TestReplayVerifier_VerifyAll(t *testing.T) {
	ctx := context.Background()

	// Setup stores
	tradeStore := memory.NewTradeRecordStore()
	candidateStore := memory.NewCandidateStore()
	priceStore := memory.NewPriceTimeseriesStore()
	liquidityStore := memory.NewLiquidityTimeseriesStore()

	// Create candidate
	candidate := &domain.TokenCandidate{
		CandidateID:  "c1",
		Mint:         "mint1",
		Source:       domain.SourceNewToken,
		DiscoveredAt: 1000,
		Slot:         100,
		TxSignature:  "tx1",
		EventIndex:   0,
	}
	_ = candidateStore.Insert(ctx, candidate)

	// Create price timeseries
	prices := []*domain.PriceTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Price: 1.0, Slot: 100},
		{CandidateID: "c1", TimestampMs: 300001000, Price: 1.2, Slot: 300000100},
	}
	_ = priceStore.InsertBulk(ctx, prices)

	// Create liquidity timeseries (required to avoid ErrNoLiquidityData)
	liquidity := []*domain.LiquidityTimeseriesPoint{
		{CandidateID: "c1", TimestampMs: 1000, Liquidity: 1000.0, Slot: 100},
	}
	_ = liquidityStore.InsertBulk(ctx, liquidity)

	// Strategy config
	holdDuration := int64(300000000)
	strategyConfig := domain.StrategyConfig{
		StrategyType:   "TIME_EXIT",
		EntryEventType: "NEW_TOKEN",
		HoldDurationMs: &holdDuration,
	}

	// Scenario config
	scenarioConfig := domain.ScenarioConfig{
		ScenarioID:     domain.ScenarioRealistic,
		DelayMs:        50,
		SlippagePct:    1.0,
		FeeSOL:         0.000005,
		PriorityFeeSOL: 0.00005,
		MEVPenaltyPct:  0.1,
	}

	// Run simulation
	strat, _ := strategy.FromConfig(strategyConfig)

	// Get the actual strategyID from the strategy
	strategyID := strat.ID()

	verifier := NewReplayVerifier(ReplayVerifierOptions{
		TradeStore:     tradeStore,
		CandidateStore: candidateStore,
		PriceStore:     priceStore,
		LiquidityStore: liquidityStore,
		StrategyConfigs: map[string]domain.StrategyConfig{
			strategyID: strategyConfig,
		},
		ScenarioConfigs: map[string]domain.ScenarioConfig{
			domain.ScenarioRealistic: scenarioConfig,
		},
	})

	// Get entry liquidity from the store (same as replay will do)
	entryLiq := ptrFloat64(1000.0)

	input := &strategy.StrategyInput{
		CandidateID:         "c1",
		EntrySignalTime:     1000,
		EntrySignalPrice:    1.0,
		EntryLiquidity:      entryLiq,
		PriceTimeseries:     prices,
		LiquidityTimeseries: liquidity,
		Scenario:            scenarioConfig,
	}
	trade, _ := strat.Execute(ctx, input)
	_ = tradeStore.Insert(ctx, trade)

	// VerifyAll
	report, err := verifier.VerifyAll(ctx)
	if err != nil {
		t.Fatalf("VerifyAll failed: %v", err)
	}

	if report.TotalTrades != 1 {
		t.Errorf("Expected 1 total trade, got %d", report.TotalTrades)
	}

	if report.MatchedTrades != 1 {
		t.Errorf("Expected 1 matched trade, got %d", report.MatchedTrades)
	}

	if report.DivergentTrades != 0 {
		t.Errorf("Expected 0 divergent trades, got %d", report.DivergentTrades)
	}
}

func TestFloatEquals(t *testing.T) {
	tests := []struct {
		name string
		a, b float64
		want bool
	}{
		{"exact match", 1.0, 1.0, true},
		{"within tolerance", 1.0, 1.0 + FloatTolerance/2, true},
		{"at tolerance boundary", 1.0, 1.0 + FloatTolerance, true},
		{"beyond tolerance", 1.0, 1.0 + FloatTolerance*2, false},
		{"zeros", 0.0, 0.0, true},
		{"small values", 1e-10, 1e-10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := floatEquals(tt.a, tt.b); got != tt.want {
				t.Errorf("floatEquals(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func ptrFloat64(v float64) *float64 {
	return &v
}
