package replay

import (
	"context"

	"solana-token-lab/internal/domain"
)

// EventType represents the type of event.
type EventType string

// Event type constants.
const (
	EventTypeSwap      EventType = "swap"
	EventTypeLiquidity EventType = "liquidity"
)

// Event represents a unified event for replay (swap or liquidity).
// Only one of Swap or Liquidity will be set based on Type.
type Event struct {
	Type        EventType
	Slot        int64
	TxSignature string
	EventIndex  int
	Timestamp   int64
	Swap        *domain.Swap
	Liquidity   *domain.LiquidityEvent
}

// ReplayEngine processes events in deterministic order.
type ReplayEngine interface {
	// OnEvent is called for each event in order.
	// Events are guaranteed to be ordered by (slot, tx_signature, event_index).
	OnEvent(ctx context.Context, event *Event) error
}
