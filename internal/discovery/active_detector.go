package discovery

import (
	"context"
	"errors"
	"math"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
	"solana-token-lab/internal/storage"
)

// Window constants in milliseconds.
const (
	Window1hMs  int64 = 3600000  // 1 hour
	Window24hMs int64 = 86400000 // 24 hours
)

// ActiveTokenConfig holds spike detection parameters.
type ActiveTokenConfig struct {
	KVol      float64 // volume spike threshold (default 3.0)
	KSwaps    float64 // swaps spike threshold (default 5.0)
	KLiq      float64 // liquidity spike threshold (default 2.0)
	Window1h  int64   // 1-hour window in ms (3600000)
	Window24h int64   // 24-hour window in ms (86400000)
}

// DefaultActiveConfig returns default configuration per spec.
func DefaultActiveConfig() ActiveTokenConfig {
	return ActiveTokenConfig{
		KVol:      3.0,
		KSwaps:    5.0,
		KLiq:      2.0, // Liquidity change threshold
		Window1h:  Window1hMs,
		Window24h: Window24hMs,
	}
}

// ActiveTokenDetector detects volume/swap/liquidity spikes for existing tokens.
type ActiveTokenDetector struct {
	config              ActiveTokenConfig
	swapEventStore      storage.SwapEventStore
	candidateStore      storage.CandidateStore
	liquidityEventStore storage.LiquidityEventStore // optional, for liquidity spike detection
	seenMints           map[string]bool
	seenMintsMu         sync.RWMutex // protects seenMints from concurrent access
}

// NewActiveDetector creates a new ACTIVE_TOKEN detector.
func NewActiveDetector(
	config ActiveTokenConfig,
	swapEventStore storage.SwapEventStore,
	candidateStore storage.CandidateStore,
) *ActiveTokenDetector {
	return &ActiveTokenDetector{
		config:         config,
		swapEventStore: swapEventStore,
		candidateStore: candidateStore,
		seenMints:      make(map[string]bool),
	}
}

// WithLiquidityStore adds liquidity event store for liquidity spike detection.
func (d *ActiveTokenDetector) WithLiquidityStore(store storage.LiquidityEventStore) *ActiveTokenDetector {
	d.liquidityEventStore = store
	return d
}

// DetectAt evaluates all mints with activity in last 24h at the given timestamp.
// Returns discovered ACTIVE_TOKEN candidates.
func (d *ActiveTokenDetector) DetectAt(ctx context.Context, evalTimestamp int64) ([]*domain.TokenCandidate, error) {
	// Get all mints with swap activity in 24h window
	start24h := evalTimestamp - d.config.Window24h
	mints, err := d.swapEventStore.GetDistinctMintsByTimeRange(ctx, start24h, evalTimestamp)
	if err != nil {
		return nil, err
	}

	// Note: Liquidity-based detection would require GetByMint on LiquidityEventStore.
	// Currently we only detect based on swap activity.

	var candidates []*domain.TokenCandidate
	for _, mint := range mints {
		candidate, err := d.EvaluateMint(ctx, mint, evalTimestamp)
		if err != nil {
			return candidates, err
		}
		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	}

	return candidates, nil
}

// EvaluateMint checks if a specific mint triggers spike at given timestamp.
// Returns candidate if spike detected, nil otherwise.
// Thread-safe: uses mutex to protect seenMints map.
func (d *ActiveTokenDetector) EvaluateMint(ctx context.Context, mint string, evalTimestamp int64) (*domain.TokenCandidate, error) {
	// Check in-memory cache first (read lock)
	d.seenMintsMu.RLock()
	seen := d.seenMints[mint]
	d.seenMintsMu.RUnlock()
	if seen {
		return nil, nil
	}

	// Check if already discovered (any source)
	existing, err := d.candidateStore.GetByMint(ctx, mint)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if len(existing) > 0 {
		d.seenMintsMu.Lock()
		d.seenMints[mint] = true
		d.seenMintsMu.Unlock()
		return nil, nil
	}

	// Compute window boundaries
	start24h := evalTimestamp - d.config.Window24h
	start1h := evalTimestamp - d.config.Window1h

	// Get swaps for 24h window
	swaps24h, err := d.swapEventStore.GetByMintTimeRange(ctx, mint, start24h, evalTimestamp)
	if err != nil {
		return nil, err
	}

	// Check volume/swap spikes
	volumeSpike, swapsSpike, triggerSwap := d.checkSwapSpikes(swaps24h, start1h, evalTimestamp)

	// Check liquidity spike if liquidity store is available
	// Note: For pre-candidate detection, we pass empty candidateID.
	// checkLiquiditySpike will return false since it can't query by mint.
	liquiditySpike, liqTrigger := false, (*domain.LiquidityEvent)(nil)
	if d.liquidityEventStore != nil {
		// Pass empty candidateID - liquidity check requires GetByMint which isn't available
		liquiditySpike, liqTrigger, err = d.checkLiquiditySpike(ctx, "", start24h, start1h, evalTimestamp)
		if err != nil {
			return nil, err
		}
	}

	// If no spike detected, return nil
	if !volumeSpike && !swapsSpike && !liquiditySpike {
		return nil, nil
	}

	// Determine trigger event: prefer swap if available, otherwise use liquidity event
	var candidate *domain.TokenCandidate
	if triggerSwap != nil {
		candidate = d.createCandidateFromSwap(triggerSwap)
	} else if liqTrigger != nil {
		candidate = d.createCandidateFromLiquidity(liqTrigger)
	} else {
		return nil, nil
	}

	// Try to insert
	err = d.candidateStore.Insert(ctx, candidate)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			d.seenMintsMu.Lock()
			d.seenMints[mint] = true
			d.seenMintsMu.Unlock()
			return nil, nil
		}
		return nil, err
	}

	d.seenMintsMu.Lock()
	d.seenMints[mint] = true
	d.seenMintsMu.Unlock()
	return candidate, nil
}

