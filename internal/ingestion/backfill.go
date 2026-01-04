package ingestion

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
)

// Backfiller handles historical data ingestion from RPC.
type Backfiller struct {
	rpc              *solana.HTTPClient
	swapSource       *RPCSwapEventSource
	liquiditySource  *RPCLiquidityEventSource
	swapEventStore   storage.SwapEventStore
	liquidityStore   storage.LiquidityEventStore
	candidateStore   storage.CandidateStore
	newTokenDetector *discovery.NewTokenDetector
	batchSize        int
	logger           *log.Logger
}

// BackfillOptions contains configuration for creating a Backfiller.
type BackfillOptions struct {
	RPC              *solana.HTTPClient
	SwapSource       *RPCSwapEventSource
	LiquiditySource  *RPCLiquidityEventSource
	SwapEventStore   storage.SwapEventStore
	LiquidityStore   storage.LiquidityEventStore
	CandidateStore   storage.CandidateStore
	NewTokenDetector *discovery.NewTokenDetector
	BatchSize        int
	Logger           *log.Logger
}

// NewBackfiller creates a new historical data backfiller.
func NewBackfiller(opts BackfillOptions) *Backfiller {
	batchSize := opts.BatchSize
	if batchSize == 0 {
		batchSize = 1000
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}

	return &Backfiller{
		rpc:              opts.RPC,
		swapSource:       opts.SwapSource,
		liquiditySource:  opts.LiquiditySource,
		swapEventStore:   opts.SwapEventStore,
		liquidityStore:   opts.LiquidityStore,
		candidateStore:   opts.CandidateStore,
		newTokenDetector: opts.NewTokenDetector,
		batchSize:        batchSize,
		logger:           logger,
	}
}

// BackfillResult contains statistics from a backfill operation.
type BackfillResult struct {
	SwapEventsIngested       int
	LiquidityEventsIngested  int
	CandidatesDiscovered     int
	DuplicatesSkipped        int
	Errors                   int
	Duration                 time.Duration
}

// BackfillSince backfills data from a given timestamp until now.
func (b *Backfiller) BackfillSince(ctx context.Context, since time.Time) (*BackfillResult, error) {
	to := time.Now()
	return b.BackfillRange(ctx, since, to)
}

// BackfillRange backfills data for a specific time range.
func (b *Backfiller) BackfillRange(ctx context.Context, from, to time.Time) (*BackfillResult, error) {
	start := time.Now()
	result := &BackfillResult{}

	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()

	b.logger.Printf("Starting backfill from %s to %s", from.Format(time.RFC3339), to.Format(time.RFC3339))

	// Fetch swap events
	if b.swapSource != nil {
		swapEvents, err := b.swapSource.Fetch(ctx, fromMs, toMs)
		if err != nil {
			return result, fmt.Errorf("fetch swap events: %w", err)
		}

		b.logger.Printf("Fetched %d swap events", len(swapEvents))

		// Store swap events in batches
		stored, dupes, errs := b.storeSwapEvents(ctx, swapEvents)
		result.SwapEventsIngested += stored
		result.DuplicatesSkipped += dupes
		result.Errors += errs

		// Run NEW_TOKEN detection on all events
		if b.newTokenDetector != nil {
			discovered := b.runDiscovery(ctx, swapEvents)
			result.CandidatesDiscovered += discovered
		}
	}

	// Fetch liquidity events (if configured)
	if b.liquiditySource != nil && b.liquidityStore != nil {
		// For backfill, fetch without candidateID filter - get all events
		liqEvents, err := b.liquiditySource.Fetch(ctx, "", fromMs, toMs)
		if err != nil {
			b.logger.Printf("Error fetching liquidity events: %v", err)
		} else {
			b.logger.Printf("Fetched %d liquidity events", len(liqEvents))

			// Store liquidity events in batches
			stored, dupes, errs := b.storeLiquidityEvents(ctx, liqEvents)
			result.LiquidityEventsIngested += stored
			result.DuplicatesSkipped += dupes
			result.Errors += errs
		}
	}

	result.Duration = time.Since(start)
	b.logger.Printf("Backfill complete: %d swaps, %d liquidity, %d candidates, %d dupes, %d errors in %v",
		result.SwapEventsIngested, result.LiquidityEventsIngested, result.CandidatesDiscovered,
		result.DuplicatesSkipped, result.Errors, result.Duration)

	return result, nil
}

