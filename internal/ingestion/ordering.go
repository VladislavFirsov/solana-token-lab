package ingestion

import (
	"errors"
	"sort"

	"solana-token-lab/internal/domain"
)

// ErrInvalidOrdering is returned when events are not properly ordered.
var ErrInvalidOrdering = errors.New("events are not in deterministic order")

// SortSwaps orders swaps by (slot ASC, tx_signature ASC, event_index ASC).
// This provides deterministic ordering based on blockchain order.
func SortSwaps(swaps []*domain.Swap) {
	sort.Slice(swaps, func(i, j int) bool {
		return compareSwaps(swaps[i], swaps[j]) < 0
	})
}

// SortLiquidityEvents orders events by (slot ASC, tx_signature ASC, event_index ASC).
func SortLiquidityEvents(events []*domain.LiquidityEvent) {
	sort.Slice(events, func(i, j int) bool {
		return compareLiquidityEvents(events[i], events[j]) < 0
	})
}

// ValidateSwapOrdering checks if swaps are properly ordered.
// Returns ErrInvalidOrdering if not.
func ValidateSwapOrdering(swaps []*domain.Swap) error {
	for i := 1; i < len(swaps); i++ {
		if compareSwaps(swaps[i-1], swaps[i]) >= 0 {
			return ErrInvalidOrdering
		}
	}
	return nil
}

// ValidateLiquidityEventOrdering checks if liquidity events are properly ordered.
// Returns ErrInvalidOrdering if not.
func ValidateLiquidityEventOrdering(events []*domain.LiquidityEvent) error {
	for i := 1; i < len(events); i++ {
		if compareLiquidityEvents(events[i-1], events[i]) >= 0 {
			return ErrInvalidOrdering
		}
	}
	return nil
}

// compareSwaps returns:
//   - negative if a < b
//   - zero if a == b
//   - positive if a > b
//
// Order: (slot ASC, tx_signature ASC, event_index ASC)
func compareSwaps(a, b *domain.Swap) int {
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
	return 0
}

// compareLiquidityEvents returns comparison result for liquidity events.
// Order: (slot ASC, tx_signature ASC, event_index ASC)
func compareLiquidityEvents(a, b *domain.LiquidityEvent) int {
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
	return 0
}