// checkSwapSpikes evaluates volume and swap count spikes.
// Returns (volumeSpike, swapsSpike, triggerSwap).
func (d *ActiveTokenDetector) checkSwapSpikes(swaps24h []*domain.SwapEvent, start1h, evalTimestamp int64) (bool, bool, *domain.SwapEvent) {
	if len(swaps24h) == 0 {
		return false, false, nil
	}

	// Compute metrics with actual history normalization
	var volume24h, volume1h float64
	var swaps1hCount int
	var firstSwapTime int64 = swaps24h[0].Timestamp

	for _, swap := range swaps24h {
		volume24h += swap.AmountOut
		if swap.Timestamp < firstSwapTime {
			firstSwapTime = swap.Timestamp
		}

		if swap.Timestamp >= start1h {
			volume1h += swap.AmountOut
			swaps1hCount++
		}
	}

	// Calculate actual history window in hours
	actualHistoryMs := evalTimestamp - firstSwapTime
	if actualHistoryMs > d.config.Window24h {
		actualHistoryMs = d.config.Window24h
	}
	if actualHistoryMs < d.config.Window1h {
		return false, false, nil // Not enough history
	}
	actualHours := float64(actualHistoryMs) / float64(d.config.Window1h)

	volume24hAvg := volume24h / actualHours
	swaps24hAvg := float64(len(swaps24h)) / actualHours

	// Check spike conditions
	volumeSpike := volume1h > d.config.KVol*volume24hAvg
	swapsSpike := float64(swaps1hCount) > d.config.KSwaps*swaps24hAvg

	if !volumeSpike && !swapsSpike {
		return false, false, nil
	}

	// Find triggering swap
	triggerSwap := d.findTriggerSwap(swaps24h, start1h, evalTimestamp)
	return volumeSpike, swapsSpike, triggerSwap
}

// findTriggerSwap finds the swap that triggered the spike.
// Uses max timestamp with deterministic tie-breaker per NORMALIZATION_SPEC.md.
func (d *ActiveTokenDetector) findTriggerSwap(swaps24h []*domain.SwapEvent, start1h, evalTimestamp int64) *domain.SwapEvent {
	var triggerSwap *domain.SwapEvent
	for _, swap := range swaps24h {
		if swap.Timestamp >= start1h && swap.Timestamp < evalTimestamp {
			if triggerSwap == nil {
				triggerSwap = swap
			} else if swap.Timestamp > triggerSwap.Timestamp {
				triggerSwap = swap
			} else if swap.Timestamp == triggerSwap.Timestamp {
				// Tie-breaker: (slot, tx_signature, event_index) ASC
				if swap.Slot < triggerSwap.Slot {
					triggerSwap = swap
				} else if swap.Slot == triggerSwap.Slot {
					if swap.TxSignature < triggerSwap.TxSignature {
						triggerSwap = swap
					} else if swap.TxSignature == triggerSwap.TxSignature {
						if swap.EventIndex < triggerSwap.EventIndex {
							triggerSwap = swap
						}
					}
				}
			}
		}
	}
	return triggerSwap
}

