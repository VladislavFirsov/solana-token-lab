package ingestion

import (
	"context"
	"errors"
	"log"
	"time"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// Runner orchestrates continuous ingestion and discovery.
type Runner struct {
	wsSwapSource      *WSSwapEventSource
	wsLiquiditySource *WSLiquidityEventSource
	metadataSource    *RPCMetadataSource
	swapEventStore    storage.SwapEventStore
	liquidityStore    storage.LiquidityEventStore
	metadataStore     storage.TokenMetadataStore
	candidateStore    storage.CandidateStore
	newTokenDetector  *discovery.NewTokenDetector
	activeDetector    *discovery.ActiveTokenDetector
	checkInterval     time.Duration // Interval for ACTIVE_TOKEN detection
	slotLagWindow     int64         // Number of slots to buffer for ordering
	flushInterval     time.Duration // Interval for periodic buffer flush
	logger            *log.Logger

	// Slot-based buffer for deterministic ordering
	// Events are grouped by slot and processed when slot is finalized
	swapBuffer      map[int64][]*domain.SwapEvent
	liquidityBuffer map[int64][]*domain.LiquidityEvent
	highestSlot     int64 // Highest slot seen
	lastEventTime   int64 // Timestamp of last processed event (for deterministic detection)
}

// RunnerOptions contains configuration for creating a Runner.
type RunnerOptions struct {
	WSSwapSource      *WSSwapEventSource
	WSLiquiditySource *WSLiquidityEventSource
	MetadataSource    *RPCMetadataSource
	SwapEventStore    storage.SwapEventStore
	LiquidityStore    storage.LiquidityEventStore
	MetadataStore     storage.TokenMetadataStore
	CandidateStore    storage.CandidateStore
	NewTokenDetector  *discovery.NewTokenDetector
	ActiveDetector    *discovery.ActiveTokenDetector
	CheckInterval     time.Duration
	SlotLagWindow     int64         // Default: 5 slots - wait this many slots before processing
	FlushInterval     time.Duration // Default: 5s - force flush buffered events periodically
	Logger            *log.Logger
}

// NewRunner creates a new ingestion runner.
func NewRunner(opts RunnerOptions) *Runner {
	checkInterval := opts.CheckInterval
	if checkInterval == 0 {
		checkInterval = 1 * time.Hour
	}

	slotLagWindow := opts.SlotLagWindow
	if slotLagWindow == 0 {
		slotLagWindow = 5 // Wait 5 slots (~2 seconds) for ordering
	}

	flushInterval := opts.FlushInterval
	if flushInterval == 0 {
		flushInterval = 5 * time.Second // Force flush every 5 seconds
	}

	logger := opts.Logger
	if logger == nil {
		logger = log.Default()
	}

	return &Runner{
		wsSwapSource:      opts.WSSwapSource,
		wsLiquiditySource: opts.WSLiquiditySource,
		metadataSource:    opts.MetadataSource,
		swapEventStore:    opts.SwapEventStore,
		liquidityStore:    opts.LiquidityStore,
		metadataStore:     opts.MetadataStore,
		candidateStore:    opts.CandidateStore,
		newTokenDetector:  opts.NewTokenDetector,
		activeDetector:    opts.ActiveDetector,
		checkInterval:     checkInterval,
		slotLagWindow:     slotLagWindow,
		flushInterval:     flushInterval,
		logger:            logger,
		swapBuffer:        make(map[int64][]*domain.SwapEvent),
		liquidityBuffer:   make(map[int64][]*domain.LiquidityEvent),
	}
}

// Run starts continuous ingestion and discovery.
// It blocks until context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	r.logger.Println("Starting ingestion runner...")

	// Subscribe to swap events
	var swapEventsCh <-chan *domain.SwapEvent
	if r.wsSwapSource != nil {
		var err error
		swapEventsCh, err = r.wsSwapSource.Subscribe(ctx)
		if err != nil {
			return err
		}
		r.logger.Println("Subscribed to swap events")
	}

	// Subscribe to liquidity events
	var liquidityEventsCh <-chan *domain.LiquidityEvent
	if r.wsLiquiditySource != nil {
		var err error
		liquidityEventsCh, err = r.wsLiquiditySource.Subscribe(ctx)
		if err != nil {
			return err
		}
		r.logger.Println("Subscribed to liquidity events")
	}

	// Start ACTIVE_TOKEN detection ticker
	ticker := time.NewTicker(r.checkInterval)
	defer ticker.Stop()

	// Start periodic flush ticker to ensure buffered events are processed
	// even if no new higher slots arrive (safety net for slot buffering)
	flushTicker := time.NewTicker(r.flushInterval)
	defer flushTicker.Stop()

	r.logger.Printf("Runner started, ACTIVE_TOKEN check interval: %v, slot lag window: %d, flush interval: %v", r.checkInterval, r.slotLagWindow, r.flushInterval)

	for {
		select {
		case <-ctx.Done():
			// Flush all remaining events before shutdown
			r.flushAllSlots(ctx)
			r.logger.Println("Runner stopping...")
			return ctx.Err()

		case event, ok := <-swapEventsCh:
			if !ok {
				r.logger.Println("Swap events channel closed")
				return errors.New("swap events channel closed")
			}
			r.bufferSwapEvent(ctx, event)

		case event, ok := <-liquidityEventsCh:
			if !ok {
				r.logger.Println("Liquidity events channel closed")
				return errors.New("liquidity events channel closed")
			}
			r.bufferLiquidityEvent(ctx, event)

		case <-flushTicker.C:
			// Periodic flush: process finalized slots (respects slotLagWindow)
			// This ensures events are written even if no new slots arrive,
			// while maintaining slot-ordering guarantees.
			// flushAllSlots() is only used on shutdown when ordering no longer matters.
			r.processFinalizedSlots(ctx)

		case <-ticker.C:
			r.runActiveTokenDetection(ctx)
		}
	}
}

