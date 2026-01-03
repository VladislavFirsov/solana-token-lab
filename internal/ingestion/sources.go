package ingestion

import (
	"context"

	"solana-token-lab/internal/domain"
)

// SwapSource provides raw swap events from external sources.
type SwapSource interface {
	// Fetch returns swaps for a candidate within time range [from, to] (inclusive).
	// Events may be unordered; Manager enforces deterministic ordering.
	Fetch(ctx context.Context, candidateID string, from, to int64) ([]*domain.Swap, error)
}

// LiquidityEventSource provides raw liquidity events from external sources.
type LiquidityEventSource interface {
	// Fetch returns liquidity events for a candidate within time range [from, to] (inclusive).
	// Events may be unordered; Manager enforces deterministic ordering.
	Fetch(ctx context.Context, candidateID string, from, to int64) ([]*domain.LiquidityEvent, error)
}

// MetadataSource provides token metadata from external sources.
type MetadataSource interface {
	// Fetch returns token metadata for a given mint address.
	Fetch(ctx context.Context, mint string) (*domain.TokenMetadata, error)
}
