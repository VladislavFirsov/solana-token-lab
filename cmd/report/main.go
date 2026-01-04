package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/pipeline"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/storage"
	chstore "solana-token-lab/internal/storage/clickhouse"
	"solana-token-lab/internal/storage/memory"
	pgstore "solana-token-lab/internal/storage/postgres"
)

func main() {
	// Parse flags
	outputDir := flag.String("output-dir", "docs", "Output directory for generated files")
	postgresDSN := flag.String("postgres-dsn", "", "PostgreSQL connection string (e.g., postgres://user:pass@host:5432/db)")
	clickhouseDSN := flag.String("clickhouse-dsn", "", "ClickHouse connection string (e.g., clickhouse://user:pass@host:9000/db)")
	useFixtures := flag.Bool("use-fixtures", false, "Use in-memory fixtures instead of database")
	flag.Parse()

	ctx := context.Background()

	// Validate flags
	if !*useFixtures && (*postgresDSN == "" || *clickhouseDSN == "") {
		fmt.Fprintln(os.Stderr, "Error: --postgres-dsn and --clickhouse-dsn are required when not using fixtures")
		fmt.Fprintln(os.Stderr, "Use --use-fixtures to run with demo data instead")
		os.Exit(1)
	}

	// Create stores based on mode
	var (
		candidateStore storage.CandidateStore
		tradeStore     storage.TradeRecordStore
		aggStore       storage.StrategyAggregateStore
		swapStore      storage.SwapStore
		liquidityStore storage.LiquidityEventStore
	)

	if *useFixtures {
		// Use memory stores with fixture data
		candidateStore, tradeStore, aggStore, swapStore, liquidityStore = createMemoryStores(ctx)
	} else {
		// Connect to databases
		var err error
		candidateStore, tradeStore, aggStore, swapStore, liquidityStore, err = createDatabaseStores(ctx, *postgresDSN, *clickhouseDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error connecting to databases: %v\n", err)
			os.Exit(1)
		}
	}

	// Create aggregator and compute aggregates (this collects missing candidates)
	aggregator := metrics.NewAggregator(tradeStore, aggStore, candidateStore)
	if err := computeAllAggregates(ctx, aggregator); err != nil {
		fmt.Fprintf(os.Stderr, "Error computing aggregates: %v\n", err)
		os.Exit(1)
	}

	// Define implementable strategies
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	// Create replay runner for replayability check
	replayRunner := replay.NewRunner(swapStore, liquidityStore)

	// Create pipeline with fixed clock for deterministic output
	// Pass aggregator to automatically collect missing candidate errors
	fixedTime := time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC)
	p := pipeline.NewPhase1Pipeline(
		candidateStore,
		tradeStore,
		aggStore,
		implementable,
		*outputDir,
	).WithSufficiencyChecker(
		candidateStore,
		tradeStore,
		swapStore,
		liquidityStore,
		replayRunner,
	).WithAggregator(aggregator).WithClock(func() time.Time { return fixedTime })

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

// createMemoryStores creates in-memory stores and loads fixture data.
func createMemoryStores(ctx context.Context) (
	storage.CandidateStore,
	storage.TradeRecordStore,
	storage.StrategyAggregateStore,
	storage.SwapStore,
	storage.LiquidityEventStore,
) {
	candidateStore := memory.NewCandidateStore()
	tradeStore := memory.NewTradeRecordStore()
	aggStore := memory.NewStrategyAggregateStore()
	swapStore := memory.NewSwapStore()
	liquidityStore := memory.NewLiquidityEventStore()

	// Load candidates and trades (not pre-computed aggregates)
	if err := pipeline.LoadCandidatesAndTrades(ctx, candidateStore, tradeStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading fixtures: %v\n", err)
		os.Exit(1)
	}

	// Load swap and liquidity fixtures (required for sufficiency check #5)
	if err := pipeline.LoadSwapsAndLiquidity(ctx, swapStore, liquidityStore); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swap/liquidity fixtures: %v\n", err)
		os.Exit(1)
	}

	return candidateStore, tradeStore, aggStore, swapStore, liquidityStore
}

// createDatabaseStores connects to PostgreSQL and ClickHouse and creates stores.
func createDatabaseStores(ctx context.Context, postgresDSN, clickhouseDSN string) (
	storage.CandidateStore,
	storage.TradeRecordStore,
	storage.StrategyAggregateStore,
	storage.SwapStore,
	storage.LiquidityEventStore,
	error,
) {
	// Connect to PostgreSQL
	pgPool, err := pgstore.NewPool(ctx, postgresDSN)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}

	// Connect to ClickHouse
	chConn, err := chstore.NewConn(ctx, clickhouseDSN)
	if err != nil {
		pgPool.Close()
		return nil, nil, nil, nil, nil, fmt.Errorf("connect to clickhouse: %w", err)
	}

	// Create Postgres stores (raw transactional data)
	candidateStore := pgstore.NewCandidateStore(pgPool)
	tradeStore := pgstore.NewTradeRecordStore(pgPool)
	swapStore := pgstore.NewSwapStore(pgPool)
	liquidityStore := pgstore.NewLiquidityEventStore(pgPool)

	// Create ClickHouse stores (aggregates)
	// Note: Phase 1 report only uses StrategyAggregateStore from ClickHouse.
	// Other ClickHouse stores (PriceTimeseriesStore, LiquidityTimeseriesStore,
	// VolumeTimeseriesStore, DerivedFeatureStore) are implemented but used by
	// the backtest/ingestion pipelines, not the report pipeline.
	aggStore := chstore.NewStrategyAggregateStore(chConn)

	return candidateStore, tradeStore, aggStore, swapStore, liquidityStore, nil
}

// computeAllAggregates computes aggregates for all strategy/scenario/entry combinations.
func computeAllAggregates(ctx context.Context, agg *metrics.Aggregator) error {
	strategies := []string{domain.StrategyTypeTimeExit}
	scenarios := []string{domain.ScenarioRealistic, domain.ScenarioPessimistic, domain.ScenarioDegraded}
	entryTypes := []string{"NEW_TOKEN", "ACTIVE_TOKEN"}

	for _, strategy := range strategies {
		for _, scenario := range scenarios {
			for _, entry := range entryTypes {
				_, err := agg.ComputeAndStore(ctx, strategy, scenario, entry)
				if err != nil && !errors.Is(err, metrics.ErrNoTrades) {
					return fmt.Errorf("compute aggregate %s/%s/%s: %w", strategy, scenario, entry, err)
				}
				// ErrNoTrades is expected for some combinations (e.g., no pessimistic trades in fixtures)
			}
		}
	}
	return nil
}
