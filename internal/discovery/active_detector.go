package discovery

import (
	"context"
	"errors"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
	"solana-token-lab/internal/storage"
)

// Window constants in milliseconds.
const (
	Window1hMs  int64 = 3600000    // 1 hour
	Window24hMs int64 = 86400000   // 24 hours
)

// ActiveTokenConfig holds spike detection parameters.
type ActiveTokenConfig struct {
	KVol      float64 // volume spike threshold (default 3.0)
	KSwaps    float64 // swaps spike threshold (default 5.0)
	Window1h  int64   // 1-hour window in ms (3600000)
	Window24h int64   // 24-hour window in ms (86400000)
}

// DefaultActiveConfig returns default configuration per spec.
func DefaultActiveConfig() ActiveTokenConfig {
	return ActiveTokenConfig{
		KVol:      3.0,
		KSwaps:    5.0,
		Window1h:  Window1hMs,
		Window24h: Window24hMs,
	}
}

// ActiveTokenDetector detects volume/swap spikes for existing tokens.
type ActiveTokenDetector struct {
	config         ActiveTokenConfig
	swapEventStore storage.SwapEventStore
	candidateStore storage.CandidateStore
	seenMints      map[string]bool
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

// DetectAt evaluates all mints with activity in last 24h at the given timestamp.
// Returns discovered ACTIVE_TOKEN candidates.
func (d *ActiveTokenDetector) DetectAt(ctx context.Context, evalTimestamp int64) ([]*domain.TokenCandidate, error) {
	// Get all mints with activity in 24h window
	start24h := evalTimestamp - d.config.Window24h
	mints, err := d.swapEventStore.GetDistinctMintsByTimeRange(ctx, start24h, evalTimestamp)
	if err != nil {
		return nil, err
	}

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
func (d *ActiveTokenDetector) EvaluateMint(ctx context.Context, mint string, evalTimestamp int64) (*domain.TokenCandidate, error) {
	// Check in-memory cache first
	if d.seenMints[mint] {
		return nil, nil
	}

	// Check if already discovered (any source)
	existing, err := d.candidateStore.GetByMint(ctx, mint)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if len(existing) > 0 {
		d.seenMints[mint] = true
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

	if len(swaps24h) == 0 {
		return nil, nil
	}

	// Compute metrics
	var volume24h, volume1h float64
	var swaps1hCount int

	for _, swap := range swaps24h {
		volume24h += swap.AmountOut

		if swap.Timestamp >= start1h {
			volume1h += swap.AmountOut
			swaps1hCount++
		}
	}

	volume24hAvg := volume24h / 24.0
	swaps24hAvg := float64(len(swaps24h)) / 24.0

	// Check spike conditions
	volumeSpike := volume1h > d.config.KVol*volume24hAvg
	swapsSpike := float64(swaps1hCount) > d.config.KSwaps*swaps24hAvg

	if !volumeSpike && !swapsSpike {
		return nil, nil
	}

	// Find triggering swap: max timestamp in 1h window, with deterministic tie-breaker
	var triggerSwap *domain.SwapEvent
	for _, swap := range swaps24h {
		if swap.Timestamp >= start1h && swap.Timestamp < evalTimestamp {
			if triggerSwap == nil {
				triggerSwap = swap
			} else if swap.Timestamp > triggerSwap.Timestamp {
				// Higher timestamp wins
				triggerSwap = swap
			} else if swap.Timestamp == triggerSwap.Timestamp {
				// Tie-breaker: (slot, tx_signature, event_index) DESC
				if swap.Slot > triggerSwap.Slot {
					triggerSwap = swap
				} else if swap.Slot == triggerSwap.Slot {
					if swap.TxSignature > triggerSwap.TxSignature {
						triggerSwap = swap
					} else if swap.TxSignature == triggerSwap.TxSignature {
						if swap.EventIndex > triggerSwap.EventIndex {
							triggerSwap = swap
						}
					}
				}
			}
		}
	}

	if triggerSwap == nil {
		// No swap in 1h window (shouldn't happen if we got here)
		return nil, nil
	}

	// Create candidate
	candidateID := idhash.ComputeCandidateID(
		triggerSwap.Mint,
		triggerSwap.Pool,
		domain.SourceActiveToken,
		triggerSwap.TxSignature,
		triggerSwap.EventIndex,
		triggerSwap.Slot,
	)

	candidate := &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceActiveToken,
		Mint:         triggerSwap.Mint,
		Pool:         triggerSwap.Pool,
		TxSignature:  triggerSwap.TxSignature,
		EventIndex:   triggerSwap.EventIndex,
		Slot:         triggerSwap.Slot,
		DiscoveredAt: triggerSwap.Timestamp,
	}

	// Try to insert
	err = d.candidateStore.Insert(ctx, candidate)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			d.seenMints[mint] = true
			return nil, nil
		}
		return nil, err
	}

	d.seenMints[mint] = true
	return candidate, nil
}

// Reset clears the in-memory seen mints cache.
func (d *ActiveTokenDetector) Reset() {
	d.seenMints = make(map[string]bool)
}
