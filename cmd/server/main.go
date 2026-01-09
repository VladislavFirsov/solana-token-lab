// Package main provides unified server that runs all components together:
// - Ingestion (continuous): WebSocket feeds, discovery, metadata
// - Pipeline (scheduled): normalization → simulation → metrics
// - Reporting (scheduled): REPORT_PHASE1.md, CSVs, DECISION_GATE
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"solana-token-lab/internal/decision"
	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/ingestion"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/observability"
	"solana-token-lab/internal/orchestrator"
	"solana-token-lab/internal/pipeline"
	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
	chstore "solana-token-lab/internal/storage/clickhouse"
	"solana-token-lab/internal/storage/memory"
	pgstore "solana-token-lab/internal/storage/postgres"
)

// DEX program aliases mapped to program IDs.
var dexAliases = map[string]string{
	"raydium": discovery.RaydiumAMMV4,
	"pumpfun": discovery.PumpFun,
}

// Server holds all components of the unified service.
type Server struct {
	// Configuration
	rpcEndpoint      string
	wsEndpoint       string
	postgresDSN      string
	clickhouseDSN    string
	useMemory        bool
	programs         []string
	outputDir        string
	pipelineInterval time.Duration
	reportInterval   time.Duration
	checkInterval    time.Duration

	// Stores
	stores *allStores

	// Components
	ingestionRunner *ingestion.Runner
	logger          *log.Logger

	// State
	mu               sync.Mutex
	lastPipelineRun  time.Time
	lastReportRun    time.Time
	pipelineRunning  bool
	reportRunning    bool
	ingestionStarted time.Time

	// Stats
	pipelineRuns int
	reportRuns   int
}

// allStores holds all storage implementations.
type allStores struct {
	candidateStore           storage.CandidateStore
	swapStore                storage.SwapStore
	swapEventStore           storage.SwapEventStore
	liquidityEventStore      storage.LiquidityEventStore
	priceTimeseriesStore     storage.PriceTimeseriesStore
	liquidityTimeseriesStore storage.LiquidityTimeseriesStore
	volumeTimeseriesStore    storage.VolumeTimeseriesStore
	derivedFeatureStore      storage.DerivedFeatureStore
	tradeRecordStore         storage.TradeRecordStore
	strategyAggregateStore   storage.StrategyAggregateStore
	metadataStore            storage.TokenMetadataStore
}

