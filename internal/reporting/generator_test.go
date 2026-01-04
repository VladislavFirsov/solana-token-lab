package reporting

import (
	"context"
	"strings"
	"testing"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage/memory"
)

func setupTestData(t *testing.T) (*memory.CandidateStore, *memory.TradeRecordStore, *memory.StrategyAggregateStore) {
	ctx := context.Background()

	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	// Insert candidates
	candidates := []*domain.TokenCandidate{
		{CandidateID: "c1", Source: domain.SourceNewToken, Mint: "mint1", TxSignature: "tx1", Slot: 100, DiscoveredAt: 1000000},
		{CandidateID: "c2", Source: domain.SourceNewToken, Mint: "mint2", TxSignature: "tx2", Slot: 101, DiscoveredAt: 2000000},
		{CandidateID: "c3", Source: domain.SourceActiveToken, Mint: "mint3", TxSignature: "tx3", Slot: 102, DiscoveredAt: 1500000},
	}
	for _, c := range candidates {
		if err := candidateStore.Insert(ctx, c); err != nil {
			t.Fatalf("Insert candidate failed: %v", err)
		}
	}

	// Insert trades
	trades := []*domain.TradeRecord{
		{TradeID: "t1", CandidateID: "c1", StrategyID: "TIME_EXIT", ScenarioID: domain.ScenarioRealistic, Outcome: 0.10, OutcomeClass: domain.OutcomeClassWin},
		{TradeID: "t2", CandidateID: "c2", StrategyID: "TIME_EXIT", ScenarioID: domain.ScenarioRealistic, Outcome: -0.05, OutcomeClass: domain.OutcomeClassLoss},
		{TradeID: "t3", CandidateID: "c3", StrategyID: "TIME_EXIT", ScenarioID: domain.ScenarioRealistic, Outcome: 0.15, OutcomeClass: domain.OutcomeClassWin},
	}
	for _, tr := range trades {
		if err := tradeStore.Insert(ctx, tr); err != nil {
			t.Fatalf("Insert trade failed: %v", err)
		}
	}

	// Insert aggregates
	aggregates := []*domain.StrategyAggregate{
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioRealistic,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          2,
			Wins:                 1,
			Losses:               1,
			WinRate:              0.5,
			OutcomeMean:          0.025,
			OutcomeMedian:        0.025,
			OutcomeP10:           -0.05,
			OutcomeP90:           0.10,
			MaxDrawdown:          0.05,
			MaxConsecutiveLosses: 1,
		},
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioRealistic,
			EntryEventType:       "ACTIVE_TOKEN",
			TotalTrades:          1,
			Wins:                 1,
			Losses:               0,
			WinRate:              1.0,
			OutcomeMean:          0.15,
			OutcomeMedian:        0.15,
			OutcomeP10:           0.15,
			OutcomeP90:           0.15,
			MaxDrawdown:          0,
			MaxConsecutiveLosses: 0,
		},
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioPessimistic,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          2,
			Wins:                 0,
			Losses:               2,
			WinRate:              0.0,
			OutcomeMean:          -0.10,
			OutcomeMedian:        -0.10,
			OutcomeP10:           -0.15,
			OutcomeP90:           -0.05,
			MaxDrawdown:          0.20,
			MaxConsecutiveLosses: 2,
		},
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioDegraded,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          2,
			Wins:                 0,
			Losses:               2,
			WinRate:              0.0,
			OutcomeMean:          -0.20,
			OutcomeMedian:        -0.20,
			OutcomeP10:           -0.25,
			OutcomeP90:           -0.15,
			MaxDrawdown:          0.40,
			MaxConsecutiveLosses: 2,
		},
	}
	for _, agg := range aggregates {
		if err := aggStore.Insert(ctx, agg); err != nil {
			t.Fatalf("Insert aggregate failed: %v", err)
		}
	}

	return candidateStore, tradeStore, aggStore
}

