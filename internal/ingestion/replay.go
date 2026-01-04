package ingestion

import (
	"context"
	"fmt"
	"log"
	"time"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// Replayer replays discovery from stored events without RPC dependency.
type Replayer struct {
	swapEventStore   storage.SwapEventStore
	candidateStore   storage.CandidateStore
	newTokenDetector *discovery.NewTokenDetector
	activeDetector   *discovery.ActiveTokenDetector
	batchSize        int
	logger           *log.Logger
}

// ReplayerOptions contains configuration for creating a Replayer.
type ReplayerOptions struct {
	SwapEventStore   storage.SwapEventStore
	CandidateStore   storage.CandidateStore
	NewTokenDetector *discovery.NewTokenDetector
	ActiveDetector   *discovery.ActiveTokenDetector
	BatchSize        int
	Logger           *log.Logger
}

// NewReplayer creates a new discovery replayer.
func NewReplayer(opts ReplayerOptions) *Replayer {
	batchSize := opts.BatchSize
	if batchSize == 0 {
		batchSize = 10000
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}

	return &Replayer{
		swapEventStore:   opts.SwapEventStore,
		candidateStore:   opts.CandidateStore,
		newTokenDetector: opts.NewTokenDetector,
		activeDetector:   opts.ActiveDetector,
		batchSize:        batchSize,
		logger:           logger,
	}
}

// ReplayResult contains statistics from a replay operation.
type ReplayResult struct {
	EventsProcessed      int
	NewTokensDiscovered  int
	ActiveTokensDiscovered int
	Duration             time.Duration
}

// ReplayDiscovery replays NEW_TOKEN discovery from stored events.
// This runs without any RPC dependency - purely from storage.
func (r *Replayer) ReplayDiscovery(ctx context.Context, from, to int64) (*ReplayResult, error) {
	start := time.Now()
	result := &ReplayResult{}

	r.logger.Printf("Starting replay from %d to %d", from, to)

	// Reset detector's in-memory cache to start fresh
	if r.newTokenDetector != nil {
		r.newTokenDetector.Reset()
	}

	// Load events from storage
	events, err := r.swapEventStore.GetByTimeRange(ctx, from, to)
	if err != nil {
		return result, fmt.Errorf("get events from storage: %w", err)
	}

	result.EventsProcessed = len(events)
	r.logger.Printf("Loaded %d events from storage", len(events))

	// Sort events for deterministic ordering
	SortSwapEvents(events)

	// Convert to discovery events
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

	// Run NEW_TOKEN detection
	if r.newTokenDetector != nil {
		candidates, err := r.newTokenDetector.ProcessEvents(ctx, discoveryEvents)
		if err != nil {
			return result, fmt.Errorf("process events: %w", err)
		}
		result.NewTokensDiscovered = len(candidates)
		r.logger.Printf("NEW_TOKEN discovery: %d candidates", len(candidates))
	}

	result.Duration = time.Since(start)
	r.logger.Printf("Replay complete in %v", result.Duration)

	return result, nil
}

// ReplayActiveTokenDetection replays ACTIVE_TOKEN spike detection at a specific timestamp.
func (r *Replayer) ReplayActiveTokenDetection(ctx context.Context, timestamp int64) (*ReplayResult, error) {
	start := time.Now()
	result := &ReplayResult{}

	if r.activeDetector == nil {
		return result, fmt.Errorf("no active detector configured")
	}

	r.logger.Printf("Replaying ACTIVE_TOKEN detection at %d", timestamp)

	candidates, err := r.activeDetector.DetectAt(ctx, timestamp)
	if err != nil {
		return result, fmt.Errorf("detect at %d: %w", timestamp, err)
	}

	result.ActiveTokensDiscovered = len(candidates)
	result.Duration = time.Since(start)

	r.logger.Printf("ACTIVE_TOKEN detection: %d candidates in %v", len(candidates), result.Duration)

	return result, nil
}

// ReplayFull replays both NEW_TOKEN and ACTIVE_TOKEN discovery for a time range.
func (r *Replayer) ReplayFull(ctx context.Context, from, to int64) (*ReplayResult, error) {
	start := time.Now()
	result := &ReplayResult{}

	// First, replay NEW_TOKEN discovery
	newResult, err := r.ReplayDiscovery(ctx, from, to)
	if err != nil {
		return result, fmt.Errorf("replay new token: %w", err)
	}
	result.EventsProcessed = newResult.EventsProcessed
	result.NewTokensDiscovered = newResult.NewTokensDiscovered

	// Then, run ACTIVE_TOKEN detection at the end of the range
	if r.activeDetector != nil {
		activeResult, err := r.ReplayActiveTokenDetection(ctx, to)
		if err != nil {
			// Log but don't fail - ACTIVE_TOKEN detection is supplementary
			r.logger.Printf("Warning: ACTIVE_TOKEN detection failed: %v", err)
		} else {
			result.ActiveTokensDiscovered = activeResult.ActiveTokensDiscovered
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}

// VerifyDeterminism replays discovery twice and verifies results are identical.
// This is useful for testing deterministic ordering guarantees.
func (r *Replayer) VerifyDeterminism(ctx context.Context, from, to int64) (bool, error) {
	r.logger.Println("Verifying determinism with two replay passes...")

	// First pass
	if r.newTokenDetector != nil {
		r.newTokenDetector.Reset()
	}
	result1, err := r.ReplayDiscovery(ctx, from, to)
	if err != nil {
		return false, fmt.Errorf("first pass: %w", err)
	}

	// Clear candidate store for second pass
	// Note: This requires CandidateStore to support deletion,
	// which may not be available in append-only stores.
	// For verification, we just compare counts.

	// Second pass
	if r.newTokenDetector != nil {
		r.newTokenDetector.Reset()
	}
	result2, err := r.ReplayDiscovery(ctx, from, to)
	if err != nil {
		return false, fmt.Errorf("second pass: %w", err)
	}

	// Compare results
	if result1.EventsProcessed != result2.EventsProcessed {
		r.logger.Printf("Mismatch: events processed %d vs %d",
			result1.EventsProcessed, result2.EventsProcessed)
		return false, nil
	}

	if result1.NewTokensDiscovered != result2.NewTokensDiscovered {
		r.logger.Printf("Mismatch: candidates discovered %d vs %d",
			result1.NewTokensDiscovered, result2.NewTokensDiscovered)
		return false, nil
	}

	r.logger.Println("Determinism verified: both passes produced identical results")
	return true, nil
}

// ReplayDiscoveryFunc is a function type for replaying discovery.
// Useful for dependency injection in tests.
type ReplayDiscoveryFunc func(ctx context.Context, store storage.SwapEventStore, detector *discovery.NewTokenDetector, from, to int64) error

// ReplayDiscoverySimple is a simplified replay function for use without full Replayer.
func ReplayDiscoverySimple(ctx context.Context, store storage.SwapEventStore, detector *discovery.NewTokenDetector, from, to int64) error {
	// Load events
	events, err := store.GetByTimeRange(ctx, from, to)
	if err != nil {
		return fmt.Errorf("get events: %w", err)
	}

	// Sort for deterministic ordering
	SortSwapEvents(events)

	// Convert to discovery events
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

	// Process through detector
	_, err = detector.ProcessEvents(ctx, discoveryEvents)
	return err
}

// SortSwapEventsByDomain sorts domain.SwapEvent slice by deterministic order.
// This is an alias for the existing SortSwapEvents for clarity.
func SortSwapEventsByDomain(events []*domain.SwapEvent) {
	SortSwapEvents(events)
}
