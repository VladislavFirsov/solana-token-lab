package memory

import (
	"context"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// CandidateStore is an in-memory implementation of storage.CandidateStore.
type CandidateStore struct {
	mu   sync.RWMutex
	data map[string]*domain.TokenCandidate // keyed by candidate_id
}

// NewCandidateStore creates a new in-memory candidate store.
func NewCandidateStore() *CandidateStore {
	return &CandidateStore{
		data: make(map[string]*domain.TokenCandidate),
	}
}

// Insert adds a new candidate. Returns ErrDuplicateKey if candidate_id exists.
func (s *CandidateStore) Insert(_ context.Context, c *domain.TokenCandidate) error {
	if c == nil || c.CandidateID == "" {
		return storage.ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[c.CandidateID]; exists {
		return storage.ErrDuplicateKey
	}

	// Store a copy to prevent external mutation
	candidateCopy := *c
	s.data[c.CandidateID] = &candidateCopy
	return nil
}

// GetByID retrieves a candidate by its ID. Returns ErrNotFound if not exists.
func (s *CandidateStore) GetByID(_ context.Context, candidateID string) (*domain.TokenCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, exists := s.data[candidateID]
	if !exists {
		return nil, storage.ErrNotFound
	}

	// Return a copy
	candidateCopy := *c
	return &candidateCopy, nil
}

// GetByMint retrieves all candidates for a given mint address.
func (s *CandidateStore) GetByMint(_ context.Context, mint string) ([]*domain.TokenCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.TokenCandidate
	for _, c := range s.data {
		if c.Mint == mint {
			candidateCopy := *c
			result = append(result, &candidateCopy)
		}
	}

	// Sort by discovered_at ASC
	sort.Slice(result, func(i, j int) bool {
		return result[i].DiscoveredAt < result[j].DiscoveredAt
	})

	return result, nil
}

// GetByTimeRange retrieves candidates discovered within [start, end] (inclusive).
func (s *CandidateStore) GetByTimeRange(_ context.Context, start, end int64) ([]*domain.TokenCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.TokenCandidate
	for _, c := range s.data {
		if c.DiscoveredAt >= start && c.DiscoveredAt <= end {
			candidateCopy := *c
			result = append(result, &candidateCopy)
		}
	}

	// Sort by discovered_at ASC
	sort.Slice(result, func(i, j int) bool {
		return result[i].DiscoveredAt < result[j].DiscoveredAt
	})

	return result, nil
}

// GetBySource retrieves all candidates of a given source type.
func (s *CandidateStore) GetBySource(_ context.Context, source domain.Source) ([]*domain.TokenCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.TokenCandidate
	for _, c := range s.data {
		if c.Source == source {
			candidateCopy := *c
			result = append(result, &candidateCopy)
		}
	}

	// Sort by discovered_at ASC
	sort.Slice(result, func(i, j int) bool {
		return result[i].DiscoveredAt < result[j].DiscoveredAt
	})

	return result, nil
}

// Verify interface compliance at compile time.
var _ storage.CandidateStore = (*CandidateStore)(nil)