func main() {
	// Load .env file if exists
	loadEnvFile()

	// Parse flags (env vars as defaults)
	rpcEndpoint := flag.String("rpc-endpoint", os.Getenv("SOLANA_RPC_ENDPOINT"), "Solana RPC HTTP endpoint")
	wsEndpoint := flag.String("ws-endpoint", os.Getenv("SOLANA_WS_ENDPOINT"), "Solana WebSocket endpoint")
	postgresDSN := flag.String("postgres-dsn", os.Getenv("POSTGRES_DSN"), "PostgreSQL connection string")
	clickhouseDSN := flag.String("clickhouse-dsn", os.Getenv("CLICKHOUSE_DSN"), "ClickHouse connection string")
	programs := flag.String("programs", "", "Comma-separated DEX program IDs to monitor")
	dex := flag.String("dex", "raydium,pumpfun", "Comma-separated DEX aliases (raydium, pumpfun)")
	outputDir := flag.String("output-dir", "output", "Output directory for reports")
	pipelineInterval := flag.Duration("pipeline-interval", 1*time.Hour, "Pipeline run interval")
	reportInterval := flag.Duration("report-interval", 6*time.Hour, "Report generation interval")
	checkInterval := flag.Duration("check-interval", 1*time.Hour, "ACTIVE_TOKEN detection interval")
	useMemory := flag.Bool("use-memory", false, "Use in-memory storage instead of PostgreSQL")
	metricsAddr := flag.String("metrics-addr", ":9090", "Prometheus metrics HTTP address")

	flag.Parse()

	// Setup logger
	logger := log.New(os.Stdout, "[server] ", log.LstdFlags|log.Lshortfile)

	// Validate required flags
	if *rpcEndpoint == "" {
		logger.Fatal("--rpc-endpoint is required")
	}
	if *wsEndpoint == "" {
		logger.Fatal("--ws-endpoint is required")
	}
	if !*useMemory && (*postgresDSN == "" || *clickhouseDSN == "") {
		logger.Fatal("--postgres-dsn and --clickhouse-dsn are required (use --use-memory for in-memory storage)")
	}

	// Resolve DEX programs
	programList := resolvePrograms(*programs, *dex)
	if len(programList) == 0 {
		logger.Fatal("No DEX programs specified. Use --programs or --dex")
	}
	logger.Printf("Monitoring DEX programs: %v", programList)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Create stores
	stores, cleanup, err := createStores(ctx, *postgresDSN, *clickhouseDSN, *useMemory)
	if err != nil {
		logger.Fatalf("Failed to create stores: %v", err)
	}
	defer cleanup()

	// Create server
	server := &Server{
		rpcEndpoint:      *rpcEndpoint,
		wsEndpoint:       *wsEndpoint,
		postgresDSN:      *postgresDSN,
		clickhouseDSN:    *clickhouseDSN,
		useMemory:        *useMemory,
		programs:         programList,
		outputDir:        *outputDir,
		pipelineInterval: *pipelineInterval,
		reportInterval:   *reportInterval,
		checkInterval:    *checkInterval,
		stores:           stores,
		logger:           logger,
	}

	// Channel to signal completion
	done := make(chan error, 1)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()

		// Wait for second signal for immediate shutdown
		select {
		case sig := <-sigCh:
			logger.Printf("Received second signal %v, forcing immediate shutdown", sig)
			os.Exit(1)
		case <-time.After(30 * time.Second):
			logger.Println("Graceful shutdown timed out after 30s, forcing exit")
			os.Exit(1)
		case <-done:
			// Normal shutdown completed
		}
	}()

	// Start HTTP server
	go server.startHTTPServer(*metricsAddr)

	// Run the unified server
	err = server.Run(ctx)
	done <- err
	cancel()

	if err != nil && err != context.Canceled {
		logger.Fatalf("Server error: %v", err)
	}

	logger.Println("Shutdown complete")
}

// resolvePrograms resolves program IDs from flags.
func resolvePrograms(programs, dex string) []string {
	result := make(map[string]bool)

	// Add explicit programs
	if programs != "" {
		for _, p := range strings.Split(programs, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				result[p] = true
			}
		}
	}

	// Add programs from DEX aliases
	if dex != "" {
		for _, alias := range strings.Split(dex, ",") {
			alias = strings.TrimSpace(strings.ToLower(alias))
			if programID, ok := dexAliases[alias]; ok {
				result[programID] = true
			}
		}
	}

	// Convert to slice
	list := make([]string, 0, len(result))
	for p := range result {
		list = append(list, p)
	}
	return list
}

