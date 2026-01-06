package storage

import "context"

// DiscoveryProgress represents the last processed position in the chain.
type DiscoveryProgress struct {
	Slot      uint64 // last processed Solana slot
	Signature string // last processed transaction signature
}

// DiscoveryProgressStore provides persistence for discovery state.
// This enables resumption after restarts without reprocessing or duplicating candidates.
type DiscoveryProgressStore interface {
	// GetLastProcessed returns the last processed slot and signature.
	// Returns ErrNotFound if no progress has been saved yet.
	GetLastProcessed(ctx context.Context) (*DiscoveryProgress, error)

	// SetLastProcessed saves the last processed slot and signature.
	SetLastProcessed(ctx context.Context, progress *DiscoveryProgress) error

	// IsMintSeen checks if a mint address has been processed.
	IsMintSeen(ctx context.Context, mint string) (bool, error)

	// MarkMintSeen records that a mint address has been processed.
	MarkMintSeen(ctx context.Context, mint string) error

	// LoadSeenMints returns all seen mints (for warming the in-memory cache).
	LoadSeenMints(ctx context.Context) ([]string, error)
}
