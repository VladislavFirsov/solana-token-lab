package strategy

import (
	"context"
	"errors"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
	"solana-token-lab/internal/lookup"
)

// Helper to create test price timeseries
func makePriceTimeseries(prices []float64, startMs, intervalMs int64) []*domain.PriceTimeseriesPoint {
	result := make([]*domain.PriceTimeseriesPoint, len(prices))
	for i, p := range prices {
		result[i] = &domain.PriceTimeseriesPoint{
			CandidateID: "test-candidate",
			TimestampMs: startMs + int64(i)*intervalMs,
			Slot:        int64(100 + i),
			Price:       p,
		}
	}
	return result
}

// Helper to create test liquidity timeseries
func makeLiquidityTimeseries(liquidity []float64, startMs, intervalMs int64) []*domain.LiquidityTimeseriesPoint {
	result := make([]*domain.LiquidityTimeseriesPoint, len(liquidity))
	for i, l := range liquidity {
		result[i] = &domain.LiquidityTimeseriesPoint{
			CandidateID: "test-candidate",
			TimestampMs: startMs + int64(i)*intervalMs,
			Slot:        int64(100 + i),
			Liquidity:   l,
		}
	}
	return result
}

func TestTimeExitStrategy_Deterministic(t *testing.T) {
	// Run multiple times, verify same output
	for run := 0; run < 5; run++ {
		strategy := NewTimeExitStrategy("NEW_TOKEN", 60000) // 60 seconds

		input := &StrategyInput{
			CandidateID:      "candidate-1",
			EntrySignalTime:  1000000,
			EntrySignalPrice: 1.0,
			PriceTimeseries: makePriceTimeseries(
				[]float64{1.0, 1.1, 1.2, 1.15, 1.05},
				1000000, 30000, // every 30 seconds
			),
			Scenario: domain.ScenarioConfigRealistic,
		}

		ctx := context.Background()
		result, err := strategy.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Run %d: Execute failed: %v", run, err)
		}

		// Verify deterministic outputs
		if result.ExitReason != domain.ExitReasonTimeExit {
			t.Errorf("Run %d: expected TIME_EXIT, got %s", run, result.ExitReason)
		}

		expectedExitTime := input.EntrySignalTime + 60000
		if result.ExitSignalTime != expectedExitTime {
			t.Errorf("Run %d: expected exit time %d, got %d", run, expectedExitTime, result.ExitSignalTime)
		}

		// TradeID should be deterministic
		expectedTradeID := computeTradeID(input.CandidateID, strategy.ID(), input.Scenario.ScenarioID, input.EntrySignalTime)
		if result.TradeID != expectedTradeID {
			t.Errorf("Run %d: TradeID not deterministic", run)
		}
	}
}

func TestTimeExitStrategy_ScenarioSensitivity(t *testing.T) {
	strategy := NewTimeExitStrategy("NEW_TOKEN", 60000)

	baseInput := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.1, 1.2},
			1000000, 30000,
		),
	}

	ctx := context.Background()

	// Test with different scenarios
	scenarios := []domain.ScenarioConfig{
		domain.ScenarioConfigOptimistic,
		domain.ScenarioConfigRealistic,
		domain.ScenarioConfigPessimistic,
		domain.ScenarioConfigDegraded,
	}

	var outcomes []float64
	for _, scenario := range scenarios {
		input := *baseInput
		input.Scenario = scenario

		result, err := strategy.Execute(ctx, &input)
		if err != nil {
			t.Fatalf("Execute failed for %s: %v", scenario.ScenarioID, err)
		}
		outcomes = append(outcomes, result.Outcome)
	}

	// Verify outcomes differ (scenario sensitivity)
	// Optimistic should have best outcome, degraded worst
	if outcomes[0] <= outcomes[3] {
		t.Errorf("Optimistic outcome (%.6f) should be better than degraded (%.6f)", outcomes[0], outcomes[3])
	}

	// All outcomes should be different
	for i := 0; i < len(outcomes); i++ {
		for j := i + 1; j < len(outcomes); j++ {
			if outcomes[i] == outcomes[j] {
				t.Errorf("Outcomes for scenario %d and %d are identical: %.6f", i, j, outcomes[i])
			}
		}
	}
}