// createStores creates all required stores.
func createStores(ctx context.Context, postgresDSN, clickhouseDSN string, useMemory bool) (*allStores, func(), error) {
	if useMemory {
		stores := &allStores{
			candidateStore:           memory.NewCandidateStore(),
			swapStore:                memory.NewSwapStore(),
			swapEventStore:           memory.NewSwapEventStore(),
			liquidityEventStore:      memory.NewLiquidityEventStore(),
			priceTimeseriesStore:     memory.NewPriceTimeseriesStore(),
			liquidityTimeseriesStore: memory.NewLiquidityTimeseriesStore(),
			volumeTimeseriesStore:    memory.NewVolumeTimeseriesStore(),
			derivedFeatureStore:      memory.NewDerivedFeatureStore(),
			tradeRecordStore:         memory.NewTradeRecordStore(),
			strategyAggregateStore:   memory.NewStrategyAggregateStore(),
			metadataStore:            memory.NewTokenMetadataStore(),
		}
		return stores, func() {}, nil
	}

	// PostgreSQL
	pool, err := pgstore.NewPool(ctx, postgresDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}

	// ClickHouse
	chConn, err := chstore.NewConn(ctx, clickhouseDSN)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("connect to clickhouse: %w", err)
	}

	stores := &allStores{
		// PostgreSQL stores (source data + trade_records)
		candidateStore:      pgstore.NewCandidateStore(pool),
		swapStore:           pgstore.NewSwapStore(pool),
		swapEventStore:      pgstore.NewSwapEventStore(pool),
		liquidityEventStore: pgstore.NewLiquidityEventStore(pool),
		tradeRecordStore:    pgstore.NewTradeRecordStore(pool),
		metadataStore:       pgstore.NewTokenMetadataStore(pool),

		// ClickHouse stores (analytics)
		priceTimeseriesStore:     chstore.NewPriceTimeseriesStore(chConn),
		liquidityTimeseriesStore: chstore.NewLiquidityTimeseriesStore(chConn),
		volumeTimeseriesStore:    chstore.NewVolumeTimeseriesStore(chConn),
		derivedFeatureStore:      chstore.NewDerivedFeatureStore(chConn),
		strategyAggregateStore:   chstore.NewStrategyAggregateStore(chConn),
	}

	cleanup := func() {
		chConn.Close()
		pool.Close()
	}

	return stores, cleanup, nil
}

