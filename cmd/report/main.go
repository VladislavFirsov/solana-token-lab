package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/pipeline"
	"solana-token-lab/internal/storage/memory"
)

func main() {
	// Parse flags
	outputDir := flag.String("output-dir", "docs", "Output directory for generated files")
	flag.Parse()

	// Create stores
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()

	// Load fixtures
	ctx := context.Background()
	if err := pipeline.LoadFixtures(ctx, candidateStore, tradeStore, aggStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading fixtures: %v\n", err)
		os.Exit(1)
	}

	// Define implementable strategies
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	// Create pipeline with fixed clock for deterministic output
	fixedTime := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)
	p := pipeline.NewPhase1Pipeline(
		candidateStore,
		tradeStore,
		aggStore,
		implementable,
		*outputDir,
	).WithClock(func() time.Time { return fixedTime })

	// Run pipeline
	if err := p.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running pipeline: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Phase 1 report generated successfully:")
	fmt.Printf("  - %s/REPORT_PHASE1.md\n", *outputDir)
	fmt.Printf("  - %s/STRATEGY_AGGREGATES.csv\n", *outputDir)
	fmt.Printf("  - %s/DECISION_GATE_REPORT.md\n", *outputDir)
}