func TestTrailingStopStrategy_Deterministic(t *testing.T) {
	for run := 0; run < 5; run++ {
		strategy := NewTrailingStopStrategy("NEW_TOKEN", 0.10, 0.10, 3600000)

		input := &StrategyInput{
			CandidateID:      "candidate-1",
			EntrySignalTime:  1000000,
			EntrySignalPrice: 1.0,
			PriceTimeseries: makePriceTimeseries(
				[]float64{1.0, 1.1, 1.2, 1.15, 1.05, 1.0},
				1000000, 60000,
			),
			Scenario: domain.ScenarioConfigRealistic,
		}

		ctx := context.Background()
		result, err := strategy.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Run %d: Execute failed: %v", run, err)
		}

		// Peak should be tracked
		if result.PeakPrice == nil {
			t.Errorf("Run %d: PeakPrice should be set", run)
		} else if *result.PeakPrice != 1.2 {
			t.Errorf("Run %d: expected peak 1.2, got %.2f", run, *result.PeakPrice)
		}
	}
}

func TestTrailingStopStrategy_InitialStop(t *testing.T) {
	strategy := NewTrailingStopStrategy("NEW_TOKEN", 0.10, 0.10, 3600000)

	// Price drops immediately below initial stop (10% drop = 0.9)
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 0.85}, // 15% drop - below initial stop
			1000000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitReason != domain.ExitReasonInitialStop {
		t.Errorf("expected INITIAL_STOP, got %s", result.ExitReason)
	}
}

func TestTrailingStopStrategy_TrailingStop(t *testing.T) {
	strategy := NewTrailingStopStrategy("NEW_TOKEN", 0.10, 0.10, 3600000)

	// Price rises then drops below trailing stop (10% from peak)
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.2, 1.3, 1.4, 1.25}, // Peak 1.4, then drop to 1.25 (~10.7% from peak)
			1000000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitReason != domain.ExitReasonTrailingStop {
		t.Errorf("expected TRAILING_STOP, got %s", result.ExitReason)
	}

	// Peak should be 1.4
	if result.PeakPrice == nil || *result.PeakPrice != 1.4 {
		t.Errorf("expected peak 1.4, got %v", result.PeakPrice)
	}
}

func TestTrailingStopStrategy_MaxDuration(t *testing.T) {
	strategy := NewTrailingStopStrategy("NEW_TOKEN", 0.10, 0.10, 120000) // 2 minutes max

	// Price stays stable, no stop triggered
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.05, 1.03, 1.04, 1.02}, // Stable, never drops 10%
			1000000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitReason != domain.ExitReasonMaxDuration {
		t.Errorf("expected MAX_DURATION, got %s", result.ExitReason)
	}
}

func TestLiquidityGuardStrategy_Deterministic(t *testing.T) {
	for run := 0; run < 5; run++ {
		strategy := NewLiquidityGuardStrategy("NEW_TOKEN", 0.30, 1800000)

		entryLiq := 1000.0
		input := &StrategyInput{
			CandidateID:      "candidate-1",
			EntrySignalTime:  1000000,
			EntrySignalPrice: 1.0,
			EntryLiquidity:   &entryLiq,
			PriceTimeseries: makePriceTimeseries(
				[]float64{1.0, 1.1, 1.05},
				1000000, 60000,
			),
			LiquidityTimeseries: makeLiquidityTimeseries(
				[]float64{1000, 900, 800, 650}, // Drops below 70% threshold (700)
				1000000, 60000,
			),
			Scenario: domain.ScenarioConfigRealistic,
		}

		ctx := context.Background()
		result, err := strategy.Execute(ctx, input)
		if err != nil {
			t.Fatalf("Run %d: Execute failed: %v", run, err)
		}

		// Min liquidity should be tracked
		if result.MinLiquidity == nil {
			t.Errorf("Run %d: MinLiquidity should be set", run)
		}
	}
}

