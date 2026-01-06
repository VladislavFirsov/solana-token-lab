package memory

import (
	"context"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// TokenMetadataStore is an in-memory implementation of storage.TokenMetadataStore.
type TokenMetadataStore struct {
	mu          sync.RWMutex
	byCandidate map[string]*domain.TokenMetadata // keyed by candidate_id
	byMint      map[string]*domain.TokenMetadata // keyed by mint (unique)
}

// NewTokenMetadataStore creates a new in-memory token metadata store.
func NewTokenMetadataStore() *TokenMetadataStore {
	return &TokenMetadataStore{
		byCandidate: make(map[string]*domain.TokenMetadata),
		byMint:      make(map[string]*domain.TokenMetadata),
	}
}

// Insert adds new metadata. Returns ErrDuplicateKey if candidate_id or mint already exists.
func (s *TokenMetadataStore) Insert(_ context.Context, m *domain.TokenMetadata) error {
	if m == nil || m.CandidateID == "" {
		return storage.ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byCandidate[m.CandidateID]; exists {
		return storage.ErrDuplicateKey
	}

	if _, exists := s.byMint[m.Mint]; exists {
		return storage.ErrDuplicateKey
	}

	metaCopy := *m
	s.byCandidate[m.CandidateID] = &metaCopy
	s.byMint[m.Mint] = &metaCopy
	return nil
}

// GetByID retrieves metadata by candidate ID. Returns ErrNotFound if not exists.
func (s *TokenMetadataStore) GetByID(_ context.Context, candidateID string) (*domain.TokenMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, exists := s.byCandidate[candidateID]
	if !exists {
		return nil, storage.ErrNotFound
	}

	metaCopy := *m
	return &metaCopy, nil
}

// GetByMint retrieves metadata by mint address. Returns ErrNotFound if not exists.
func (s *TokenMetadataStore) GetByMint(_ context.Context, mint string) (*domain.TokenMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, exists := s.byMint[mint]
	if !exists {
		return nil, storage.ErrNotFound
	}

	metaCopy := *m
	return &metaCopy, nil
}

var _ storage.TokenMetadataStore = (*TokenMetadataStore)(nil)
