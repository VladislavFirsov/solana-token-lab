package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/ingestion"
	"solana-token-lab/internal/observability"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
	pgstore "solana-token-lab/internal/storage/postgres"
)

// DEX program aliases mapped to program IDs.
var dexAliases = map[string]string{
	"raydium": discovery.RaydiumAMMV4,
	"pumpfun": discovery.PumpFun,
	// Add more as needed
}

func main() {
	// Parse flags
	mode := flag.String("mode", "live", "Ingestion mode: live, backfill, or replay")
	rpcEndpoint := flag.String("rpc-endpoint", "", "Solana RPC HTTP endpoint")
	wsEndpoint := flag.String("ws-endpoint", "", "Solana WebSocket endpoint")
	postgresDSN := flag.String("postgres-dsn", "", "PostgreSQL connection string")
	fromSlot := flag.Int64("from-slot", 0, "Start slot for backfill")
	toSlot := flag.Int64("to-slot", 0, "End slot for backfill")
	fromTime := flag.String("from-time", "", "Start time for backfill (RFC3339)")
	toTime := flag.String("to-time", "", "End time for backfill (RFC3339)")
	programs := flag.String("programs", "", "Comma-separated DEX program IDs to monitor")
	dex := flag.String("dex", "raydium,pumpfun", "Comma-separated DEX aliases (raydium, pumpfun)")
	checkInterval := flag.Duration("check-interval", 1*time.Hour, "ACTIVE_TOKEN detection interval")
	useMemory := flag.Bool("use-memory", false, "Use in-memory storage instead of PostgreSQL")
	metricsAddr := flag.String("metrics-addr", ":9090", "Prometheus metrics HTTP address (empty to disable)")

	flag.Parse()

	// Setup logger
	logger := log.New(os.Stdout, "[ingest] ", log.LstdFlags|log.Lshortfile)

	// Start metrics server if enabled
	if *metricsAddr != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/metrics", observability.Handler())
			mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			})
			logger.Printf("Starting metrics server on %s", *metricsAddr)
			if err := http.ListenAndServe(*metricsAddr, mux); err != nil && err != http.ErrServerClosed {
				logger.Printf("Metrics server error: %v", err)
			}
		}()
	}

	// Resolve DEX programs
	programList := resolvePrograms(*programs, *dex)
	if len(programList) == 0 {
		logger.Fatal("No DEX programs specified. Use --programs or --dex")
	}
	logger.Printf("Monitoring DEX programs: %v", programList)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Handle shutdown signals with graceful timeout
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal main goroutine completion
	done := make(chan error, 1)

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

	// Run based on mode
	var err error
	switch *mode {
	case "live":
		err = runLive(ctx, logger, *rpcEndpoint, *wsEndpoint, *postgresDSN, programList, *checkInterval, *useMemory)
	case "backfill":
		err = runBackfill(ctx, logger, *rpcEndpoint, *postgresDSN, programList, *fromSlot, *toSlot, *fromTime, *toTime, *useMemory)
	case "replay":
		err = runReplay(ctx, logger, *postgresDSN, *fromTime, *toTime, *useMemory)
	default:
		logger.Fatalf("Unknown mode: %s", *mode)
	}

	// Signal completion to shutdown handler
	done <- err
	cancel()

	if err != nil && err != context.Canceled {
		logger.Fatalf("Error: %v", err)
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

// runLive runs continuous live ingestion.
func runLive(ctx context.Context, logger *log.Logger, rpcEndpoint, wsEndpoint, postgresDSN string, programs []string, checkInterval time.Duration, useMemory bool) error {
	if rpcEndpoint == "" {
		return fmt.Errorf("--rpc-endpoint is required for live mode")
	}
	if wsEndpoint == "" {
		return fmt.Errorf("--ws-endpoint is required for live mode")
	}

	// Create RPC client
	rpc := solana.NewHTTPClient(rpcEndpoint)

	// Create SEPARATE WebSocket clients for swap and liquidity
	// This is required because Helius deduplicates subscriptions to the same program
	// on the same connection, returning the same subscription ID which causes
	// the second subscriber to overwrite the first one's channel
	wsSwap, err := solana.NewWSClient(ctx, wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("create websocket client for swaps: %w", err)
	}
	defer wsSwap.Close()

	wsLiquidity, err := solana.NewWSClient(ctx, wsEndpoint, nil)
	if err != nil {
		return fmt.Errorf("create websocket client for liquidity: %w", err)
	}
	defer wsLiquidity.Close()

	// Require --postgres-dsn unless --use-memory is explicitly set
	if !useMemory && postgresDSN == "" {
		return fmt.Errorf("--postgres-dsn is required for live mode (use --use-memory for in-memory storage)")
	}

	// Create stores (use interfaces)
	var swapEventStore storage.SwapEventStore = memory.NewSwapEventStore()
	var liquidityStore storage.LiquidityEventStore = memory.NewLiquidityEventStore()
	var candidateStore storage.CandidateStore = memory.NewCandidateStore()
	var metadataStore storage.TokenMetadataStore = memory.NewTokenMetadataStore()

	if !useMemory {
		pool, err := pgstore.NewPool(ctx, postgresDSN)
		if err != nil {
			return fmt.Errorf("connect to postgres: %w", err)
		}
		defer pool.Close()

		swapEventStore = pgstore.NewSwapEventStore(pool)
		liquidityStore = pgstore.NewLiquidityEventStore(pool)
		candidateStore = pgstore.NewCandidateStore(pool)
		metadataStore = pgstore.NewTokenMetadataStore(pool)
	}

	// Create sources with separate WebSocket clients
	wsSwapSource := ingestion.NewWSSwapEventSource(wsSwap, rpc, programs)
	wsLiquiditySource := ingestion.NewWSLiquidityEventSourceWithStore(wsLiquidity, rpc, programs, candidateStore)
	metadataSource := ingestion.NewRPCMetadataSource(rpc)

	// Create detectors
	newTokenDetector := discovery.NewDetector(candidateStore)
	activeDetector := discovery.NewActiveDetector(discovery.DefaultActiveConfig(), swapEventStore, candidateStore)

	// Create and run runner
	runner := ingestion.NewRunner(ingestion.RunnerOptions{
		WSSwapSource:      wsSwapSource,
		WSLiquiditySource: wsLiquiditySource,
		MetadataSource:    metadataSource,
		SwapEventStore:    swapEventStore,
		LiquidityStore:    liquidityStore,
		MetadataStore:     metadataStore,
		CandidateStore:    candidateStore,
		NewTokenDetector:  newTokenDetector,
		ActiveDetector:    activeDetector,
		CheckInterval:     checkInterval,
		Logger:            logger,
	})

	logger.Println("Starting live ingestion...")
	return runner.Run(ctx)
}

// runBackfill runs historical data backfill.
func runBackfill(ctx context.Context, logger *log.Logger, rpcEndpoint, postgresDSN string, programs []string, fromSlot, toSlot int64, fromTimeStr, toTimeStr string, useMemory bool) error {
	if rpcEndpoint == "" {
		return fmt.Errorf("--rpc-endpoint is required for backfill mode")
	}

	// Create RPC client
	rpc := solana.NewHTTPClient(rpcEndpoint)

	// Require --postgres-dsn unless --use-memory is explicitly set
	if !useMemory && postgresDSN == "" {
		return fmt.Errorf("--postgres-dsn is required for backfill mode (use --use-memory for in-memory storage)")
	}

	// Create stores (use interfaces)
	var swapEventStore storage.SwapEventStore = memory.NewSwapEventStore()
	var liquidityStore storage.LiquidityEventStore = memory.NewLiquidityEventStore()
	var candidateStore storage.CandidateStore = memory.NewCandidateStore()

	if !useMemory {
		pool, err := pgstore.NewPool(ctx, postgresDSN)
		if err != nil {
			return fmt.Errorf("connect to postgres: %w", err)
		}
		defer pool.Close()

		swapEventStore = pgstore.NewSwapEventStore(pool)
		liquidityStore = pgstore.NewLiquidityEventStore(pool)
		candidateStore = pgstore.NewCandidateStore(pool)
	}

	// Create sources
	swapSource := ingestion.NewRPCSwapEventSource(rpc, programs)
	liquiditySource := ingestion.NewRPCLiquidityEventSource(rpc, programs, candidateStore)

	// Create detector
	newTokenDetector := discovery.NewDetector(candidateStore)

	// Create backfiller
	backfiller := ingestion.NewBackfiller(ingestion.BackfillOptions{
		RPC:              rpc,
		SwapSource:       swapSource,
		LiquiditySource:  liquiditySource,
		SwapEventStore:   swapEventStore,
		LiquidityStore:   liquidityStore,
		CandidateStore:   candidateStore,
		NewTokenDetector: newTokenDetector,
		Logger:           logger,
	})

	// Determine time range
	var from, to time.Time
	var err error

	if fromSlot > 0 && toSlot > 0 {
		// Use slot range
		logger.Printf("Backfilling slot range: %d to %d", fromSlot, toSlot)
		_, err = backfiller.BackfillSlotRange(ctx, fromSlot, toSlot)
	} else if fromTimeStr != "" {
		// Use time range
		from, err = time.Parse(time.RFC3339, fromTimeStr)
		if err != nil {
			return fmt.Errorf("parse from-time: %w", err)
		}

		if toTimeStr != "" {
			to, err = time.Parse(time.RFC3339, toTimeStr)
			if err != nil {
				return fmt.Errorf("parse to-time: %w", err)
			}
		} else {
			to = time.Now()
		}

		logger.Printf("Backfilling time range: %s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339))
		_, err = backfiller.BackfillRange(ctx, from, to)
	} else {
		// Default: last 24 hours
		logger.Println("No time range specified, backfilling last 24 hours")
		_, err = backfiller.BackfillSince(ctx, time.Now().Add(-24*time.Hour))
	}

	return err
}

// runReplay runs discovery replay from stored events.
func runReplay(ctx context.Context, logger *log.Logger, postgresDSN, fromTimeStr, toTimeStr string, useMemory bool) error {
	// Require --postgres-dsn unless --use-memory is explicitly set
	if !useMemory && postgresDSN == "" {
		return fmt.Errorf("--postgres-dsn is required for replay mode (use --use-memory for in-memory storage)")
	}

	// Create stores (use interfaces)
	var swapEventStore storage.SwapEventStore = memory.NewSwapEventStore()
	var candidateStore storage.CandidateStore = memory.NewCandidateStore()

	if !useMemory {
		pool, err := pgstore.NewPool(ctx, postgresDSN)
		if err != nil {
			return fmt.Errorf("connect to postgres: %w", err)
		}
		defer pool.Close()

		swapEventStore = pgstore.NewSwapEventStore(pool)
		candidateStore = pgstore.NewCandidateStore(pool)
	}

	// Create detector
	newTokenDetector := discovery.NewDetector(candidateStore)
	activeDetector := discovery.NewActiveDetector(discovery.DefaultActiveConfig(), swapEventStore, candidateStore)

	// Create replayer
	replayer := ingestion.NewReplayer(ingestion.ReplayerOptions{
		SwapEventStore:   swapEventStore,
		CandidateStore:   candidateStore,
		NewTokenDetector: newTokenDetector,
		ActiveDetector:   activeDetector,
		Logger:           logger,
	})

	// Determine time range
	var from, to int64

	if fromTimeStr != "" {
		t, err := time.Parse(time.RFC3339, fromTimeStr)
		if err != nil {
			return fmt.Errorf("parse from-time: %w", err)
		}
		from = t.UnixMilli()
	} else {
		from = time.Now().Add(-24 * time.Hour).UnixMilli()
	}

	if toTimeStr != "" {
		t, err := time.Parse(time.RFC3339, toTimeStr)
		if err != nil {
			return fmt.Errorf("parse to-time: %w", err)
		}
		to = t.UnixMilli()
	} else {
		to = time.Now().UnixMilli()
	}

	logger.Printf("Replaying discovery from %d to %d", from, to)

	result, err := replayer.ReplayFull(ctx, from, to)
	if err != nil {
		return err
	}

	logger.Printf("Replay complete: %d events, %d NEW_TOKEN, %d ACTIVE_TOKEN in %v",
		result.EventsProcessed, result.NewTokensDiscovered,
		result.ActiveTokensDiscovered, result.Duration)

	return nil
}