func TestGenerate_Deterministic(t *testing.T) {
	ctx := context.Background()

	// Fixed time for deterministic output
	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	fixedClock := func() time.Time { return fixedTime }

	// Run multiple times and verify same output
	var firstReport *Report
	for run := 0; run < 5; run++ {
		candidateStore, tradeStore, aggStore := setupTestData(t)
		generator := NewGenerator(candidateStore, tradeStore, aggStore).WithClock(fixedClock)

		report, err := generator.Generate(ctx)
		if err != nil {
			t.Fatalf("Run %d: Generate failed: %v", run, err)
		}

		if firstReport == nil {
			firstReport = report
			continue
		}

		// Verify GeneratedAt is stable
		if !report.GeneratedAt.Equal(firstReport.GeneratedAt) {
			t.Errorf("Run %d: GeneratedAt mismatch: got %v, want %v", run, report.GeneratedAt, firstReport.GeneratedAt)
		}

		// Verify deterministic values
		if report.StrategyCount != firstReport.StrategyCount {
			t.Errorf("Run %d: StrategyCount mismatch", run)
		}
		if report.ScenarioCount != firstReport.ScenarioCount {
			t.Errorf("Run %d: ScenarioCount mismatch", run)
		}
		if len(report.StrategyMetrics) != len(firstReport.StrategyMetrics) {
			t.Errorf("Run %d: StrategyMetrics length mismatch", run)
		}
		if len(report.SourceComparison) != len(firstReport.SourceComparison) {
			t.Errorf("Run %d: SourceComparison length mismatch", run)
		}
		if len(report.ScenarioSensitivity) != len(firstReport.ScenarioSensitivity) {
			t.Errorf("Run %d: ScenarioSensitivity length mismatch", run)
		}

		// Verify order is deterministic
		for i := range report.StrategyMetrics {
			if report.StrategyMetrics[i].StrategyID != firstReport.StrategyMetrics[i].StrategyID {
				t.Errorf("Run %d: StrategyMetrics[%d] StrategyID mismatch", run, i)
			}
			if report.StrategyMetrics[i].ScenarioID != firstReport.StrategyMetrics[i].ScenarioID {
				t.Errorf("Run %d: StrategyMetrics[%d] ScenarioID mismatch", run, i)
			}
			if report.StrategyMetrics[i].EntryEventType != firstReport.StrategyMetrics[i].EntryEventType {
				t.Errorf("Run %d: StrategyMetrics[%d] EntryEventType mismatch", run, i)
			}
		}
	}
}

func TestGenerate_WithClock(t *testing.T) {
	ctx := context.Background()
	candidateStore, tradeStore, aggStore := setupTestData(t)

	fixedTime := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	generator := NewGenerator(candidateStore, tradeStore, aggStore).WithClock(func() time.Time {
		return fixedTime
	})

	report, err := generator.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !report.GeneratedAt.Equal(fixedTime) {
		t.Errorf("Expected GeneratedAt %v, got %v", fixedTime, report.GeneratedAt)
	}
}

func TestGenerate_ContainsRequiredSections(t *testing.T) {
	ctx := context.Background()
	candidateStore, tradeStore, aggStore := setupTestData(t)
	generator := NewGenerator(candidateStore, tradeStore, aggStore)

	report, err := generator.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify all sections are present
	if report.StrategyCount == 0 {
		t.Error("StrategyCount should be > 0")
	}
	if report.ScenarioCount == 0 {
		t.Error("ScenarioCount should be > 0")
	}
	if report.DataSummary.TotalCandidates == 0 {
		t.Error("TotalCandidates should be > 0")
	}
	if len(report.StrategyMetrics) == 0 {
		t.Error("StrategyMetrics should not be empty")
	}
	if len(report.SourceComparison) == 0 {
		t.Error("SourceComparison should not be empty")
	}
	if len(report.ScenarioSensitivity) == 0 {
		t.Error("ScenarioSensitivity should not be empty")
	}
	if len(report.ReplayReferences) == 0 {
		t.Error("ReplayReferences should not be empty")
	}
}

func TestRenderMarkdown_Format(t *testing.T) {
	ctx := context.Background()
	candidateStore, tradeStore, aggStore := setupTestData(t)
	generator := NewGenerator(candidateStore, tradeStore, aggStore)

	report, err := generator.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	md := RenderMarkdown(report)

	// Verify required sections are in markdown
	requiredSections := []string{
		"# Phase 1 Report",
		"## Data Summary",
		"## Strategy Metrics",
		"## NEW_TOKEN vs ACTIVE_TOKEN Comparison",
		"## Scenario Sensitivity",
		"## Replay References",
	}

	for _, section := range requiredSections {
		if !strings.Contains(md, section) {
			t.Errorf("Markdown missing section: %s", section)
		}
	}

	// Verify tables are present (pipe characters)
	if !strings.Contains(md, "|") {
		t.Error("Markdown should contain tables with pipe characters")
	}
}

