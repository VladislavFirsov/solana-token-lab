package memory

import (
	"context"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// TradeRecordStore is an in-memory implementation of storage.TradeRecordStore.
type TradeRecordStore struct {
	mu   sync.RWMutex
	data map[string]*domain.TradeRecord // keyed by trade_id
}

// NewTradeRecordStore creates a new in-memory trade record store.
func NewTradeRecordStore() *TradeRecordStore {
	return &TradeRecordStore{
		data: make(map[string]*domain.TradeRecord),
	}
}

// Insert adds a new trade. Returns ErrDuplicateKey if trade_id exists.
func (s *TradeRecordStore) Insert(_ context.Context, t *domain.TradeRecord) error {
	if t == nil || t.TradeID == "" {
		return storage.ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[t.TradeID]; exists {
		return storage.ErrDuplicateKey
	}

	copy := *t
	s.data[t.TradeID] = &copy
	return nil
}

// InsertBulk adds multiple trades atomically. Fails entire batch on any duplicate.
func (s *TradeRecordStore) InsertBulk(_ context.Context, trades []*domain.TradeRecord) error {
	if len(trades) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track keys in this batch to detect intra-batch duplicates
	batchKeys := make(map[string]struct{}, len(trades))

	// First pass: check for duplicates (existing + intra-batch)
	for _, t := range trades {
		if t == nil || t.TradeID == "" {
			return storage.ErrInvalidInput
		}

		// Check existing data
		if _, exists := s.data[t.TradeID]; exists {
			return storage.ErrDuplicateKey
		}
		// Check intra-batch duplicate
		if _, exists := batchKeys[t.TradeID]; exists {
			return storage.ErrDuplicateKey
		}
		batchKeys[t.TradeID] = struct{}{}
	}

	// Second pass: insert all
	for _, t := range trades {
		copy := *t
		s.data[t.TradeID] = &copy
	}

	return nil
}

// GetByID retrieves a trade by its ID. Returns ErrNotFound if not exists.
func (s *TradeRecordStore) GetByID(_ context.Context, tradeID string) (*domain.TradeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, exists := s.data[tradeID]
	if !exists {
		return nil, storage.ErrNotFound
	}

	copy := *t
	return &copy, nil
}

// GetByCandidateID retrieves all trades for a candidate, ordered by entry_signal_time ASC.
func (s *TradeRecordStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.TradeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.TradeRecord
	for _, t := range s.data {
		if t.CandidateID == candidateID {
			copy := *t
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].EntrySignalTime < result[j].EntrySignalTime
	})

	return result, nil
}

// GetByStrategyScenario retrieves all trades for a strategy/scenario combination.
func (s *TradeRecordStore) GetByStrategyScenario(_ context.Context, strategyID, scenarioID string) ([]*domain.TradeRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.TradeRecord
	for _, t := range s.data {
		if t.StrategyID == strategyID && t.ScenarioID == scenarioID {
			copy := *t
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].EntrySignalTime < result[j].EntrySignalTime
	})

	return result, nil
}

var _ storage.TradeRecordStore = (*TradeRecordStore)(nil)
