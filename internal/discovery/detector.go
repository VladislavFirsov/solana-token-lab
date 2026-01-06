package discovery

import (
	"context"
	"errors"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/idhash"
	"solana-token-lab/internal/storage"
)

// NewTokenDetector detects first swaps for new mints.
type NewTokenDetector struct {
	seenMints      map[string]bool
	candidateStore storage.CandidateStore
	progressStore  storage.DiscoveryProgressStore // optional, for persistence
}

// NewDetector creates a new NEW_TOKEN detector.
func NewDetector(store storage.CandidateStore) *NewTokenDetector {
	return &NewTokenDetector{
		seenMints:      make(map[string]bool),
		candidateStore: store,
	}
}

// WithProgressStore adds persistence for discovery state.
// Enables resumption after restarts without reprocessing candidates.
func (d *NewTokenDetector) WithProgressStore(store storage.DiscoveryProgressStore) *NewTokenDetector {
	d.progressStore = store
	return d
}

// LoadState loads previously seen mints from persistent storage.
// Call this at startup to resume from previous state.
func (d *NewTokenDetector) LoadState(ctx context.Context) error {
	if d.progressStore == nil {
		return nil // No persistence configured
	}

	mints, err := d.progressStore.LoadSeenMints(ctx)
	if err != nil {
		return err
	}

	// Populate in-memory cache
	for _, mint := range mints {
		d.seenMints[mint] = true
	}

	return nil
}

// GetProgress returns the last processed position.
// Returns nil if no progress saved or no persistence configured.
func (d *NewTokenDetector) GetProgress(ctx context.Context) (*storage.DiscoveryProgress, error) {
	if d.progressStore == nil {
		return nil, nil
	}

	progress, err := d.progressStore.GetLastProcessed(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return progress, nil
}

// SaveProgress saves the last processed position.
// No-op if persistence is not configured.
func (d *NewTokenDetector) SaveProgress(ctx context.Context, slot uint64, signature string) error {
	if d.progressStore == nil {
		return nil
	}

	return d.progressStore.SetLastProcessed(ctx, &storage.DiscoveryProgress{
		Slot:      slot,
		Signature: signature,
	})
}

// ProcessEvent checks if swap is first for mint, creates candidate if so.
// Returns the created candidate, or nil if mint was already seen.
// Returns error if storage operation fails (except ErrDuplicateKey which is handled).
func (d *NewTokenDetector) ProcessEvent(ctx context.Context, event *SwapEvent) (*domain.TokenCandidate, error) {
	// Check in-memory cache first
	if d.seenMints[event.Mint] {
		return nil, nil
	}

	// Check persistent store if available
	if d.progressStore != nil {
		seen, err := d.progressStore.IsMintSeen(ctx, event.Mint)
		if err != nil {
			return nil, err
		}
		if seen {
			d.seenMints[event.Mint] = true
			return nil, nil
		}
	}

	// Check if already exists in store (GetByMint)
	existing, err := d.candidateStore.GetByMint(ctx, event.Mint)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if len(existing) > 0 {
		// Already discovered, mark as seen and skip
		if err := d.markSeen(ctx, event.Mint); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// First swap for this mint — create candidate
	candidateID := idhash.ComputeCandidateID(
		event.Mint,
		event.Pool,
		domain.SourceNewToken,
		event.TxSignature,
		event.EventIndex,
		event.Slot,
	)

	candidate := &domain.TokenCandidate{
		CandidateID:  candidateID,
		Source:       domain.SourceNewToken,
		Mint:         event.Mint,
		Pool:         event.Pool,
		TxSignature:  event.TxSignature,
		EventIndex:   event.EventIndex,
		Slot:         event.Slot,
		DiscoveredAt: event.Timestamp,
	}

	// Try to insert
	err = d.candidateStore.Insert(ctx, candidate)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateKey) {
			// Race condition: another process inserted first
			if err := d.markSeen(ctx, event.Mint); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return nil, err
	}

	// Mark as seen
	if err := d.markSeen(ctx, event.Mint); err != nil {
		return nil, err
	}

	return candidate, nil
}

// markSeen marks a mint as seen in both in-memory cache and persistent store.
func (d *NewTokenDetector) markSeen(ctx context.Context, mint string) error {
	d.seenMints[mint] = true

	if d.progressStore != nil {
		return d.progressStore.MarkMintSeen(ctx, mint)
	}

	return nil
}

// ProcessEvents processes events in order, returns discovered candidates.
// Events should be pre-sorted by (slot, tx_signature, event_index).
func (d *NewTokenDetector) ProcessEvents(ctx context.Context, events []*SwapEvent) ([]*domain.TokenCandidate, error) {
	// Sort events for deterministic ordering
	SortSwapEvents(events)

	var candidates []*domain.TokenCandidate

	for _, event := range events {
		candidate, err := d.ProcessEvent(ctx, event)
		if err != nil {
			return candidates, err
		}
		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	}

	return candidates, nil
}

// Reset clears the in-memory seen mints cache.
// Useful for replay scenarios where we want to re-detect from storage state.
// Note: Does NOT clear persistent storage.
func (d *NewTokenDetector) Reset() {
	d.seenMints = make(map[string]bool)
}

// SeenCount returns the number of mints in the in-memory cache.
func (d *NewTokenDetector) SeenCount() int {
	return len(d.seenMints)
}