func TestLiquidityGuardStrategy_LiquidityDrop(t *testing.T) {
	strategy := NewLiquidityGuardStrategy("NEW_TOKEN", 0.30, 1800000)

	entryLiq := 1000.0
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		EntryLiquidity:   &entryLiq,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.1, 1.05, 0.95},
			1000000, 60000,
		),
		LiquidityTimeseries: makeLiquidityTimeseries(
			[]float64{1000, 900, 600}, // Drops to 60% (below 70% threshold)
			1000000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitReason != domain.ExitReasonLiquidityDrop {
		t.Errorf("expected LIQUIDITY_DROP, got %s", result.ExitReason)
	}

	// Min liquidity should be 600
	if result.MinLiquidity == nil || *result.MinLiquidity != 600 {
		t.Errorf("expected min liquidity 600, got %v", result.MinLiquidity)
	}
}

func TestLiquidityGuardStrategy_MaxDuration(t *testing.T) {
	strategy := NewLiquidityGuardStrategy("NEW_TOKEN", 0.30, 120000) // 2 minutes max

	entryLiq := 1000.0
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		EntryLiquidity:   &entryLiq,
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.1, 1.05, 1.02},
			1000000, 60000,
		),
		LiquidityTimeseries: makeLiquidityTimeseries(
			[]float64{1000, 950, 900, 850}, // Never drops below 70%
			1000000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.ExitReason != domain.ExitReasonMaxDuration {
		t.Errorf("expected MAX_DURATION, got %s", result.ExitReason)
	}
}

func TestHelpers_PriceAt(t *testing.T) {
	prices := makePriceTimeseries([]float64{1.0, 2.0, 3.0}, 1000, 1000)

	// Exact match
	p, err := lookup.PriceAt(2000, prices)
	if err != nil || p != 2.0 {
		t.Errorf("expected 2.0, got %.2f, err: %v", p, err)
	}

	// Between points - use prior
	p, err = lookup.PriceAt(2500, prices)
	if err != nil || p != 2.0 {
		t.Errorf("expected 2.0 (prior), got %.2f, err: %v", p, err)
	}

	// Before first - use first
	p, err = lookup.PriceAt(500, prices)
	if err != nil || p != 1.0 {
		t.Errorf("expected 1.0 (first), got %.2f, err: %v", p, err)
	}

	// After last - use last
	p, err = lookup.PriceAt(5000, prices)
	if err != nil || p != 3.0 {
		t.Errorf("expected 3.0 (last), got %.2f, err: %v", p, err)
	}

	// Empty slice - error
	_, err = lookup.PriceAt(1000, nil)
	if !errors.Is(err, lookup.ErrNoPriceData) {
		t.Errorf("expected ErrNoPriceData, got %v", err)
	}
}

func TestHelpers_LiquidityAt(t *testing.T) {
	liq := makeLiquidityTimeseries([]float64{100, 200, 300}, 1000, 1000)

	// Exact match
	l, err := lookup.LiquidityAt(2000, liq)
	if err != nil || l == nil || *l != 200 {
		t.Errorf("expected 200, got %v, err: %v", l, err)
	}

	// Before first - nil (valid case, not error)
	l, err = lookup.LiquidityAt(500, liq)
	if err != nil || l != nil {
		t.Errorf("expected nil, got %v, err: %v", l, err)
	}

	// Empty slice - error
	_, err = lookup.LiquidityAt(1000, nil)
	if !errors.Is(err, lookup.ErrNoLiquidityData) {
		t.Errorf("expected ErrNoLiquidityData, got %v", err)
	}
}

func TestHelpers_ComputeTradeID(t *testing.T) {
	id1 := computeTradeID("c1", "s1", "realistic", 1000)
	id2 := computeTradeID("c1", "s1", "realistic", 1000)
	id3 := computeTradeID("c1", "s1", "realistic", 1001)

	// Same inputs = same ID
	if id1 != id2 {
		t.Error("Same inputs should produce same ID")
	}

	// Different inputs = different ID
	if id1 == id3 {
		t.Error("Different inputs should produce different ID")
	}

	// Should be 64 hex chars (SHA256)
	if len(id1) != 64 {
		t.Errorf("Expected 64 char hex, got %d", len(id1))
	}

	// Should match idhash.ComputeTradeID (single source of truth)
	expected := idhash.ComputeTradeID("c1", "s1", "realistic", 1000)
	if id1 != expected {
		t.Errorf("computeTradeID should delegate to idhash.ComputeTradeID")
	}
}

