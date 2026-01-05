package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	expectedDataVersion := flag.String("data-version", "", "Expected data version hash (validates data integrity if provided)")
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
	// TIME_EXIT is implementable (simple time-based exit)
	// TRAILING_STOP and LIQUIDITY_GUARD require real-time price/liquidity feeds - not yet implementable
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:          true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}:       true,
		{StrategyID: "TRAILING_STOP", EntryEventType: "NEW_TOKEN"}:      false,
		{StrategyID: "TRAILING_STOP", EntryEventType: "ACTIVE_TOKEN"}:   false,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "NEW_TOKEN"}:    false,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "ACTIVE_TOKEN"}: false,
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

	// Set data source for replay command
	if *useFixtures {
		p = p.WithDataSource("fixtures")
	} else {
		p = p.WithDBSource(*postgresDSN, *clickhouseDSN)
	}

	// Run pipeline
	if err := p.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error running pipeline: %v\n", err)
		os.Exit(1)
	}

	// Validate data version if provided (for reproducibility verification)
	if *expectedDataVersion != "" {
		actualDataVersion, err := readDataVersionFromMetadata(*outputDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading data version: %v\n", err)
			os.Exit(1)
		}
		if actualDataVersion != *expectedDataVersion {
			fmt.Fprintf(os.Stderr, "ERROR: Data version mismatch!\n")
			fmt.Fprintf(os.Stderr, "  Expected: %s\n", *expectedDataVersion)
			fmt.Fprintf(os.Stderr, "  Actual:   %s\n", actualDataVersion)
			fmt.Fprintf(os.Stderr, "\nThis indicates the underlying data has changed since the original report.\n")
			fmt.Fprintf(os.Stderr, "Reproducibility requirement violated.\n")
			os.Exit(2)
		}
		fmt.Printf("Data version validated: %s\n", actualDataVersion)
	}

	fmt.Println("Phase 1 report generated successfully:")
	fmt.Printf("  - %s/REPORT_PHASE1.md\n", *outputDir)
	fmt.Printf("  - %s/strategy_aggregates.csv\n", *outputDir)
	fmt.Printf("  - %s/trade_records.csv\n", *outputDir)
	fmt.Printf("  - %s/scenario_outcomes.csv\n", *outputDir)
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
// Idempotent: ignores ErrDuplicateKey if aggregate already exists.
func computeAllAggregates(ctx context.Context, agg *metrics.Aggregator) error {
	// All 3 strategies per STRATEGY_CATALOG.md
	strategies := []string{
		domain.StrategyTypeTimeExit,
		domain.StrategyTypeTrailingStop,
		domain.StrategyTypeLiquidityGuard,
	}
	// All 4 scenarios per EXECUTION_SCENARIOS.md
	scenarios := []string{
		domain.ScenarioOptimistic,
		domain.ScenarioRealistic,
		domain.ScenarioPessimistic,
		domain.ScenarioDegraded,
	}
	entryTypes := []string{"NEW_TOKEN", "ACTIVE_TOKEN"}

	for _, strategy := range strategies {
		for _, scenario := range scenarios {
			for _, entry := range entryTypes {
				_, err := agg.ComputeAndStore(ctx, strategy, scenario, entry)
				if err != nil {
					// ErrNoTrades is expected for some combinations (e.g., no trades in fixtures)
					if errors.Is(err, metrics.ErrNoTrades) {
						continue
					}
					// ErrDuplicateKey means aggregate already exists - idempotent behavior
					if errors.Is(err, storage.ErrDuplicateKey) {
						continue
					}
					return fmt.Errorf("compute aggregate %s/%s/%s: %w", strategy, scenario, entry, err)
				}
			}
		}
	}
	return nil
}

// readDataVersionFromMetadata reads the data_version from metadata.json.
func readDataVersionFromMetadata(outputDir string) (string, error) {
	metadataPath := filepath.Join(outputDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return "", fmt.Errorf("read metadata.json: %w", err)
	}

	var metadata struct {
		DataVersion string `json:"data_version"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return "", fmt.Errorf("parse metadata.json: %w", err)
	}

	if metadata.DataVersion == "" {
		return "", fmt.Errorf("data_version not found in metadata.json")
	}

	return metadata.DataVersion, nil
}