// Run starts the unified server with all components.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Println("Starting unified server...")

	// Create error channel for goroutines
	errCh := make(chan error, 3)

	// Start ingestion in background
	go func() {
		err := s.runIngestion(ctx)
		if err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("ingestion: %w", err)
		}
	}()

	// Start pipeline scheduler in background
	go func() {
		err := s.runPipelineScheduler(ctx)
		if err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("pipeline scheduler: %w", err)
		}
	}()

	// Start report scheduler in background
	go func() {
		err := s.runReportScheduler(ctx)
		if err != nil && err != context.Canceled {
			errCh <- fmt.Errorf("report scheduler: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// runIngestion runs continuous data ingestion.
func (s *Server) runIngestion(ctx context.Context) error {
	s.logger.Println("Starting ingestion...")

	// Create RPC client
	rpc := solana.NewHTTPClient(s.rpcEndpoint)

	// Create SEPARATE WebSocket clients for swap and liquidity
	// This is required because Helius deduplicates subscriptions to the same program
	// on the same connection, returning the same subscription ID which causes
	// the second subscriber to overwrite the first one's channel
	wsSwap, err := solana.NewWSClient(ctx, s.wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("create websocket client for swaps: %w", err)
	}
	defer wsSwap.Close()

	wsLiquidity, err := solana.NewWSClient(ctx, s.wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("create websocket client for liquidity: %w", err)
	}
	defer wsLiquidity.Close()

	// Create sources with separate WebSocket clients
	wsSwapSource := ingestion.NewWSSwapEventSource(wsSwap, rpc, s.programs)
	wsLiquiditySource := ingestion.NewWSLiquidityEventSourceWithStore(wsLiquidity, rpc, s.programs, s.stores.candidateStore)
	metadataSource := ingestion.NewRPCMetadataSource(rpc)

	// Create detectors
	newTokenDetector := discovery.NewDetector(s.stores.candidateStore)
	activeDetector := discovery.NewActiveDetector(discovery.DefaultActiveConfig(), s.stores.swapEventStore, s.stores.candidateStore)

	// Create runner
	runner := ingestion.NewRunner(ingestion.RunnerOptions{
		WSSwapSource:      wsSwapSource,
		WSLiquiditySource: wsLiquiditySource,
		MetadataSource:    metadataSource,
		SwapEventStore:    s.stores.swapEventStore,
		LiquidityStore:    s.stores.liquidityEventStore,
		MetadataStore:     s.stores.metadataStore,
		CandidateStore:    s.stores.candidateStore,
		NewTokenDetector:  newTokenDetector,
		ActiveDetector:    activeDetector,
		CheckInterval:     s.checkInterval,
		Logger:            log.New(os.Stdout, "[ingestion] ", log.LstdFlags|log.Lshortfile),
	})

	s.mu.Lock()
	s.ingestionRunner = runner
	s.ingestionStarted = time.Now()
	s.mu.Unlock()

	s.logger.Println("Ingestion started")
	return runner.Run(ctx)
}

// runPipelineScheduler runs pipeline on schedule.
func (s *Server) runPipelineScheduler(ctx context.Context) error {
	s.logger.Printf("Starting pipeline scheduler (interval: %v)...", s.pipelineInterval)

	// Run immediately on start
	s.runPipeline(ctx)

	ticker := time.NewTicker(s.pipelineInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.runPipeline(ctx)
		}
	}
}

// runPipeline executes the processing pipeline.
func (s *Server) runPipeline(ctx context.Context) {
	s.mu.Lock()
	if s.pipelineRunning {
		s.mu.Unlock()
		s.logger.Println("Pipeline already running, skipping...")
		return
	}
	s.pipelineRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.pipelineRunning = false
		s.lastPipelineRun = time.Now()
		s.pipelineRuns++
		s.mu.Unlock()
	}()

	s.logger.Println("Running pipeline...")
	start := time.Now()

	// Create orchestrator
	orch := orchestrator.New(orchestrator.Options{
		CandidateStore:           s.stores.candidateStore,
		SwapStore:                s.stores.swapStore,
		LiquidityEventStore:      s.stores.liquidityEventStore,
		PriceTimeseriesStore:     s.stores.priceTimeseriesStore,
		LiquidityTimeseriesStore: s.stores.liquidityTimeseriesStore,
		VolumeTimeseriesStore:    s.stores.volumeTimeseriesStore,
		DerivedFeatureStore:      s.stores.derivedFeatureStore,
		TradeRecordStore:         s.stores.tradeRecordStore,
		StrategyAggregateStore:   s.stores.strategyAggregateStore,
		StrategyConfigs:          createStrategyConfigs(),
		ScenarioConfigs:          createScenarioConfigs(),
		Verbose:                  true,
	})

	result, err := orch.Run(ctx)
	if err != nil {
		s.logger.Printf("Pipeline error: %v", err)
		observability.RecordPipelineRun("orchestrator", "error", time.Since(start).Seconds())
		return
	}

	s.logger.Printf("Pipeline completed in %v: %d candidates, %d trades, %d aggregates",
		time.Since(start), result.CandidatesProcessed, result.TradesCreated, result.AggregatesCreated)

	observability.RecordPipelineRun("orchestrator", "success", time.Since(start).Seconds())
}

// runReportScheduler runs report generation on schedule.
func (s *Server) runReportScheduler(ctx context.Context) error {
	s.logger.Printf("Starting report scheduler (interval: %v)...", s.reportInterval)

	// Wait for first pipeline run before generating reports
	time.Sleep(s.pipelineInterval + 1*time.Minute)

	// Run immediately after first pipeline
	s.runReport(ctx)

	ticker := time.NewTicker(s.reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.runReport(ctx)
		}
	}
}

