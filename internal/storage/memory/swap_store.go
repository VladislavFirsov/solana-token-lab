package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// SwapStore is an in-memory implementation of storage.SwapStore.
type SwapStore struct {
	mu   sync.RWMutex
	data map[string]*domain.Swap // keyed by composite key
}

// NewSwapStore creates a new in-memory swap store.
func NewSwapStore() *SwapStore {
	return &SwapStore{
		data: make(map[string]*domain.Swap),
	}
}

// swapKey generates a unique key for a swap.
func swapKey(candidateID, txSignature string, eventIndex int) string {
	return fmt.Sprintf("%s|%s|%d", candidateID, txSignature, eventIndex)
}

// Insert adds a new swap. Returns ErrDuplicateKey if exists.
func (s *SwapStore) Insert(_ context.Context, swap *domain.Swap) error {
	if swap == nil || swap.CandidateID == "" {
		return storage.ErrInvalidInput
	}

	key := swapKey(swap.CandidateID, swap.TxSignature, swap.EventIndex)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; exists {
		return storage.ErrDuplicateKey
	}

	copy := *swap
	s.data[key] = &copy
	return nil
}

// InsertBulk adds multiple swaps atomically. Fails entire batch on any duplicate.
func (s *SwapStore) InsertBulk(_ context.Context, swaps []*domain.Swap) error {
	if len(swaps) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track keys in this batch to detect intra-batch duplicates
	batchKeys := make(map[string]struct{}, len(swaps))

	// First pass: check for duplicates (existing + intra-batch)
	for _, swap := range swaps {
		if swap == nil || swap.CandidateID == "" {
			return storage.ErrInvalidInput
		}
		key := swapKey(swap.CandidateID, swap.TxSignature, swap.EventIndex)

		// Check existing data
		if _, exists := s.data[key]; exists {
			return storage.ErrDuplicateKey
		}
		// Check intra-batch duplicate
		if _, exists := batchKeys[key]; exists {
			return storage.ErrDuplicateKey
		}
		batchKeys[key] = struct{}{}
	}

	// Second pass: insert all
	for _, swap := range swaps {
		key := swapKey(swap.CandidateID, swap.TxSignature, swap.EventIndex)
		copy := *swap
		s.data[key] = &copy
	}

	return nil
}

// GetByCandidateID retrieves all swaps for a candidate, ordered by timestamp ASC.
func (s *SwapStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.Swap, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Swap
	for _, swap := range s.data {
		if swap.CandidateID == candidateID {
			copy := *swap
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Timestamp != result[j].Timestamp {
			return result[i].Timestamp < result[j].Timestamp
		}
		return result[i].Slot < result[j].Slot
	})

	return result, nil
}

// GetByTimeRange retrieves swaps for a candidate within [start, end] (inclusive).
func (s *SwapStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.Swap, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Swap
	for _, swap := range s.data {
		if swap.CandidateID == candidateID && swap.Timestamp >= start && swap.Timestamp <= end {
			copy := *swap
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Timestamp != result[j].Timestamp {
			return result[i].Timestamp < result[j].Timestamp
		}
		return result[i].Slot < result[j].Slot
	})

	return result, nil
}

var _ storage.SwapStore = (*SwapStore)(nil)
