package metrics

import (
	"context"
	"errors"
	"math"
	"testing"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
)

// Helper to create a candidate with specific source.
func makeCandidate(candidateID string, source domain.Source) *domain.TokenCandidate {
	return &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       source,
		Mint:         "mint-" + candidateID,
		TxSignature:  "tx-" + candidateID,
		Slot:         100,
		DiscoveredAt: 1000000,
	}
}

// Helper to create a trade record with specific outcome.
func makeTrade(id, candidateID, strategyID, scenarioID string, outcome float64, outcomeClass string, entrySignalTime int64) *domain.TradeRecord {
	return &domain.TradeRecord{
		TradeID:         id,
		CandidateID:     candidateID,
		StrategyID:      strategyID,
		ScenarioID:      scenarioID,
		Outcome:         outcome,
		OutcomeClass:    outcomeClass,
		EntrySignalTime: entrySignalTime,
	}
}

func TestComputeAggregate_Deterministic(t *testing.T) {
	ctx := context.Background()
	strategyID := "test-strategy"
	scenarioID := domain.ScenarioRealistic
	entryEventType := "NEW_TOKEN"

	// Run multiple times to verify determinism
	for run := 0; run < 5; run++ {
		candidateStore := memory.NewCandidateStore()
		tradeStore := memory.NewTradeRecordStore()
		aggStore := memory.NewStrategyAggregateStore()

		// Insert candidates with NEW_TOKEN source
		candidates := []*domain.TokenCandidate{
			makeCandidate("c1", domain.SourceNewToken),
			makeCandidate("c2", domain.SourceNewToken),
			makeCandidate("c3", domain.SourceNewToken),
			makeCandidate("c4", domain.SourceNewToken),
			makeCandidate("c5", domain.SourceNewToken),
		}
		for _, c := range candidates {
			if err := candidateStore.Insert(ctx, c); err != nil {
				t.Fatalf("Insert candidate failed: %v", err)
			}
		}

		// Insert trades
		trades := []*domain.TradeRecord{
			makeTrade("t1", "c1", strategyID, scenarioID, 0.10, domain.OutcomeClassWin, 1000),
			makeTrade("t2", "c2", strategyID, scenarioID, -0.05, domain.OutcomeClassLoss, 2000),
			makeTrade("t3", "c3", strategyID, scenarioID, 0.15, domain.OutcomeClassWin, 3000),
			makeTrade("t4", "c4", strategyID, scenarioID, -0.02, domain.OutcomeClassLoss, 4000),
			makeTrade("t5", "c5", strategyID, scenarioID, 0.08, domain.OutcomeClassWin, 5000),
		}

		if err := tradeStore.InsertBulk(ctx, trades); err != nil {
			t.Fatalf("InsertBulk failed: %v", err)
		}

		aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
		agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, entryEventType)
		if err != nil {
			t.Fatalf("Run %d: ComputeAggregate failed: %v", run, err)
		}

		// Verify deterministic values
		if agg.TotalTrades != 5 {
			t.Errorf("Run %d: expected TotalTrades 5, got %d", run, agg.TotalTrades)
		}
		if agg.Wins != 3 {
			t.Errorf("Run %d: expected Wins 3, got %d", run, agg.Wins)
		}
		if agg.Losses != 2 {
			t.Errorf("Run %d: expected Losses 2, got %d", run, agg.Losses)
		}
	}
}

