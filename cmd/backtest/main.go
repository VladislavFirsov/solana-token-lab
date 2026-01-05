package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/simulation"
	"solana-token-lab/internal/storage"
	chstore "solana-token-lab/internal/storage/clickhouse"
	"solana-token-lab/internal/storage/memory"
	pgstore "solana-token-lab/internal/storage/postgres"
)

func main() {
	// Parse flags
	candidateID := flag.String("candidate-id", "", "Candidate ID to backtest (required)")
	strategyType := flag.String("strategy", "", "Strategy: TIME_EXIT, TRAILING_STOP, LIQUIDITY_GUARD (required)")
	scenarioName := flag.String("scenario", "realistic", "Scenario: optimistic, realistic, pessimistic, degraded")
	entryEventType := flag.String("entry-event", "NEW_TOKEN", "Entry event type: NEW_TOKEN, ACTIVE_TOKEN")

	// Strategy parameters
	holdDurationMs := flag.Int64("hold-duration-ms", 300000, "Hold duration for TIME_EXIT (ms)")
	trailPct := flag.Float64("trail-pct", 0.10, "Trail percentage for TRAILING_STOP")
	initialStopPct := flag.Float64("initial-stop-pct", 0.10, "Initial stop for TRAILING_STOP")
	liquidityDropPct := flag.Float64("liquidity-drop-pct", 0.30, "Liquidity drop for LIQUIDITY_GUARD")
	maxHoldMs := flag.Int64("max-hold-ms", 3600000, "Max hold duration (ms)")

	// Storage
	postgresDSN := flag.String("postgres-dsn", "", "PostgreSQL connection string")
	clickhouseDSN := flag.String("clickhouse-dsn", "", "ClickHouse connection string")
	useMemory := flag.Bool("use-memory", false, "Use in-memory storage")

	// Output
	outputJSON := flag.Bool("json", false, "Output as JSON")
	persistResult := flag.Bool("persist", false, "Persist trade record to storage")

	flag.Parse()

	// Setup logger
	logger := log.New(os.Stderr, "[backtest] ", log.LstdFlags)

	// Validate required flags
	if *candidateID == "" {
		logger.Fatal("--candidate-id is required")
	}
	if *strategyType == "" {
		logger.Fatal("--strategy is required")
	}

	// Normalize strategy type
	*strategyType = strings.ToUpper(*strategyType)
	if *strategyType != domain.StrategyTypeTimeExit &&
		*strategyType != domain.StrategyTypeTrailingStop &&
		*strategyType != domain.StrategyTypeLiquidityGuard {
		logger.Fatalf("Invalid strategy: %s. Must be TIME_EXIT, TRAILING_STOP, or LIQUIDITY_GUARD", *strategyType)
	}

	// Normalize and validate entry event type
	*entryEventType = strings.ToUpper(*entryEventType)
	if *entryEventType != "NEW_TOKEN" && *entryEventType != "ACTIVE_TOKEN" {
		logger.Fatalf("Invalid entry event type: %s. Must be NEW_TOKEN or ACTIVE_TOKEN", *entryEventType)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	// Create stores
	var candidateStore storage.CandidateStore = memory.NewCandidateStore()
	var priceStore storage.PriceTimeseriesStore = memory.NewPriceTimeseriesStore()
	var liqStore storage.LiquidityTimeseriesStore = memory.NewLiquidityTimeseriesStore()
	var tradeStore storage.TradeRecordStore = memory.NewTradeRecordStore()

	if !*useMemory {
		// Require DSNs when not using memory
		if *postgresDSN == "" {
			logger.Fatal("--postgres-dsn is required when not using --use-memory (candidates and trade records)")
		}
		if *clickhouseDSN == "" {
			logger.Fatal("--clickhouse-dsn is required when not using --use-memory (price/liquidity timeseries)")
		}

		// PostgreSQL for candidates and trade records
		pool, err := pgstore.NewPool(ctx, *postgresDSN)
		if err != nil {
			logger.Fatalf("connect to postgres: %v", err)
		}
		defer pool.Close()

		candidateStore = pgstore.NewCandidateStore(pool)
		tradeStore = pgstore.NewTradeRecordStore(pool)

		// ClickHouse for time series
		conn, err := chstore.NewConn(ctx, *clickhouseDSN)
		if err != nil {
			logger.Fatalf("connect to clickhouse: %v", err)
		}
		defer conn.Close()

		priceStore = chstore.NewPriceTimeseriesStore(conn)
		liqStore = chstore.NewLiquidityTimeseriesStore(conn)
	}

	// Build strategy config
	strategyConfig := buildStrategyConfig(
		*strategyType,
		*entryEventType,
		*holdDurationMs,
		*trailPct,
		*initialStopPct,
		*liquidityDropPct,
		*maxHoldMs,
	)

	// Get scenario config
	scenarioConfig := getScenarioConfig(*scenarioName)
	if scenarioConfig == nil {
		logger.Fatalf("Invalid scenario: %s. Must be optimistic, realistic, pessimistic, or degraded", *scenarioName)
	}

	// Create simulation runner
	var tradeRecordStore storage.TradeRecordStore
	if *persistResult {
		tradeRecordStore = tradeStore
	}

	runner := simulation.NewRunner(simulation.RunnerOptions{
		CandidateStore:       candidateStore,
		PriceTimeseriesStore: priceStore,
		LiqTimeseriesStore:   liqStore,
		TradeRecordStore:     tradeRecordStore,
	})

	// Run simulation
	logger.Printf("Running backtest: candidate=%s strategy=%s scenario=%s",
		*candidateID, *strategyType, *scenarioName)

	trade, err := runner.Run(ctx, *candidateID, strategyConfig, *scenarioConfig)
	if err != nil {
		logger.Fatalf("backtest failed: %v", err)
	}

	// Output result
	if *outputJSON {
		output, _ := json.MarshalIndent(trade, "", "  ")
		fmt.Println(string(output))
	} else {
		printTradeRecord(trade)
	}
}

// buildStrategyConfig creates a StrategyConfig from CLI flags.
func buildStrategyConfig(
	strategyType, entryEventType string,
	holdDurationMs int64,
	trailPct, initialStopPct, liquidityDropPct float64,
	maxHoldMs int64,
) domain.StrategyConfig {
	cfg := domain.StrategyConfig{
		StrategyType:   strategyType,
		EntryEventType: entryEventType,
	}

	switch strategyType {
	case domain.StrategyTypeTimeExit:
		cfg.HoldDurationMs = &holdDurationMs
	case domain.StrategyTypeTrailingStop:
		cfg.TrailPct = &trailPct
		cfg.InitialStopPct = &initialStopPct
		cfg.MaxHoldDurationMs = &maxHoldMs
	case domain.StrategyTypeLiquidityGuard:
		cfg.LiquidityDropPct = &liquidityDropPct
		cfg.MaxHoldDurationMs = &maxHoldMs
	}

	return cfg
}

// getScenarioConfig returns the predefined scenario config by name.
func getScenarioConfig(name string) *domain.ScenarioConfig {
	switch strings.ToLower(name) {
	case "optimistic":
		return &domain.ScenarioConfigOptimistic
	case "realistic":
		return &domain.ScenarioConfigRealistic
	case "pessimistic":
		return &domain.ScenarioConfigPessimistic
	case "degraded":
		return &domain.ScenarioConfigDegraded
	default:
		return nil
	}
}

// printTradeRecord outputs human-readable trade record.
func printTradeRecord(t *domain.TradeRecord) {
	fmt.Println()
	fmt.Println("=== Backtest Result ===")
	fmt.Printf("Trade ID:           %s\n", t.TradeID)
	fmt.Printf("Candidate ID:       %s\n", t.CandidateID)
	fmt.Printf("Strategy:           %s\n", t.StrategyID)
	fmt.Printf("Scenario:           %s\n", t.ScenarioID)
	fmt.Println()

	fmt.Println("Entry:")
	fmt.Printf("  Signal Time:      %s\n", time.UnixMilli(t.EntrySignalTime).Format(time.RFC3339Nano))
	fmt.Printf("  Signal Price:     %.8f\n", t.EntrySignalPrice)
	fmt.Printf("  Actual Time:      %s\n", time.UnixMilli(t.EntryActualTime).Format(time.RFC3339Nano))
	fmt.Printf("  Actual Price:     %.8f\n", t.EntryActualPrice)
	if t.EntryLiquidity != nil {
		fmt.Printf("  Liquidity:        %.2f\n", *t.EntryLiquidity)
	}
	fmt.Println()

	fmt.Println("Exit:")
	fmt.Printf("  Signal Time:      %s\n", time.UnixMilli(t.ExitSignalTime).Format(time.RFC3339Nano))
	fmt.Printf("  Signal Price:     %.8f\n", t.ExitSignalPrice)
	fmt.Printf("  Actual Time:      %s\n", time.UnixMilli(t.ExitActualTime).Format(time.RFC3339Nano))
	fmt.Printf("  Actual Price:     %.8f\n", t.ExitActualPrice)
	fmt.Printf("  Reason:           %s\n", t.ExitReason)
	fmt.Println()

	fmt.Println("Costs:")
	fmt.Printf("  Entry Cost:       %.6f SOL\n", t.EntryCostSOL)
	fmt.Printf("  Exit Cost:        %.6f SOL\n", t.ExitCostSOL)
	fmt.Printf("  MEV Cost:         %.6f SOL\n", t.MEVCostSOL)
	fmt.Printf("  Total Cost:       %.6f SOL (%.2f%%)\n", t.TotalCostSOL, t.TotalCostPct*100)
	fmt.Println()

	fmt.Println("Result:")
	fmt.Printf("  Gross Return:     %.2f%%\n", t.GrossReturn*100)
	fmt.Printf("  Net Outcome:      %.2f%%\n", t.Outcome*100)
	fmt.Printf("  Outcome Class:    %s\n", t.OutcomeClass)
	fmt.Printf("  Hold Duration:    %v\n", time.Duration(t.HoldDurationMs)*time.Millisecond)
	if t.PeakPrice != nil {
		fmt.Printf("  Peak Price:       %.8f\n", *t.PeakPrice)
	}
	if t.MinLiquidity != nil {
		fmt.Printf("  Min Liquidity:    %.2f\n", *t.MinLiquidity)
	}
}