// bufferSwapEvent adds event to slot-based buffer and processes finalized slots.
func (r *Runner) bufferSwapEvent(ctx context.Context, event *domain.SwapEvent) {
	slot := event.Slot
	r.swapBuffer[slot] = append(r.swapBuffer[slot], event)

	// Update highest slot and process finalized slots
	if slot > r.highestSlot {
		r.highestSlot = slot
		r.processFinalizedSlots(ctx)
	} else if slot <= r.highestSlot-r.slotLagWindow {
		// Late event for already-finalized slot: process immediately
		r.processSlot(ctx, slot)
	}
}

// bufferLiquidityEvent adds event to slot-based buffer and processes finalized slots.
func (r *Runner) bufferLiquidityEvent(ctx context.Context, event *domain.LiquidityEvent) {
	slot := event.Slot
	r.liquidityBuffer[slot] = append(r.liquidityBuffer[slot], event)

	// Update highest slot and process finalized slots
	if slot > r.highestSlot {
		r.highestSlot = slot
		r.processFinalizedSlots(ctx)
	} else if slot <= r.highestSlot-r.slotLagWindow {
		// Late event for already-finalized slot: process immediately
		r.processSlot(ctx, slot)
	}
}

// processFinalizedSlots processes all slots that are finalized (behind current by lag window).
func (r *Runner) processFinalizedSlots(ctx context.Context) {
	finalizedSlot := r.highestSlot - r.slotLagWindow
	if finalizedSlot < 0 {
		return
	}

	// Collect all slots to process (in order)
	var slotsToProcess []int64
	for slot := range r.swapBuffer {
		if slot <= finalizedSlot {
			slotsToProcess = append(slotsToProcess, slot)
		}
	}
	for slot := range r.liquidityBuffer {
		if slot <= finalizedSlot {
			found := false
			for _, s := range slotsToProcess {
				if s == slot {
					found = true
					break
				}
			}
			if !found {
				slotsToProcess = append(slotsToProcess, slot)
			}
		}
	}

	// Sort slots
	sortInt64s(slotsToProcess)

	// Process each slot in order
	for _, slot := range slotsToProcess {
		r.processSlot(ctx, slot)
	}
}

// processSlot processes all events for a single slot with deterministic ordering.
func (r *Runner) processSlot(ctx context.Context, slot int64) {
	// Process swap events for this slot
	if events, ok := r.swapBuffer[slot]; ok && len(events) > 0 {
		// Sort by (tx_signature, event_index) within slot
		SortSwapEvents(events)
		for _, event := range events {
			r.handleSwapEvent(ctx, event)
		}
		delete(r.swapBuffer, slot)
	}

	// Process liquidity events for this slot
	if events, ok := r.liquidityBuffer[slot]; ok && len(events) > 0 {
		SortLiquidityEvents(events)
		for _, event := range events {
			r.handleLiquidityEvent(ctx, event)
		}
		delete(r.liquidityBuffer, slot)
	}
}

// flushAllSlots processes all remaining buffered events on shutdown.
func (r *Runner) flushAllSlots(ctx context.Context) {
	// Collect all slots
	var allSlots []int64
	for slot := range r.swapBuffer {
		allSlots = append(allSlots, slot)
	}
	for slot := range r.liquidityBuffer {
		found := false
		for _, s := range allSlots {
			if s == slot {
				found = true
				break
			}
		}
		if !found {
			allSlots = append(allSlots, slot)
		}
	}

	// Sort and process
	sortInt64s(allSlots)
	for _, slot := range allSlots {
		r.processSlot(ctx, slot)
	}
}

