package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LiquidityEventStore is an in-memory implementation of storage.LiquidityEventStore.
type LiquidityEventStore struct {
	mu   sync.RWMutex
	data map[string]*domain.LiquidityEvent // keyed by composite key
}

// NewLiquidityEventStore creates a new in-memory liquidity event store.
func NewLiquidityEventStore() *LiquidityEventStore {
	return &LiquidityEventStore{
		data: make(map[string]*domain.LiquidityEvent),
	}
}

// liquidityEventKey generates a unique key for a liquidity event.
func liquidityEventKey(candidateID, txSignature string, eventIndex int) string {
	return fmt.Sprintf("%s|%s|%d", candidateID, txSignature, eventIndex)
}

// Insert adds a new liquidity event. Returns ErrDuplicateKey if exists.
func (s *LiquidityEventStore) Insert(_ context.Context, e *domain.LiquidityEvent) error {
	if e == nil || e.CandidateID == "" {
		return storage.ErrInvalidInput
	}

	key := liquidityEventKey(e.CandidateID, e.TxSignature, e.EventIndex)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; exists {
		return storage.ErrDuplicateKey
	}

	eventCopy := *e
	s.data[key] = &eventCopy
	return nil
}

// InsertBulk adds multiple events atomically. Fails entire batch on any duplicate.
func (s *LiquidityEventStore) InsertBulk(_ context.Context, events []*domain.LiquidityEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track keys in this batch to detect intra-batch duplicates
	batchKeys := make(map[string]struct{}, len(events))

	// First pass: check for duplicates (existing + intra-batch)
	for _, e := range events {
		if e == nil || e.CandidateID == "" {
			return storage.ErrInvalidInput
		}
		key := liquidityEventKey(e.CandidateID, e.TxSignature, e.EventIndex)

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
	for _, e := range events {
		key := liquidityEventKey(e.CandidateID, e.TxSignature, e.EventIndex)
		eventCopy := *e
		s.data[key] = &eventCopy
	}

	return nil
}

// GetByCandidateID retrieves all events for a candidate, ordered by timestamp ASC.
func (s *LiquidityEventStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.LiquidityEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.LiquidityEvent
	for _, e := range s.data {
		if e.CandidateID == candidateID {
			eventCopy := *e
			result = append(result, &eventCopy)
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

// GetByTimeRange retrieves events for a candidate within [start, end] (inclusive).
func (s *LiquidityEventStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.LiquidityEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.LiquidityEvent
	for _, e := range s.data {
		if e.CandidateID == candidateID && e.Timestamp >= start && e.Timestamp <= end {
			eventCopy := *e
			result = append(result, &eventCopy)
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

var _ storage.LiquidityEventStore = (*LiquidityEventStore)(nil)
