package normalization

import (
	"sort"

	"solana-token-lab/internal/domain"
)

// SortSwaps orders swaps by (slot ASC, tx_signature ASC, event_index ASC).
// This provides deterministic ordering based on blockchain order.
func SortSwaps(swaps []*domain.Swap) {
	sort.Slice(swaps, func(i, j int) bool {
		return compareSwaps(swaps[i], swaps[j]) < 0
	})
}

// SortLiquidityEvents orders liquidity events by (slot ASC, tx_signature ASC, event_index ASC).
func SortLiquidityEvents(events []*domain.LiquidityEvent) {
	sort.Slice(events, func(i, j int) bool {
		return compareLiquidityEvents(events[i], events[j]) < 0
	})
}

// compareSwaps returns:
//   - negative if a < b
//   - zero if a == b
//   - positive if a > b
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

// compareLiquidityEvents returns:
//   - negative if a < b
//   - zero if a == b
//   - positive if a > b
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