func TestLiquidityGuardStrategy_NoEntryLiquidity(t *testing.T) {
	strategy := NewLiquidityGuardStrategy("NEW_TOKEN", 0.30, 1800000)

	// No entry liquidity provided and no liquidity events before entry
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		EntryLiquidity:   nil, // nil
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.1},
			1000000, 60000,
		),
		LiquidityTimeseries: makeLiquidityTimeseries(
			[]float64{1000}, // only after entry
			1060000, 60000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	_, err := strategy.Execute(ctx, input)

	if !errors.Is(err, ErrNoEntryLiquidity) {
		t.Errorf("expected ErrNoEntryLiquidity, got %v", err)
	}
}

func TestLiquidityGuardStrategy_MergedEvents_MaxDuration(t *testing.T) {
	// Test that MAX_DURATION triggers on first merged event after duration,
	// not waiting for next liquidity event
	strategy := NewLiquidityGuardStrategy("NEW_TOKEN", 0.30, 60000) // 1 minute max

	entryLiq := 1000.0
	input := &StrategyInput{
		CandidateID:      "candidate-1",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		EntryLiquidity:   &entryLiq,
		// Price events at 30s, 60s, 90s
		PriceTimeseries: makePriceTimeseries(
			[]float64{1.0, 1.1, 1.2, 1.15},
			1000000, 30000,
		),
		// Liquidity events at 120s (after max duration)
		LiquidityTimeseries: makeLiquidityTimeseries(
			[]float64{1000, 900}, // first at entry, second at 120s
			1000000, 120000,
		),
		Scenario: domain.ScenarioConfigRealistic,
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should exit at 60s (price event), not 120s (liquidity event)
	if result.ExitReason != domain.ExitReasonMaxDuration {
		t.Errorf("expected MAX_DURATION, got %s", result.ExitReason)
	}

	// Exit should be at first event >= 60s from entry
	expectedExitTime := int64(1060000) // entry + 60s
	if result.ExitSignalTime != expectedExitTime {
		t.Errorf("expected exit at %d, got %d", expectedExitTime, result.ExitSignalTime)
	}
}

func TestStrategyInput_Validate(t *testing.T) {
	validInput := &StrategyInput{
		CandidateID:      "test-candidate",
		EntrySignalTime:  1000000,
		EntrySignalPrice: 1.0,
		PriceTimeseries:  makePriceTimeseries([]float64{1.0}, 1000000, 1000),
		Scenario:         domain.ScenarioConfigRealistic,
	}

	// Valid input
	if err := validInput.Validate(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	// Nil input
	var nilInput *StrategyInput
	if err := nilInput.Validate(); err == nil {
		t.Error("expected error for nil input")
	}

	// Empty candidate ID
	input := *validInput
	input.CandidateID = ""
	if err := input.Validate(); !errors.Is(err, ErrEmptyCandidateID) {
		t.Errorf("expected ErrEmptyCandidateID, got %v", err)
	}

	// Invalid signal time
	input = *validInput
	input.EntrySignalTime = 0
	if err := input.Validate(); !errors.Is(err, ErrInvalidSignalTime) {
		t.Errorf("expected ErrInvalidSignalTime, got %v", err)
	}

	// Invalid signal price
	input = *validInput
	input.EntrySignalPrice = 0
	if err := input.Validate(); !errors.Is(err, ErrInvalidSignalPrice) {
		t.Errorf("expected ErrInvalidSignalPrice, got %v", err)
	}

	// Empty price timeseries
	input = *validInput
	input.PriceTimeseries = nil
	if err := input.Validate(); !errors.Is(err, ErrEmptyPriceTimeseries) {
		t.Errorf("expected ErrEmptyPriceTimeseries, got %v", err)
	}

	// Empty scenario ID
	input = *validInput
	input.Scenario.ScenarioID = ""
	if err := input.Validate(); !errors.Is(err, ErrEmptyScenarioID) {
		t.Errorf("expected ErrEmptyScenarioID, got %v", err)
	}
}