func TestComputeAggregate_WinLossCounts(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-1"
	scenarioID := domain.ScenarioRealistic

	// Insert 10 candidates with NEW_TOKEN source
	for i := 0; i < 10; i++ {
		c := makeCandidate("c"+string(rune('0'+i)), domain.SourceNewToken)
		c.CandidateID = "cand-" + string(rune('A'+i))
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// 7 wins, 3 losses
	trades := []*domain.TradeRecord{
		makeTrade("t1", "cand-A", strategyID, scenarioID, 0.10, domain.OutcomeClassWin, 1000),
		makeTrade("t2", "cand-B", strategyID, scenarioID, 0.05, domain.OutcomeClassWin, 2000),
		makeTrade("t3", "cand-C", strategyID, scenarioID, -0.03, domain.OutcomeClassLoss, 3000),
		makeTrade("t4", "cand-D", strategyID, scenarioID, 0.12, domain.OutcomeClassWin, 4000),
		makeTrade("t5", "cand-E", strategyID, scenarioID, 0.08, domain.OutcomeClassWin, 5000),
		makeTrade("t6", "cand-F", strategyID, scenarioID, -0.05, domain.OutcomeClassLoss, 6000),
		makeTrade("t7", "cand-G", strategyID, scenarioID, 0.15, domain.OutcomeClassWin, 7000),
		makeTrade("t8", "cand-H", strategyID, scenarioID, 0.02, domain.OutcomeClassWin, 8000),
		makeTrade("t9", "cand-I", strategyID, scenarioID, -0.08, domain.OutcomeClassLoss, 9000),
		makeTrade("t10", "cand-J", strategyID, scenarioID, 0.20, domain.OutcomeClassWin, 10000),
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
	agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate failed: %v", err)
	}

	if agg.TotalTrades != 10 {
		t.Errorf("expected TotalTrades 10, got %d", agg.TotalTrades)
	}
	if agg.Wins != 7 {
		t.Errorf("expected Wins 7, got %d", agg.Wins)
	}
	if agg.Losses != 3 {
		t.Errorf("expected Losses 3, got %d", agg.Losses)
	}

	expectedWinRate := 0.7
	if math.Abs(agg.WinRate-expectedWinRate) > 0.0001 {
		t.Errorf("expected WinRate %.4f, got %.4f", expectedWinRate, agg.WinRate)
	}
}

func TestComputeAggregate_Quantiles(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-quantiles"
	scenarioID := domain.ScenarioRealistic

	// Create 10 candidates
	for i := 0; i < 10; i++ {
		c := makeCandidate("quant-"+string(rune('A'+i)), domain.SourceNewToken)
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Create 10 trades with outcomes: -0.20, -0.10, 0.00, 0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.40
	// Sorted: [-0.20, -0.10, 0.00, 0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.40]
	outcomes := []float64{0.10, -0.10, 0.30, 0.00, 0.20, -0.20, 0.40, 0.05, 0.25, 0.15}
	trades := make([]*domain.TradeRecord, len(outcomes))
	for i, o := range outcomes {
		class := domain.OutcomeClassWin
		if o <= 0 {
			class = domain.OutcomeClassLoss
		}
		trades[i] = makeTrade(
			"qt-"+string(rune('A'+i)),
			"quant-"+string(rune('A'+i)),
			strategyID,
			scenarioID,
			o,
			class,
			int64((i+1)*1000),
		)
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
	agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate failed: %v", err)
	}

	// n=10, sorted: [-0.20, -0.10, 0.00, 0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.40]
	// P10: idx = 0.10 * 9 = 0.9 → lerp(sorted[0], sorted[1], 0.9) = -0.20 + 0.9*0.10 = -0.11
	// P25: idx = 0.25 * 9 = 2.25 → lerp(sorted[2], sorted[3], 0.25) = 0.00 + 0.25*0.05 = 0.0125
	// P50: idx = 0.50 * 9 = 4.5 → lerp(sorted[4], sorted[5], 0.5) = 0.10 + 0.5*0.05 = 0.125
	// P75: idx = 0.75 * 9 = 6.75 → lerp(sorted[6], sorted[7], 0.75) = 0.20 + 0.75*0.05 = 0.2375
	// P90: idx = 0.90 * 9 = 8.1 → lerp(sorted[8], sorted[9], 0.1) = 0.30 + 0.1*0.10 = 0.31

	tests := []struct {
		name     string
		got      float64
		expected float64
	}{
		{"OutcomeMin", agg.OutcomeMin, -0.20},
		{"OutcomeMax", agg.OutcomeMax, 0.40},
		{"OutcomeP10", agg.OutcomeP10, -0.11},
		{"OutcomeP25", agg.OutcomeP25, 0.0125},
		{"OutcomeMedian", agg.OutcomeMedian, 0.125},
		{"OutcomeP75", agg.OutcomeP75, 0.2375},
		{"OutcomeP90", agg.OutcomeP90, 0.31},
	}

	for _, tt := range tests {
		if math.Abs(tt.got-tt.expected) > 0.0001 {
			t.Errorf("%s: expected %.4f, got %.4f", tt.name, tt.expected, tt.got)
		}
	}
}

func TestComputeAggregate_MaxDrawdown(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-drawdown"
	scenarioID := domain.ScenarioRealistic

	// Create candidates
	for i := 0; i < 6; i++ {
		c := makeCandidate("dd-"+string(rune('A'+i)), domain.SourceNewToken)
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Outcomes in order (by EntrySignalTime): +0.10, +0.20, -0.15, -0.10, +0.05, -0.25
	// Cumulative: 0.10, 0.30, 0.15, 0.05, 0.10, -0.15
	// Peak tracking: 0.10, 0.30, 0.30, 0.30, 0.30, 0.30
	// Drawdown: 0, 0, 0.15, 0.25, 0.20, 0.45
	// Max drawdown = 0.45
	outcomes := []float64{0.10, 0.20, -0.15, -0.10, 0.05, -0.25}
	trades := make([]*domain.TradeRecord, len(outcomes))
	for i, o := range outcomes {
		class := domain.OutcomeClassWin
		if o <= 0 {
			class = domain.OutcomeClassLoss
		}
		trades[i] = &domain.TradeRecord{
			TradeID:         "dd-t" + string(rune('A'+i)),
			CandidateID:     "dd-" + string(rune('A'+i)),
			StrategyID:      strategyID,
			ScenarioID:      scenarioID,
			Outcome:         o,
			OutcomeClass:    class,
			EntrySignalTime: int64((i + 1) * 1000), // Preserve order
		}
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
	agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate failed: %v", err)
	}

	expectedMaxDrawdown := 0.45
	if math.Abs(agg.MaxDrawdown-expectedMaxDrawdown) > 0.0001 {
		t.Errorf("expected MaxDrawdown %.4f, got %.4f", expectedMaxDrawdown, agg.MaxDrawdown)
	}
}

func TestComputeAggregate_MaxConsecutiveLosses(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-streak"
	scenarioID := domain.ScenarioRealistic

	// Create candidates
	for i := 0; i < 9; i++ {
		c := makeCandidate("streak-"+string(rune('A'+i)), domain.SourceNewToken)
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Pattern (by EntrySignalTime order): W, L, L, W, L, L, L, W, L
	// Max consecutive losses = 3
	outcomes := []float64{0.10, -0.05, -0.03, 0.08, -0.02, -0.04, -0.06, 0.12, -0.01}
	trades := make([]*domain.TradeRecord, len(outcomes))
	for i, o := range outcomes {
		class := domain.OutcomeClassWin
		if o <= 0 {
			class = domain.OutcomeClassLoss
		}
		trades[i] = &domain.TradeRecord{
			TradeID:         "streak-t" + string(rune('A'+i)),
			CandidateID:     "streak-" + string(rune('A'+i)),
			StrategyID:      strategyID,
			ScenarioID:      scenarioID,
			Outcome:         o,
			OutcomeClass:    class,
			EntrySignalTime: int64((i + 1) * 1000), // Preserve order
		}
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
	agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate failed: %v", err)
	}

	if agg.MaxConsecutiveLosses != 3 {
		t.Errorf("expected MaxConsecutiveLosses 3, got %d", agg.MaxConsecutiveLosses)
	}
}

func TestComputeAggregate_SensitivityFields(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		scenarioID        string
		expectRealistic   bool
		expectPessimistic bool
		expectDegraded    bool
	}{
		{domain.ScenarioRealistic, true, false, false},
		{domain.ScenarioPessimistic, false, true, false},
		{domain.ScenarioDegraded, false, false, true},
		{domain.ScenarioOptimistic, false, false, false}, // No sensitivity field for optimistic
	}

	for _, tt := range tests {
		t.Run(tt.scenarioID, func(t *testing.T) {
			candidateStore := memory.NewCandidateStore()
			tradeStore := memory.NewTradeRecordStore()
			aggStore := memory.NewStrategyAggregateStore()

			strategyID := "strategy-sens"

			// Create candidate
			c := makeCandidate("sens-cand-"+tt.scenarioID, domain.SourceNewToken)
			if err := candidateStore.Insert(ctx, c); err != nil {
				t.Fatalf("Insert candidate failed: %v", err)
			}

			trade := &domain.TradeRecord{
				TradeID:         "sens-" + tt.scenarioID,
				CandidateID:     "sens-cand-" + tt.scenarioID,
				StrategyID:      strategyID,
				ScenarioID:      tt.scenarioID,
				Outcome:         0.10,
				OutcomeClass:    domain.OutcomeClassWin,
				EntrySignalTime: 1000,
			}

			if err := tradeStore.Insert(ctx, trade); err != nil {
				t.Fatalf("Insert failed: %v", err)
			}

			aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
			agg, err := aggregator.ComputeAggregate(ctx, strategyID, tt.scenarioID, "NEW_TOKEN")
			if err != nil {
				t.Fatalf("ComputeAggregate failed: %v", err)
			}

			if tt.expectRealistic && agg.OutcomeRealistic == nil {
				t.Error("expected OutcomeRealistic to be set")
			}
			if !tt.expectRealistic && agg.OutcomeRealistic != nil {
				t.Error("expected OutcomeRealistic to be nil")
			}

			if tt.expectPessimistic && agg.OutcomePessimistic == nil {
				t.Error("expected OutcomePessimistic to be set")
			}
			if !tt.expectPessimistic && agg.OutcomePessimistic != nil {
				t.Error("expected OutcomePessimistic to be nil")
			}

			if tt.expectDegraded && agg.OutcomeDegraded == nil {
				t.Error("expected OutcomeDegraded to be set")
			}
			if !tt.expectDegraded && agg.OutcomeDegraded != nil {
				t.Error("expected OutcomeDegraded to be nil")
			}
		})
	}
}

func TestComputeAndStore_Duplicate(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-dup"
	scenarioID := domain.ScenarioRealistic
	entryEventType := "NEW_TOKEN"

	// Insert a candidate
	c := makeCandidate("dup-cand", domain.SourceNewToken)
	if err := candidateStore.Insert(ctx, c); err != nil {
		t.Fatalf("Insert candidate failed: %v", err)
	}

	// Insert a trade
	trade := &domain.TradeRecord{
		TradeID:         "dup-trade-1",
		CandidateID:     "dup-cand",
		StrategyID:      strategyID,
		ScenarioID:      scenarioID,
		Outcome:         0.10,
		OutcomeClass:    domain.OutcomeClassWin,
		EntrySignalTime: 1000,
	}

	if err := tradeStore.Insert(ctx, trade); err != nil {
		t.Fatalf("Insert trade failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)

	// First store should succeed
	agg1, err := aggregator.ComputeAndStore(ctx, strategyID, scenarioID, entryEventType)
	if err != nil {
		t.Fatalf("First ComputeAndStore failed: %v", err)
	}
	if agg1 == nil {
		t.Fatal("First ComputeAndStore returned nil aggregate")
	}

	// Second store should return ErrDuplicateKey
	_, err = aggregator.ComputeAndStore(ctx, strategyID, scenarioID, entryEventType)
	if !errors.Is(err, storage.ErrDuplicateKey) {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}
}

func TestComputeAggregate_NoTrades(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)

	_, err := aggregator.ComputeAggregate(ctx, "nonexistent", domain.ScenarioRealistic, "NEW_TOKEN")
	if !errors.Is(err, ErrNoTrades) {
		t.Errorf("expected ErrNoTrades, got %v", err)
	}
}

func TestComputeAggregate_FiltersByEntryEventType(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-filter"
	scenarioID := domain.ScenarioRealistic

	// Create candidates with different sources
	newTokenCandidates := []*domain.TokenCandidate{
		makeCandidate("new-1", domain.SourceNewToken),
		makeCandidate("new-2", domain.SourceNewToken),
		makeCandidate("new-3", domain.SourceNewToken),
	}
	activeTokenCandidates := []*domain.TokenCandidate{
		makeCandidate("active-1", domain.SourceActiveToken),
		makeCandidate("active-2", domain.SourceActiveToken),
	}

	for _, c := range newTokenCandidates {
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}
	for _, c := range activeTokenCandidates {
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Create trades for both types
	trades := []*domain.TradeRecord{
		// NEW_TOKEN trades (3)
		makeTrade("t-new-1", "new-1", strategyID, scenarioID, 0.10, domain.OutcomeClassWin, 1000),
		makeTrade("t-new-2", "new-2", strategyID, scenarioID, 0.15, domain.OutcomeClassWin, 2000),
		makeTrade("t-new-3", "new-3", strategyID, scenarioID, -0.05, domain.OutcomeClassLoss, 3000),
		// ACTIVE_TOKEN trades (2)
		makeTrade("t-active-1", "active-1", strategyID, scenarioID, 0.20, domain.OutcomeClassWin, 4000),
		makeTrade("t-active-2", "active-2", strategyID, scenarioID, -0.08, domain.OutcomeClassLoss, 5000),
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)

	// Test NEW_TOKEN filter
	aggNew, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate NEW_TOKEN failed: %v", err)
	}
	if aggNew.TotalTrades != 3 {
		t.Errorf("NEW_TOKEN: expected TotalTrades 3, got %d", aggNew.TotalTrades)
	}
	if aggNew.Wins != 2 {
		t.Errorf("NEW_TOKEN: expected Wins 2, got %d", aggNew.Wins)
	}
	if aggNew.Losses != 1 {
		t.Errorf("NEW_TOKEN: expected Losses 1, got %d", aggNew.Losses)
	}

	// Test ACTIVE_TOKEN filter
	aggActive, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "ACTIVE_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate ACTIVE_TOKEN failed: %v", err)
	}
	if aggActive.TotalTrades != 2 {
		t.Errorf("ACTIVE_TOKEN: expected TotalTrades 2, got %d", aggActive.TotalTrades)
	}
	if aggActive.Wins != 1 {
		t.Errorf("ACTIVE_TOKEN: expected Wins 1, got %d", aggActive.Wins)
	}
	if aggActive.Losses != 1 {
		t.Errorf("ACTIVE_TOKEN: expected Losses 1, got %d", aggActive.Losses)
	}
}

func TestComputeAggregate_DeterministicSorting(t *testing.T) {
	ctx := context.Background()
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	strategyID := "strategy-sort"
	scenarioID := domain.ScenarioRealistic

	// Create candidates
	for i := 0; i < 3; i++ {
		c := makeCandidate("sort-"+string(rune('A'+i)), domain.SourceNewToken)
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Create trades with same EntrySignalTime to test TradeID tie-breaker
	// Pattern by TradeID order (A < B < C): W, L, L → max consecutive = 2
	trades := []*domain.TradeRecord{
		{
			TradeID:         "sort-C", // Last alphabetically
			CandidateID:     "sort-A",
			StrategyID:      strategyID,
			ScenarioID:      scenarioID,
			Outcome:         -0.05,
			OutcomeClass:    domain.OutcomeClassLoss,
			EntrySignalTime: 1000, // Same time
		},
		{
			TradeID:         "sort-A", // First alphabetically
			CandidateID:     "sort-B",
			StrategyID:      strategyID,
			ScenarioID:      scenarioID,
			Outcome:         0.10,
			OutcomeClass:    domain.OutcomeClassWin,
			EntrySignalTime: 1000, // Same time
		},
		{
			TradeID:         "sort-B", // Middle alphabetically
			CandidateID:     "sort-C",
			StrategyID:      strategyID,
			ScenarioID:      scenarioID,
			Outcome:         -0.03,
			OutcomeClass:    domain.OutcomeClassLoss,
			EntrySignalTime: 1000, // Same time
		},
	}

	if err := tradeStore.InsertBulk(ctx, trades); err != nil {
		t.Fatalf("InsertBulk failed: %v", err)
	}

	aggregator := NewAggregator(tradeStore, aggStore, candidateStore)
	agg, err := aggregator.ComputeAggregate(ctx, strategyID, scenarioID, "NEW_TOKEN")
	if err != nil {
		t.Fatalf("ComputeAggregate failed: %v", err)
	}

	// Sorted by TradeID: A (W), B (L), C (L) → max consecutive losses = 2
	if agg.MaxConsecutiveLosses != 2 {
		t.Errorf("expected MaxConsecutiveLosses 2, got %d", agg.MaxConsecutiveLosses)
	}
}