// checkLiquiditySpike evaluates liquidity changes for spike detection.
// Returns (liquiditySpike, triggerEvent, error).
// Note: This requires candidateID to query events. For pre-candidate detection,
// we'd need GetByMint method which isn't in the current interface.
// For now, this returns false when used for ACTIVE_TOKEN detection.
func (d *ActiveTokenDetector) checkLiquiditySpike(ctx context.Context, candidateID string, start24h, start1h, evalTimestamp int64) (bool, *domain.LiquidityEvent, error) {
	if candidateID == "" {
		// Cannot check liquidity without candidateID
		// This is expected for ACTIVE_TOKEN detection before candidate exists
		return false, nil, nil
	}

	// Get liquidity events for 24h window
	events, err := d.liquidityEventStore.GetByTimeRange(ctx, candidateID, start24h, evalTimestamp)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return false, nil, nil
		}
		return false, nil, err
	}

	if len(events) == 0 {
		return false, nil, nil
	}

	// Calculate absolute liquidity change in 1h window
	var liqChange1h float64
	var firstEventTime int64 = events[0].Timestamp
	var totalAbsChange float64

	for _, evt := range events {
		if evt.Timestamp < firstEventTime {
			firstEventTime = evt.Timestamp
		}
		// Use AmountQuote as liquidity proxy (SOL/USDC added/removed)
		change := evt.AmountQuote
		if evt.EventType == "remove" {
			change = -change
		}
		totalAbsChange += math.Abs(change)

		if evt.Timestamp >= start1h {
			liqChange1h += math.Abs(change)
		}
	}

	// Calculate actual history window
	actualHistoryMs := evalTimestamp - firstEventTime
	if actualHistoryMs > d.config.Window24h {
		actualHistoryMs = d.config.Window24h
	}
	if actualHistoryMs < d.config.Window1h {
		return false, nil, nil
	}
	actualHours := float64(actualHistoryMs) / float64(d.config.Window1h)

	liqChange24hAvg := totalAbsChange / actualHours

	// Check spike condition
	liquiditySpike := liqChange1h > d.config.KLiq*liqChange24hAvg

	if !liquiditySpike {
		return false, nil, nil
	}

	// Find triggering liquidity event
	triggerEvent := d.findTriggerLiquidityEvent(events, start1h, evalTimestamp)
	return true, triggerEvent, nil
}

// findTriggerLiquidityEvent finds the liquidity event that triggered the spike.
func (d *ActiveTokenDetector) findTriggerLiquidityEvent(events []*domain.LiquidityEvent, start1h, evalTimestamp int64) *domain.LiquidityEvent {
	var trigger *domain.LiquidityEvent
	for _, evt := range events {
		if evt.Timestamp >= start1h && evt.Timestamp < evalTimestamp {
			if trigger == nil {
				trigger = evt
			} else if evt.Timestamp > trigger.Timestamp {
				trigger = evt
			} else if evt.Timestamp == trigger.Timestamp {
				// Tie-breaker: (slot, tx_signature, event_index) ASC
				if evt.Slot < trigger.Slot {
					trigger = evt
				} else if evt.Slot == trigger.Slot {
					if evt.TxSignature < trigger.TxSignature {
						trigger = evt
					} else if evt.TxSignature == trigger.TxSignature {
						if evt.EventIndex < trigger.EventIndex {
							trigger = evt
						}
					}
				}
			}
		}
	}
	return trigger
}

// createCandidateFromSwap creates a candidate from a swap event.
func (d *ActiveTokenDetector) createCandidateFromSwap(swap *domain.SwapEvent) *domain.TokenCandidate {
	candidateID := idhash.ComputeCandidateID(
		swap.Mint,
		swap.Pool,
		domain.SourceActiveToken,
		swap.TxSignature,
		swap.EventIndex,
		swap.Slot,
	)

	return &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceActiveToken,
		Mint:         swap.Mint,
		Pool:         swap.Pool,
		TxSignature:  swap.TxSignature,
		EventIndex:   swap.EventIndex,
		Slot:         swap.Slot,
		DiscoveredAt: swap.Timestamp,
	}
}

// createCandidateFromLiquidity creates a candidate from a liquidity event.
func (d *ActiveTokenDetector) createCandidateFromLiquidity(evt *domain.LiquidityEvent) *domain.TokenCandidate {
	pool := evt.Pool // Convert string to *string
	candidateID := idhash.ComputeCandidateID(
		evt.Mint,
		&pool,
		domain.SourceActiveToken,
		evt.TxSignature,
		evt.EventIndex,
		evt.Slot,
	)

	return &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceActiveToken,
		Mint:         evt.Mint,
		Pool:         &pool,
		TxSignature:  evt.TxSignature,
		EventIndex:   evt.EventIndex,
		Slot:         evt.Slot,
		DiscoveredAt: evt.Timestamp,
	}
}

// Reset clears the in-memory seen mints cache.
// Thread-safe: uses mutex to protect seenMints map.
func (d *ActiveTokenDetector) Reset() {
	d.seenMintsMu.Lock()
	d.seenMints = make(map[string]bool)
	d.seenMintsMu.Unlock()
}
