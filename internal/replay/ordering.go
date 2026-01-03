package replay

import (
	"sort"

	"solana-token-lab/internal/domain"
)

// SortEvents orders events by (slot ASC, tx_signature ASC, event_index ASC, type ASC).
// This provides deterministic ordering based on blockchain order.
// EventType is used as tie-breaker when slot, tx_signature, and event_index are equal.
func SortEvents(events []*Event) {
	sort.Slice(events, func(i, j int) bool {
		return compareEvents(events[i], events[j]) < 0
	})
}

// MergeEvents combines swaps and liquidity events into a sorted event stream.
// Returns events ordered by (slot, tx_signature, event_index).
func MergeEvents(swaps []*domain.Swap, liquidity []*domain.LiquidityEvent) []*Event {
	events := make([]*Event, 0, len(swaps)+len(liquidity))

	for _, s := range swaps {
		events = append(events, &Event{
			Type:        EventTypeSwap,
			Slot:        s.Slot,
			TxSignature: s.TxSignature,
			EventIndex:  s.EventIndex,
			Timestamp:   s.Timestamp,
			Swap:        s,
		})
	}

	for _, l := range liquidity {
		events = append(events, &Event{
			Type:        EventTypeLiquidity,
			Slot:        l.Slot,
			TxSignature: l.TxSignature,
			EventIndex:  l.EventIndex,
			Timestamp:   l.Timestamp,
			Liquidity:   l,
		})
	}

	SortEvents(events)
	return events
}

// compareEvents returns:
//   - negative if a < b
//   - zero if a == b
//   - positive if a > b
//
// Order: (slot ASC, tx_signature ASC, event_index ASC, type ASC)
// EventType order: "liquidity" < "swap" (alphabetically)
func compareEvents(a, b *Event) int {
	if a.Slot != b.Slot {
		if a.Slot < b.Slot {
			return -1
		}
		return 1
	}
	if a.TxSignature != b.TxSignature {
		if a.TxSignature < b.TxSignature {
			return -1
		}
		return 1
	}
	if a.EventIndex != b.EventIndex {
		if a.EventIndex < b.EventIndex {
			return -1
		}
		return 1
	}
	// Tie-breaker: compare EventType (deterministic ordering when composite key is equal)
	if a.Type != b.Type {
		if a.Type < b.Type {
			return -1
		}
		return 1
	}
	return 0
}