// sortInt64s sorts a slice of int64 in ascending order.
func sortInt64s(s []int64) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// handleSwapEvent processes a single swap event.
func (r *Runner) handleSwapEvent(ctx context.Context, event *domain.SwapEvent) {
	// Update last event time for deterministic ACTIVE_TOKEN detection
	if event.Timestamp > r.lastEventTime {
		r.lastEventTime = event.Timestamp
	}

	// Store the swap event
	if r.swapEventStore != nil {
		if err := r.swapEventStore.Insert(ctx, event); err != nil {
			if !errors.Is(err, storage.ErrDuplicateKey) {
				r.logger.Printf("Error storing swap event: %v", err)
			}
			// Duplicate is expected, not an error
		}
	}

	// Run NEW_TOKEN detection
	if r.newTokenDetector != nil {
		swapEvent := &discovery.SwapEvent{
			Mint:        event.Mint,
			Pool:        event.Pool,
			TxSignature: event.TxSignature,
			EventIndex:  event.EventIndex,
			Slot:        event.Slot,
			Timestamp:   event.Timestamp,
		}

		candidate, err := r.newTokenDetector.ProcessEvent(ctx, swapEvent)
		if err != nil {
			r.logger.Printf("Error in NEW_TOKEN detection: %v", err)
			return
		}

		if candidate != nil {
			r.logger.Printf("NEW_TOKEN discovered: %s (mint=%s)", candidate.CandidateID, candidate.Mint)

			// Fetch and store metadata for new tokens
			r.ingestMetadata(ctx, candidate.CandidateID, candidate.Mint)
		}
	}
}

// ingestMetadata fetches and stores token metadata.
func (r *Runner) ingestMetadata(ctx context.Context, candidateID, mint string) {
	if r.metadataSource == nil || r.metadataStore == nil {
		return
	}

	meta, err := r.metadataSource.Fetch(ctx, mint)
	if err != nil {
		r.logger.Printf("Error fetching metadata for %s: %v", mint, err)
		return
	}

	if meta == nil {
		return
	}

	meta.CandidateID = candidateID

	if err := r.metadataStore.Insert(ctx, meta); err != nil {
		if !errors.Is(err, storage.ErrDuplicateKey) {
			r.logger.Printf("Error storing metadata for %s: %v", mint, err)
		}
	} else {
		r.logger.Printf("Metadata stored for %s: name=%v symbol=%v decimals=%d",
			mint, meta.Name, meta.Symbol, meta.Decimals)
	}
}

// handleLiquidityEvent processes a single liquidity event.
func (r *Runner) handleLiquidityEvent(ctx context.Context, event *domain.LiquidityEvent) {
	// Store the liquidity event
	if r.liquidityStore != nil {
		if err := r.liquidityStore.Insert(ctx, event); err != nil {
			if !errors.Is(err, storage.ErrDuplicateKey) {
				r.logger.Printf("Error storing liquidity event: %v", err)
			}
		}
	}
}

// runActiveTokenDetection runs periodic ACTIVE_TOKEN spike detection.
func (r *Runner) runActiveTokenDetection(ctx context.Context) {
	if r.activeDetector == nil {
		return
	}

	// Use last processed event time for deterministic detection across live/replay
	// If no events processed yet, skip detection to maintain determinism
	evalTime := r.lastEventTime
	if evalTime == 0 {
		r.logger.Println("Skipping ACTIVE_TOKEN detection: no events processed yet")
		return
	}
	r.logger.Printf("Running ACTIVE_TOKEN detection at %d", evalTime)

	candidates, err := r.activeDetector.DetectAt(ctx, evalTime)
	if err != nil {
		r.logger.Printf("Error in ACTIVE_TOKEN detection: %v", err)
		return
	}

	for _, candidate := range candidates {
		r.logger.Printf("ACTIVE_TOKEN discovered: %s (mint=%s)", candidate.CandidateID, candidate.Mint)
	}

	if len(candidates) > 0 {
		r.logger.Printf("ACTIVE_TOKEN detection found %d candidates", len(candidates))
	}
}

// Stats returns current runner statistics.
type RunnerStats struct {
	SwapEventsProcessed      int64
	LiquidityEventsProcessed int64
	NewTokensDiscovered      int64
	ActiveTokensDiscovered   int64
	LastActiveCheck          time.Time
}
