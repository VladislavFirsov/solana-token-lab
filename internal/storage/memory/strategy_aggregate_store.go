package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// StrategyAggregateStore is an in-memory implementation of storage.StrategyAggregateStore.
type StrategyAggregateStore struct {
	mu   sync.RWMutex
	data map[string]*domain.StrategyAggregate // keyed by composite key
}

// NewStrategyAggregateStore creates a new in-memory strategy aggregate store.
func NewStrategyAggregateStore() *StrategyAggregateStore {
	return &StrategyAggregateStore{
		data: make(map[string]*domain.StrategyAggregate),
	}
}

// aggregateKey generates a unique key for an aggregate.
func aggregateKey(strategyID, scenarioID, entryEventType string) string {
	return fmt.Sprintf("%s|%s|%s", strategyID, scenarioID, entryEventType)
}

// Insert adds a new aggregate. Returns ErrDuplicateKey if key exists.
func (s *StrategyAggregateStore) Insert(_ context.Context, a *domain.StrategyAggregate) error {
	if a == nil || a.StrategyID == "" || a.ScenarioID == "" || a.EntryEventType == "" {
		return storage.ErrInvalidInput
	}

	key := aggregateKey(a.StrategyID, a.ScenarioID, a.EntryEventType)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; exists {
		return storage.ErrDuplicateKey
	}

	aggCopy := *a
	s.data[key] = &aggCopy
	return nil
}

// InsertBulk adds multiple aggregates atomically. Fails entire batch on any duplicate.
func (s *StrategyAggregateStore) InsertBulk(_ context.Context, aggregates []*domain.StrategyAggregate) error {
	if len(aggregates) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track keys in this batch to detect intra-batch duplicates
	batchKeys := make(map[string]struct{}, len(aggregates))

	// First pass: check for duplicates (existing + intra-batch)
	for _, a := range aggregates {
		if a == nil || a.StrategyID == "" || a.ScenarioID == "" || a.EntryEventType == "" {
			return storage.ErrInvalidInput
		}
		key := aggregateKey(a.StrategyID, a.ScenarioID, a.EntryEventType)

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
	for _, a := range aggregates {
		key := aggregateKey(a.StrategyID, a.ScenarioID, a.EntryEventType)
		aggCopy := *a
		s.data[key] = &aggCopy
	}

	return nil
}

// GetByKey retrieves an aggregate by its composite key. Returns ErrNotFound if not exists.
func (s *StrategyAggregateStore) GetByKey(_ context.Context, strategyID, scenarioID, entryEventType string) (*domain.StrategyAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := aggregateKey(strategyID, scenarioID, entryEventType)
	a, exists := s.data[key]
	if !exists {
		return nil, storage.ErrNotFound
	}

	aggCopy := *a
	return &aggCopy, nil
}

// GetByStrategy retrieves all aggregates for a strategy.
func (s *StrategyAggregateStore) GetByStrategy(_ context.Context, strategyID string) ([]*domain.StrategyAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.StrategyAggregate
	for _, a := range s.data {
		if a.StrategyID == strategyID {
			aggCopy := *a
			result = append(result, &aggCopy)
		}
	}

	// Sort by scenario, then entry event type
	sort.Slice(result, func(i, j int) bool {
		if result[i].ScenarioID != result[j].ScenarioID {
			return result[i].ScenarioID < result[j].ScenarioID
		}
		return result[i].EntryEventType < result[j].EntryEventType
	})

	return result, nil
}

// GetAll retrieves all aggregates.
func (s *StrategyAggregateStore) GetAll(_ context.Context) ([]*domain.StrategyAggregate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.StrategyAggregate
	for _, a := range s.data {
		aggCopy := *a
		result = append(result, &aggCopy)
	}

	// Sort by strategy, scenario, entry event type
	sort.Slice(result, func(i, j int) bool {
		if result[i].StrategyID != result[j].StrategyID {
			return result[i].StrategyID < result[j].StrategyID
		}
		if result[i].ScenarioID != result[j].ScenarioID {
			return result[i].ScenarioID < result[j].ScenarioID
		}
		return result[i].EntryEventType < result[j].EntryEventType
	})

	return result, nil
}

var _ storage.StrategyAggregateStore = (*StrategyAggregateStore)(nil)
