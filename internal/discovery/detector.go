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
}

// NewDetector creates a new NEW_TOKEN detector.
func NewDetector(store storage.CandidateStore) *NewTokenDetector {
	return &NewTokenDetector{
		seenMints:      make(map[string]bool),
		candidateStore: store,
	}
}

// ProcessEvent checks if swap is first for mint, creates candidate if so.
// Returns the created candidate, or nil if mint was already seen.
// Returns error if storage operation fails (except ErrDuplicateKey which is handled).
func (d *NewTokenDetector) ProcessEvent(ctx context.Context, event *SwapEvent) (*domain.TokenCandidate, error) {
	// Check in-memory cache first
	if d.seenMints[event.Mint] {
		return nil, nil
	}

	// Check if already exists in store (GetByMint)
	existing, err := d.candidateStore.GetByMint(ctx, event.Mint)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return nil, err
	}
	if len(existing) > 0 {
		// Already discovered, mark as seen and skip
		d.seenMints[event.Mint] = true
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
			d.seenMints[event.Mint] = true
			return nil, nil
		}
		return nil, err
	}

	// Mark as seen
	d.seenMints[event.Mint] = true

	return candidate, nil
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
func (d *NewTokenDetector) Reset() {
	d.seenMints = make(map[string]bool)
}