// BackfillSlotRange backfills data for a specific slot range.
func (b *Backfiller) BackfillSlotRange(ctx context.Context, fromSlot, toSlot int64) (*BackfillResult, error) {
	// Get block times for slot range
	fromTime, err := b.rpc.GetBlockTime(ctx, fromSlot)
	if err != nil {
		return nil, fmt.Errorf("get block time for slot %d: %w", fromSlot, err)
	}

	toTime, err := b.rpc.GetBlockTime(ctx, toSlot)
	if err != nil {
		return nil, fmt.Errorf("get block time for slot %d: %w", toSlot, err)
	}

	if fromTime == nil || toTime == nil {
		return nil, fmt.Errorf("could not get block times for slot range")
	}

	from := time.Unix(*fromTime, 0)
	to := time.Unix(*toTime, 0)

	return b.BackfillRange(ctx, from, to)
}

// storeSwapEvents stores swap events in batches, handling duplicates.
func (b *Backfiller) storeSwapEvents(ctx context.Context, events []*domain.SwapEvent) (stored, dupes, errs int) {
	if b.swapEventStore == nil {
		return 0, 0, 0
	}

	for i := 0; i < len(events); i += b.batchSize {
		end := i + b.batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		err := b.swapEventStore.InsertBulk(ctx, batch)
		if err != nil {
			if errors.Is(err, storage.ErrDuplicateKey) {
				// Insert one by one to find which are duplicates
				for _, event := range batch {
					if err := b.swapEventStore.Insert(ctx, event); err != nil {
						if errors.Is(err, storage.ErrDuplicateKey) {
							dupes++
						} else {
							errs++
						}
					} else {
						stored++
					}
				}
			} else {
				errs += len(batch)
				b.logger.Printf("Error storing batch: %v", err)
			}
		} else {
			stored += len(batch)
		}
	}

	return stored, dupes, errs
}

// storeLiquidityEvents stores liquidity events in batches, handling duplicates.
func (b *Backfiller) storeLiquidityEvents(ctx context.Context, events []*domain.LiquidityEvent) (stored, dupes, errs int) {
	if b.liquidityStore == nil {
		return 0, 0, 0
	}

	for i := 0; i < len(events); i += b.batchSize {
		end := i + b.batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		err := b.liquidityStore.InsertBulk(ctx, batch)
		if err != nil {
			if errors.Is(err, storage.ErrDuplicateKey) {
				// Insert one by one to find which are duplicates
				for _, event := range batch {
					if err := b.liquidityStore.Insert(ctx, event); err != nil {
						if errors.Is(err, storage.ErrDuplicateKey) {
							dupes++
						} else {
							errs++
						}
					} else {
						stored++
					}
				}
			} else {
				errs += len(batch)
				b.logger.Printf("Error storing liquidity batch: %v", err)
			}
		} else {
			stored += len(batch)
		}
	}

	return stored, dupes, errs
}

// runDiscovery runs NEW_TOKEN detection on swap events.
func (b *Backfiller) runDiscovery(ctx context.Context, events []*domain.SwapEvent) int {
	if b.newTokenDetector == nil {
		return 0
	}

	discovered := 0

	// Convert domain.SwapEvent to discovery.SwapEvent
	discoveryEvents := make([]*discovery.SwapEvent, len(events))
	for i, e := range events {
		discoveryEvents[i] = &discovery.SwapEvent{
			Mint:        e.Mint,
			Pool:        e.Pool,
			TxSignature: e.TxSignature,
			EventIndex:  e.EventIndex,
			Slot:        e.Slot,
			Timestamp:   e.Timestamp,
		}
	}

	// Process events through detector
	candidates, err := b.newTokenDetector.ProcessEvents(ctx, discoveryEvents)
	if err != nil {
		b.logger.Printf("Error in discovery: %v", err)
		return discovered
	}

	discovered = len(candidates)
	for _, c := range candidates {
		b.logger.Printf("Discovered candidate: %s (mint=%s)", c.CandidateID, c.Mint)
	}

	return discovered
}

// ResumeBackfill resumes a backfill from the last ingested event.
func (b *Backfiller) ResumeBackfill(ctx context.Context) (*BackfillResult, error) {
	// Find the latest ingested event timestamp
	// For now, just start from 24h ago
	since := time.Now().Add(-24 * time.Hour)
	return b.BackfillSince(ctx, since)
}