func TestRenderCSV_DeterministicOrder(t *testing.T) {
	metrics := []StrategyMetricRow{
		{StrategyID: "B", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN", TotalTrades: 10},
		{StrategyID: "A", ScenarioID: "realistic", EntryEventType: "NEW_TOKEN", TotalTrades: 5},
		{StrategyID: "A", ScenarioID: "pessimistic", EntryEventType: "NEW_TOKEN", TotalTrades: 3},
	}

	// Sort before rendering (as generator does)
	sortStrategyMetrics(metrics)

	csv := RenderCSV(metrics)
	lines := strings.Split(csv, "\n")

	// Header + 3 data rows + empty line
	if len(lines) < 4 {
		t.Fatalf("Expected at least 4 lines, got %d", len(lines))
	}

	// Verify header
	if !strings.HasPrefix(lines[0], "strategy_id,scenario_id,entry_event_type") {
		t.Error("CSV header is incorrect")
	}

	// Verify order: A,pessimistic < A,realistic < B,realistic
	if !strings.HasPrefix(lines[1], "A,pessimistic") {
		t.Errorf("Expected first row to be A,pessimistic, got: %s", lines[1])
	}
	if !strings.HasPrefix(lines[2], "A,realistic") {
		t.Errorf("Expected second row to be A,realistic, got: %s", lines[2])
	}
	if !strings.HasPrefix(lines[3], "B,realistic") {
		t.Errorf("Expected third row to be B,realistic, got: %s", lines[3])
	}
}

func TestSourceComparison_Correct(t *testing.T) {
	ctx := context.Background()
	candidateStore, tradeStore, aggStore := setupTestData(t)
	generator := NewGenerator(candidateStore, tradeStore, aggStore)

	report, err := generator.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Find TIME_EXIT + realistic comparison
	var found bool
	for _, c := range report.SourceComparison {
		if c.StrategyID == "TIME_EXIT" && c.ScenarioID == domain.ScenarioRealistic {
			found = true
			if c.NewTokenWinRate != 0.5 {
				t.Errorf("Expected NewTokenWinRate 0.5, got %.4f", c.NewTokenWinRate)
			}
			if c.ActiveTokenWinRate != 1.0 {
				t.Errorf("Expected ActiveTokenWinRate 1.0, got %.4f", c.ActiveTokenWinRate)
			}
			if c.NewTokenMedian != 0.025 {
				t.Errorf("Expected NewTokenMedian 0.025, got %.4f", c.NewTokenMedian)
			}
			if c.ActiveTokenMedian != 0.15 {
				t.Errorf("Expected ActiveTokenMedian 0.15, got %.4f", c.ActiveTokenMedian)
			}
			break
		}
	}
	if !found {
		t.Error("SourceComparison missing TIME_EXIT + realistic row")
	}
}

func TestScenarioSensitivity_Correct(t *testing.T) {
	ctx := context.Background()
	candidateStore, tradeStore, aggStore := setupTestData(t)
	generator := NewGenerator(candidateStore, tradeStore, aggStore)

	report, err := generator.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Find TIME_EXIT + NEW_TOKEN sensitivity
	var found bool
	for _, s := range report.ScenarioSensitivity {
		if s.StrategyID == "TIME_EXIT" && s.EntryEventType == "NEW_TOKEN" {
			found = true
			if s.RealisticMean != 0.025 {
				t.Errorf("Expected RealisticMean 0.025, got %.4f", s.RealisticMean)
			}
			if s.PessimisticMean != -0.10 {
				t.Errorf("Expected PessimisticMean -0.10, got %.4f", s.PessimisticMean)
			}
			if s.DegradedMean != -0.20 {
				t.Errorf("Expected DegradedMean -0.20, got %.4f", s.DegradedMean)
			}
			// DegradationPct = (0.025 - (-0.20)) / 0.025 * 100 = 900%
			expectedDegradation := (0.025 - (-0.20)) / 0.025 * 100
			if s.DegradationPct != expectedDegradation {
				t.Errorf("Expected DegradationPct %.2f, got %.2f", expectedDegradation, s.DegradationPct)
			}
			break
		}
	}
	if !found {
		t.Error("ScenarioSensitivity missing TIME_EXIT + NEW_TOKEN row")
	}
}