// runReport generates reports.
func (s *Server) runReport(ctx context.Context) {
	s.mu.Lock()
	if s.reportRunning {
		s.mu.Unlock()
		s.logger.Println("Report generation already running, skipping...")
		return
	}
	// Wait for pipeline to finish
	if s.pipelineRunning {
		s.mu.Unlock()
		s.logger.Println("Pipeline running, waiting before report generation...")
		time.Sleep(30 * time.Second)
		s.mu.Lock()
	}
	s.reportRunning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.reportRunning = false
		s.lastReportRun = time.Now()
		s.reportRuns++
		s.mu.Unlock()
	}()

	s.logger.Println("Generating reports...")
	start := time.Now()

	// Ensure output directory exists
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		s.logger.Printf("Failed to create output directory: %v", err)
		return
	}

	// Create aggregator
	aggregator := metrics.NewAggregator(
		s.stores.tradeRecordStore,
		s.stores.strategyAggregateStore,
		s.stores.candidateStore,
	)

	// Define implementable strategies
	implementable := map[decision.StrategyKey]bool{
		{StrategyID: "TIME_EXIT", EntryEventType: "NEW_TOKEN"}:          true,
		{StrategyID: "TIME_EXIT", EntryEventType: "ACTIVE_TOKEN"}:       true,
		{StrategyID: "TRAILING_STOP", EntryEventType: "NEW_TOKEN"}:      true,
		{StrategyID: "TRAILING_STOP", EntryEventType: "ACTIVE_TOKEN"}:   true,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "NEW_TOKEN"}:    true,
		{StrategyID: "LIQUIDITY_GUARD", EntryEventType: "ACTIVE_TOKEN"}: true,
	}

	// Create replay runner
	replayRunner := replay.NewRunner(s.stores.swapStore, s.stores.liquidityEventStore)

	// Create pipeline
	p := pipeline.NewPhase1Pipeline(
		s.stores.candidateStore,
		s.stores.tradeRecordStore,
		s.stores.strategyAggregateStore,
		implementable,
		s.outputDir,
	).WithSufficiencyChecker(
		s.stores.candidateStore,
		s.stores.tradeRecordStore,
		s.stores.swapStore,
		s.stores.liquidityEventStore,
		replayRunner,
	).WithAggregator(aggregator)

	// Set data source based on mode
	if s.useMemory {
		p = p.WithDataSource("unified-server")
	} else {
		p = p.WithDBSource(s.postgresDSN, s.clickhouseDSN).
			WithRawDataStores(s.stores.candidateStore, s.stores.priceTimeseriesStore, s.stores.liquidityTimeseriesStore)
	}

	// Run reporting pipeline
	if err := p.Run(ctx); err != nil {
		s.logger.Printf("Report generation error: %v", err)
		return
	}

	s.logger.Printf("Reports generated in %v to %s/", time.Since(start), s.outputDir)
}

// startHTTPServer starts the HTTP server for health/metrics/status.
func (s *Server) startHTTPServer(addr string) {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Prometheus metrics
	mux.Handle("/metrics", observability.Handler())

	// Status endpoint
	mux.HandleFunc("/status", s.handleStatus)

	s.logger.Printf("Starting HTTP server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
		s.logger.Printf("HTTP server error: %v", err)
	}
}

// StatusResponse is the JSON response for /status endpoint.
type StatusResponse struct {
	Status           string    `json:"status"`
	Uptime           string    `json:"uptime"`
	IngestionStarted time.Time `json:"ingestion_started"`
	LastPipelineRun  time.Time `json:"last_pipeline_run,omitempty"`
	LastReportRun    time.Time `json:"last_report_run,omitempty"`
	PipelineRuns     int       `json:"pipeline_runs"`
	ReportRuns       int       `json:"report_runs"`
	PipelineRunning  bool      `json:"pipeline_running"`
	ReportRunning    bool      `json:"report_running"`
}

// handleStatus returns server status as JSON.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp := StatusResponse{
		Status:           "running",
		Uptime:           time.Since(s.ingestionStarted).String(),
		IngestionStarted: s.ingestionStarted,
		LastPipelineRun:  s.lastPipelineRun,
		LastReportRun:    s.lastReportRun,
		PipelineRuns:     s.pipelineRuns,
		ReportRuns:       s.reportRuns,
		PipelineRunning:  s.pipelineRunning,
		ReportRunning:    s.reportRunning,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
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

// loadEnvFile loads environment variables from .env file if it exists.
func loadEnvFile() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return // File doesn't exist, use system env vars
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
