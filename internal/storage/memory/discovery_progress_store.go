package memory

import (
	"context"
	"sync"

	"solana-token-lab/internal/storage"
)

// DiscoveryProgressStore is an in-memory implementation of storage.DiscoveryProgressStore.
type DiscoveryProgressStore struct {
	mu        sync.RWMutex
	progress  *storage.DiscoveryProgress
	seenMints map[string]bool
}

// NewDiscoveryProgressStore creates a new in-memory discovery progress store.
func NewDiscoveryProgressStore() *DiscoveryProgressStore {
	return &DiscoveryProgressStore{
		seenMints: make(map[string]bool),
	}
}

// GetLastProcessed returns the last processed slot and signature.
func (s *DiscoveryProgressStore) GetLastProcessed(_ context.Context) (*storage.DiscoveryProgress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.progress == nil {
		return nil, storage.ErrNotFound
	}

	return &storage.DiscoveryProgress{
		Slot:      s.progress.Slot,
		Signature: s.progress.Signature,
	}, nil
}

// SetLastProcessed saves the last processed slot and signature.
func (s *DiscoveryProgressStore) SetLastProcessed(_ context.Context, progress *storage.DiscoveryProgress) error {
	if progress == nil {
		return storage.ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress = &storage.DiscoveryProgress{
		Slot:      progress.Slot,
		Signature: progress.Signature,
	}
	return nil
}

// IsMintSeen checks if a mint address has been processed.
func (s *DiscoveryProgressStore) IsMintSeen(_ context.Context, mint string) (bool, error) {
	if mint == "" {
		return false, storage.ErrInvalidInput
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.seenMints[mint], nil
}

// MarkMintSeen records that a mint address has been processed.
func (s *DiscoveryProgressStore) MarkMintSeen(_ context.Context, mint string) error {
	if mint == "" {
		return storage.ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seenMints[mint] = true
	return nil
}

// LoadSeenMints returns all seen mints.
func (s *DiscoveryProgressStore) LoadSeenMints(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mints := make([]string, 0, len(s.seenMints))
	for mint := range s.seenMints {
		mints = append(mints, mint)
	}
	return mints, nil
}
