package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"solana-token-lab/internal/replay"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/storage/memory"
	pgstore "solana-token-lab/internal/storage/postgres"
)

func main() {
	// Parse flags
	candidateID := flag.String("candidate-id", "", "Candidate ID to replay (required)")
	fromTime := flag.String("from-time", "", "Start time (RFC3339)")
	toTime := flag.String("to-time", "", "End time (RFC3339)")
	postgresDSN := flag.String("postgres-dsn", "", "PostgreSQL connection string")
	useMemory := flag.Bool("use-memory", false, "Use in-memory storage")
	outputJSON := flag.Bool("json", false, "Output as JSON")

	flag.Parse()

	// Setup structured logger
	logger := log.New(os.Stderr, "[replay] ", log.LstdFlags)

	// Validate required flags
	if *candidateID == "" {
		logger.Fatal("--candidate-id is required")
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
	var swapStore storage.SwapStore = memory.NewSwapStore()
	var liquidityStore storage.LiquidityEventStore = memory.NewLiquidityEventStore()

	if !*useMemory && *postgresDSN != "" {
		pool, err := pgstore.NewPool(ctx, *postgresDSN)
		if err != nil {
			logger.Fatalf("connect to postgres: %v", err)
		}
		defer pool.Close()

		swapStore = pgstore.NewSwapStore(pool)
		liquidityStore = pgstore.NewLiquidityEventStore(pool)
	}

	// Create replay runner
	replayRunner := replay.NewRunner(swapStore, liquidityStore)

	// Determine time range
	var from, to int64
	if *fromTime != "" {
		t, err := time.Parse(time.RFC3339, *fromTime)
		if err != nil {
			logger.Fatalf("parse from-time: %v", err)
		}
		from = t.UnixMilli()
	}
	if *toTime != "" {
		t, err := time.Parse(time.RFC3339, *toTime)
		if err != nil {
			logger.Fatalf("parse to-time: %v", err)
		}
		to = t.UnixMilli()
	}

	// Create logging engine
	engine := NewLoggingEngine(*candidateID, *outputJSON)

	// Run replay - deterministic behavior only
	// Either use explicit time range (both bounds required) or replay all events
	var err error
	if from > 0 && to > 0 {
		// Explicit time range - both bounds required for determinism
		logger.Printf("Replaying candidate %s from %d to %d", *candidateID, from, to)
		err = replayRunner.Run(ctx, *candidateID, from, to, engine)
	} else if from > 0 || to > 0 {
		// Partial range is non-deterministic - reject
		logger.Fatal("Both --from-time and --to-time must be specified together for deterministic replay")
	} else {
		// No time range - replay all stored events (deterministic)
		logger.Printf("Replaying all events for candidate %s", *candidateID)
		err = replayRunner.RunAll(ctx, *candidateID, engine)
	}

	if err != nil {
		logger.Fatalf("replay failed: %v", err)
	}

	// Output summary
	stats := engine.Stats()
	if *outputJSON {
		output, _ := json.MarshalIndent(stats, "", "  ")
		fmt.Println(string(output))
	} else {
		fmt.Printf("\n=== Replay Summary ===\n")
		fmt.Printf("Candidate ID:      %s\n", stats.CandidateID)
		fmt.Printf("Total Events:      %d\n", stats.TotalEvents)
		fmt.Printf("Swap Events:       %d\n", stats.SwapEvents)
		fmt.Printf("Liquidity Events:  %d\n", stats.LiquidityEvents)
		if stats.TotalEvents > 0 {
			fmt.Printf("First Event Time:  %s\n", time.UnixMilli(stats.FirstEventTime).Format(time.RFC3339))
			fmt.Printf("Last Event Time:   %s\n", time.UnixMilli(stats.LastEventTime).Format(time.RFC3339))
			fmt.Printf("Duration:          %v\n", time.Duration(stats.LastEventTime-stats.FirstEventTime)*time.Millisecond)
		} else {
			fmt.Printf("First Event Time:  N/A\n")
			fmt.Printf("Last Event Time:   N/A\n")
			fmt.Printf("Duration:          N/A\n")
		}
	}
}

// LoggingEngine implements replay.ReplayEngine and logs events.
type LoggingEngine struct {
	candidateID string
	outputJSON  bool
	stats       ReplayStats
}

// ReplayStats holds replay statistics.
type ReplayStats struct {
	CandidateID     string `json:"candidate_id"`
	TotalEvents     int    `json:"total_events"`
	SwapEvents      int    `json:"swap_events"`
	LiquidityEvents int    `json:"liquidity_events"`
	FirstEventTime  int64  `json:"first_event_time"`
	LastEventTime   int64  `json:"last_event_time"`
}

// NewLoggingEngine creates a new logging engine.
func NewLoggingEngine(candidateID string, outputJSON bool) *LoggingEngine {
	return &LoggingEngine{
		candidateID: candidateID,
		outputJSON:  outputJSON,
		stats: ReplayStats{
			CandidateID: candidateID,
		},
	}
}

// OnEvent processes an event.
func (e *LoggingEngine) OnEvent(ctx context.Context, event *replay.Event) error {
	e.stats.TotalEvents++

	// Update time bounds
	if e.stats.FirstEventTime == 0 || event.Timestamp < e.stats.FirstEventTime {
		e.stats.FirstEventTime = event.Timestamp
	}
	if event.Timestamp > e.stats.LastEventTime {
		e.stats.LastEventTime = event.Timestamp
	}

	// Count by type
	switch event.Type {
	case replay.EventTypeSwap:
		e.stats.SwapEvents++
	case replay.EventTypeLiquidity:
		e.stats.LiquidityEvents++
	}

	// Log event if not in JSON mode
	if !e.outputJSON {
		fmt.Printf("[%s] slot=%d type=%s\n",
			time.UnixMilli(event.Timestamp).Format(time.RFC3339Nano),
			event.Slot,
			event.Type,
		)
	}

	return nil
}

// Stats returns replay statistics.
func (e *LoggingEngine) Stats() ReplayStats {
	return e.stats
}

// Ensure LoggingEngine implements replay.ReplayEngine
var _ replay.ReplayEngine = (*LoggingEngine)(nil)
