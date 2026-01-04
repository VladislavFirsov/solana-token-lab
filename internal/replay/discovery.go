package replay

import (
	"context"
	"sort"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// DiscoveryReplay replays discovery from stored swap events.
// Produces deterministic candidate stream (NEW_TOKEN + ACTIVE_TOKEN).
type DiscoveryReplay struct {
	swapEventStore storage.SwapEventStore
	// candidateStore must be isolated (in-memory or wrapper) to avoid mutating production data.
	// It is shared by NEW/ACTIVE detectors so NEW_TOKEN inserts exclude later ACTIVE_TOKEN.
	candidateStore storage.CandidateStore
	activeConfig   discovery.ActiveTokenConfig
}

// NewDiscoveryReplay creates a new discovery replay runner.
func NewDiscoveryReplay(
	swapEventStore storage.SwapEventStore,
	candidateStore storage.CandidateStore,
	activeConfig discovery.ActiveTokenConfig,
) *DiscoveryReplay {
	return &DiscoveryReplay{
		swapEventStore: swapEventStore,
		candidateStore: candidateStore,
		activeConfig:   activeConfig,
	}
}

// Run replays discovery for a time range [from, to).
// Returns candidates in deterministic discovery order.
// Process:
//  1. Load swap events from storage
//  2. Sort by (slot, tx_signature, event_index)
//  3. Process through NewTokenDetector (emits NEW_TOKEN)
//  4. At each event timestamp, evaluate ACTIVE_TOKEN detection
//
// Returns: candidates in emission order (already deterministic)
//
// Note: For partial replays, the caller should pre-seed candidateStore with
// any candidates discovered before `from` to preserve uniqueness rules.
func (r *DiscoveryReplay) Run(ctx context.Context, from, to int64) ([]*domain.TokenCandidate, error) {
	// 1. Load swap events from storage
	domainEvents, err := r.swapEventStore.GetByTimeRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	if len(domainEvents) == 0 {
		return nil, nil
	}

	// 2. Sort by (slot, tx_signature, event_index) for deterministic ordering
	sortDomainSwapEvents(domainEvents)

	// 3. Create fresh detectors
	newTokenDetector := discovery.NewDetector(r.candidateStore)
	activeTokenDetector := discovery.NewActiveDetector(r.activeConfig, r.swapEventStore, r.candidateStore)

	// 4. Process each event in order
	var candidates []*domain.TokenCandidate

	for _, domainEvent := range domainEvents {
		// Convert domain.SwapEvent -> discovery.SwapEvent
		discEvent := domainToDiscoveryEvent(domainEvent)

		// Try NEW_TOKEN detection (first swap for mint)
		newCandidate, err := newTokenDetector.ProcessEvent(ctx, discEvent)
		if err != nil {
			return candidates, err
		}
		if newCandidate != nil {
			candidates = append(candidates, newCandidate)
		}

		// Try ACTIVE_TOKEN detection (spike evaluation at this timestamp)
		// Only evaluate if mint was not just discovered as NEW_TOKEN
		if newCandidate == nil {
			activeCandidate, err := activeTokenDetector.EvaluateMint(ctx, domainEvent.Mint, domainEvent.Timestamp)
			if err != nil {
				return candidates, err
			}
			if activeCandidate != nil {
				candidates = append(candidates, activeCandidate)
			}
		}
	}

	return candidates, nil
}

// sortDomainSwapEvents sorts events by (slot, tx_signature, event_index).
func sortDomainSwapEvents(events []*domain.SwapEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Slot != events[j].Slot {
			return events[i].Slot < events[j].Slot
		}
		if events[i].TxSignature != events[j].TxSignature {
			return events[i].TxSignature < events[j].TxSignature
		}
		return events[i].EventIndex < events[j].EventIndex
	})
}

// domainToDiscoveryEvent converts domain.SwapEvent to discovery.SwapEvent.
func domainToDiscoveryEvent(e *domain.SwapEvent) *discovery.SwapEvent {
	return &discovery.SwapEvent{
		Mint:        e.Mint,
		Pool:        e.Pool,
		TxSignature: e.TxSignature,
		EventIndex:  e.EventIndex,
		Slot:        e.Slot,
		Timestamp:   e.Timestamp,
	}
}
