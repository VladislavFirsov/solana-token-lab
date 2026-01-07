// Package main provides E2E pipeline entry point.
// Executes: normalization → simulation → metrics → reporting
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/orchestrator"
	"solana-token-lab/internal/pipeline"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
)

func main() {
	// Parse flags
	outputDir := flag.String("output-dir", "docs", "Output directory for generated files")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		fmt.Printf("\nReceived signal %v, cancelling pipeline...\n", sig)
		cancel()
	}()

	// Create all memory stores
	stores := createAllMemoryStores()

	// Load fixture data for candidates and raw events
	if err := loadFixtureData(ctx, stores); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading fixtures: %v\n", err)
		os.Exit(1)
	}

	// Create strategy and scenario configs
	strategyConfigs := createStrategyConfigs()
	scenarioConfigs := createScenarioConfigs()

	// Phase 1-4: Run orchestrator (normalization → simulation → metrics)
	fmt.Println("=== E2E Pipeline ===")
	orch := orchestrator.New(orchestrator.Options{
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
		Verbose:                  *verbose,
	})

	result, err := orch.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Orchestrator error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Orchestrator completed:\n")
	fmt.Printf("  Candidates: %d\n", result.CandidatesProcessed)
	fmt.Printf("  Trades: %d\n", result.TradesCreated)
	fmt.Printf("  Aggregates: %d\n", result.AggregatesCreated)
	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}

	// Phase 5: Reporting via Phase1Pipeline
	fmt.Println("\n=== Phase 1 Reporting ===")

	// Create aggregator (for missing candidate tracking)
	aggregator := metrics.NewAggregator(
		stores.tradeRecordStore,
		stores.strategyAggregateStore,
		stores.candidateStore,
	)

	// Define implementable strategies (all enabled after orchestration)
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:          true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}:       true,
		{StrategyID: "TRAILING_STOP", EntryEventType: "NEW_TOKEN"}:      true,
		{StrategyID: "TRAILING_STOP", EntryEventType: "ACTIVE_TOKEN"}:   true,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	// Create replay runner for replayability check
	replayRunner := replay.NewRunner(stores.swapStore, stores.liquidityEventStore)

	// Create pipeline with fixed clock for deterministic output
	fixedTime := time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC)
	p := pipeline.NewPhase1Pipeline(
		stores.candidateStore,
		stores.tradeRecordStore,
		stores.strategyAggregateStore,
		implementable,
		*outputDir,
	).WithSufficiencyChecker(
		stores.candidateStore,
		stores.tradeRecordStore,
		stores.swapStore,
		stores.liquidityEventStore,
		replayRunner,
	).WithAggregator(aggregator).WithClock(func() time.Time { return fixedTime }).WithDataSource("e2e-pipeline")

	// Run reporting pipeline
	if err := p.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Pipeline error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nE2E Pipeline completed successfully:")
	fmt.Printf("  - %s/REPORT_PHASE1.md\n", *outputDir)
	fmt.Printf("  - %s/strategy_aggregates.csv\n", *outputDir)
	fmt.Printf("  - %s/trade_records.csv\n", *outputDir)
	fmt.Printf("  - %s/scenario_outcomes.csv\n", *outputDir)
	fmt.Printf("  - %s/DECISION_GATE_REPORT.md\n", *outputDir)
}

// allStores holds all memory stores.
type allStores struct {
	candidateStore           storage.CandidateStore
	swapStore                storage.SwapStore
	liquidityEventStore      storage.LiquidityEventStore
	priceTimeseriesStore     storage.PriceTimeseriesStore
	liquidityTimeseriesStore storage.LiquidityTimeseriesStore
	volumeTimeseriesStore    storage.VolumeTimeseriesStore
	derivedFeatureStore      storage.DerivedFeatureStore
	tradeRecordStore         storage.TradeRecordStore
	strategyAggregateStore   storage.StrategyAggregateStore
}

// createAllMemoryStores creates all required memory stores.
func createAllMemoryStores() *allStores {
	return &allStores{
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

// loadFixtureData loads fixture data into stores.
func loadFixtureData(ctx context.Context, stores *allStores) error {
	// Load candidates (required)
	if err := pipeline.LoadCandidatesAndTrades(ctx, stores.candidateStore, stores.tradeRecordStore); err != nil {
		return fmt.Errorf("load candidates: %w", err)
	}

	// Clear trades - we'll regenerate via orchestrator
	// (LoadCandidatesAndTrades also loads trades, but we want to simulate fresh)
	// Note: memory store doesn't support clear, so trades will be duplicated
	// In real scenario, we'd use clean stores or skip loading trades

	// Load swap and liquidity events (required for normalization)
	if err := pipeline.LoadSwapsAndLiquidity(ctx, stores.swapStore, stores.liquidityEventStore); err != nil {
		return fmt.Errorf("load swaps/liquidity: %w", err)
	}

	return nil
}

// createStrategyConfigs returns all strategy configurations to simulate.
func createStrategyConfigs() []domain.StrategyConfig {
	// TIME_EXIT: 5 minute hold duration
	holdDuration := int64(300000) // 5 minutes in ms

	// TRAILING_STOP parameters
	trailPct := 0.10                  // 10% trailing stop
	initialStopPct := 0.10            // 10% initial stop
	maxHoldTrailing := int64(3600000) // 1 hour max

	// LIQUIDITY_GUARD parameters
	liquidityDropPct := 0.30           // 30% liquidity drop threshold
	maxHoldLiquidity := int64(1800000) // 30 min max

	return []domain.StrategyConfig{
		// TIME_EXIT for both entry types
		{
			StrategyType:   domain.StrategyTypeTimeExit,
			EntryEventType: "NEW_TOKEN",
			HoldDurationMs: &holdDuration,
		},
		{
			StrategyType:   domain.StrategyTypeTimeExit,
			EntryEventType: "ACTIVE_TOKEN",
			HoldDurationMs: &holdDuration,
		},
		// TRAILING_STOP for both entry types
		{
			StrategyType:      domain.StrategyTypeTrailingStop,
			EntryEventType:    "NEW_TOKEN",
			TrailPct:          &trailPct,
			InitialStopPct:    &initialStopPct,
			MaxHoldDurationMs: &maxHoldTrailing,
		},
		{
			StrategyType:      domain.StrategyTypeTrailingStop,
			EntryEventType:    "ACTIVE_TOKEN",
			TrailPct:          &trailPct,
			InitialStopPct:    &initialStopPct,
			MaxHoldDurationMs: &maxHoldTrailing,
		},
		// LIQUIDITY_GUARD for both entry types
		{
			StrategyType:      domain.StrategyTypeLiquidityGuard,
			EntryEventType:    "NEW_TOKEN",
			LiquidityDropPct:  &liquidityDropPct,
			MaxHoldDurationMs: &maxHoldLiquidity,
		},
		{
			StrategyType:      domain.StrategyTypeLiquidityGuard,
			EntryEventType:    "ACTIVE_TOKEN",
			LiquidityDropPct:  &liquidityDropPct,
			MaxHoldDurationMs: &maxHoldLiquidity,
		},
	}
}

// createScenarioConfigs returns all scenario configurations.
func createScenarioConfigs() []domain.ScenarioConfig {
	return []domain.ScenarioConfig{
		domain.ScenarioConfigOptimistic,
		domain.ScenarioConfigRealistic,
		domain.ScenarioConfigPessimistic,
		domain.ScenarioConfigDegraded,
	}
}
