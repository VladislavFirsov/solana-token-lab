package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/storage/memory"
)

func TestPhase1Pipeline_Run(t *testing.T) {
	// Create temp directory for output
	tempDir, err := os.MkdirTemp("", "phase1_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create stores and load fixtures
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	ctx := context.Background()
	if err := LoadFixtures(ctx, candidateStore, tradeStore, aggStore); err != nil {
		t.Fatalf("Failed to load fixtures: %v", err)
	}

	// Define implementable strategies
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	// Create pipeline with fixed clock
	fixedTime := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)
	p := NewPhase1Pipeline(
		candidateStore,
		tradeStore,
		aggStore,
		implementable,
		tempDir,
	).WithClock(func() time.Time { return fixedTime })

	// Run pipeline
	if err := p.Run(ctx); err != nil {
		t.Fatalf("Pipeline run failed: %v", err)
	}

	// Verify all files exist
	files := []string{"REPORT_PHASE1.md", "STRATEGY_AGGREGATES.csv", "DECISION_GATE_REPORT.md"}
	for _, f := range files {
		path := filepath.Join(tempDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", f)
		}
	}
}

func TestPhase1Pipeline_Deterministic(t *testing.T) {
	ctx := context.Background()
	fixedTime := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)

	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	var outputs []map[string]string

	// Run pipeline twice
	for run := 0; run < 2; run++ {
		tempDir, err := os.MkdirTemp("", "phase1_determ_test")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Fresh stores each run
		candidateStore := memory.NewCandidateStore()
		tradeStore := memory.NewTradeRecordStore()
		aggStore := memory.NewStrategyAggregateStore()

		if err := LoadFixtures(ctx, candidateStore, tradeStore, aggStore); err != nil {
			t.Fatalf("Failed to load fixtures: %v", err)
		}

		p := NewPhase1Pipeline(
			candidateStore,
			tradeStore,
			aggStore,
			implementable,
			tempDir,
		).WithClock(func() time.Time { return fixedTime })

		if err := p.Run(ctx); err != nil {
			t.Fatalf("Run %d failed: %v", run, err)
		}

		// Read all output files
		runOutput := make(map[string]string)
		files := []string{"REPORT_PHASE1.md", "STRATEGY_AGGREGATES.csv", "DECISION_GATE_REPORT.md"}
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(tempDir, f))
			if err != nil {
				t.Fatalf("Run %d: failed to read %s: %v", run, f, err)
			}
			runOutput[f] = string(data)
		}
		outputs = append(outputs, runOutput)
	}

	// Compare outputs
	for _, f := range []string{"REPORT_PHASE1.md", "STRATEGY_AGGREGATES.csv", "DECISION_GATE_REPORT.md"} {
		if outputs[0][f] != outputs[1][f] {
			t.Errorf("File %s is not deterministic between runs", f)
		}
	}
}

func TestPhase1Pipeline_OutputFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "phase1_format_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	ctx := context.Background()
	if err := LoadFixtures(ctx, candidateStore, tradeStore, aggStore); err != nil {
		t.Fatalf("Failed to load fixtures: %v", err)
	}

	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	fixedTime := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)
	p := NewPhase1Pipeline(
		candidateStore,
		tradeStore,
		aggStore,
		implementable,
		tempDir,
	).WithClock(func() time.Time { return fixedTime })

	if err := p.Run(ctx); err != nil {
		t.Fatalf("Pipeline run failed: %v", err)
	}

	// Verify REPORT_PHASE1.md format
	reportData, _ := os.ReadFile(filepath.Join(tempDir, "REPORT_PHASE1.md"))
	report := string(reportData)
	if !strings.Contains(report, "# Phase 1 Report") {
		t.Error("Report should contain header")
	}
	if !strings.Contains(report, "## Data Summary") {
		t.Error("Report should contain Data Summary section")
	}
	if !strings.Contains(report, "## Strategy Metrics") {
		t.Error("Report should contain Strategy Metrics section")
	}

	// Verify STRATEGY_AGGREGATES.csv format
	csvData, _ := os.ReadFile(filepath.Join(tempDir, "STRATEGY_AGGREGATES.csv"))
	csv := string(csvData)
	if !strings.HasPrefix(csv, "strategy_id,scenario_id,entry_event_type,") {
		t.Error("CSV should have proper header")
	}
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	if len(lines) < 2 {
		t.Error("CSV should have header + at least one data row")
	}

	// Verify DECISION_GATE_REPORT.md format
	decisionData, _ := os.ReadFile(filepath.Join(tempDir, "DECISION_GATE_REPORT.md"))
	decisionReport := string(decisionData)
	if !strings.Contains(decisionReport, "# Phase 1 Decision Gate Report") {
		t.Error("Decision report should contain header")
	}
	if !strings.Contains(decisionReport, "Generated at: 2025-01-04 12:00:00 UTC") {
		t.Error("Decision report should contain fixed timestamp")
	}
	if !strings.Contains(decisionReport, "## Strategy: TIME_EXIT") {
		t.Error("Decision report should contain strategy section")
	}
	// Should contain GO decision for TIME_EXIT|NEW_TOKEN (good metrics)
	if !strings.Contains(decisionReport, "Decision: GO") {
		t.Error("Decision report should contain GO decision for good strategy")
	}
}
