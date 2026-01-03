package memory

import (
	"context"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// swapEventKey is the composite key for swap event deduplication.
type swapEventKey struct {
	Mint        string
	TxSignature string
	EventIndex  int
}

// SwapEventStore is an in-memory implementation of storage.SwapEventStore.
type SwapEventStore struct {
	mu   sync.RWMutex
	data []*domain.SwapEvent
	keys map[swapEventKey]bool
}

// NewSwapEventStore creates a new in-memory swap event store.
func NewSwapEventStore() *SwapEventStore {
	return &SwapEventStore{
		data: make([]*domain.SwapEvent, 0),
		keys: make(map[swapEventKey]bool),
	}
}

// Insert adds a new swap event. Returns ErrDuplicateKey if (mint, tx_signature, event_index) exists.
func (s *SwapEventStore) Insert(_ context.Context, e *domain.SwapEvent) error {
	if e == nil {
		return storage.ErrInvalidInput
	}

	key := swapEventKey{
		Mint:        e.Mint,
		TxSignature: e.TxSignature,
		EventIndex:  e.EventIndex,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.keys[key] {
		return storage.ErrDuplicateKey
	}

	// Store a copy
	copy := *e
	s.data = append(s.data, &copy)
	s.keys[key] = true

	return nil
}

// InsertBulk adds multiple swap events atomically. Fails entire batch on any duplicate.
func (s *SwapEventStore) InsertBulk(_ context.Context, events []*domain.SwapEvent) error {
	if len(events) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicates (both existing and intra-batch)
	batchKeys := make(map[swapEventKey]bool)
	for _, e := range events {
		if e == nil {
			return storage.ErrInvalidInput
		}

		key := swapEventKey{
			Mint:        e.Mint,
			TxSignature: e.TxSignature,
			EventIndex:  e.EventIndex,
		}

		if s.keys[key] || batchKeys[key] {
			return storage.ErrDuplicateKey
		}
		batchKeys[key] = true
	}

	// Insert all
	for _, e := range events {
		copy := *e
		s.data = append(s.data, &copy)

		key := swapEventKey{
			Mint:        e.Mint,
			TxSignature: e.TxSignature,
			EventIndex:  e.EventIndex,
		}
		s.keys[key] = true
	}

	return nil
}

// GetByTimeRange retrieves swap events within [start, end) (inclusive start, exclusive end).
func (s *SwapEventStore) GetByTimeRange(_ context.Context, start, end int64) ([]*domain.SwapEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.SwapEvent
	for _, e := range s.data {
		if e.Timestamp >= start && e.Timestamp < end {
			copy := *e
			result = append(result, &copy)
		}
	}

	// Sort by (slot, tx_signature, event_index)
	sortSwapEvents(result)

	return result, nil
}

// GetByMintTimeRange retrieves swap events for a mint within [start, end).
func (s *SwapEventStore) GetByMintTimeRange(_ context.Context, mint string, start, end int64) ([]*domain.SwapEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.SwapEvent
	for _, e := range s.data {
		if e.Mint == mint && e.Timestamp >= start && e.Timestamp < end {
			copy := *e
			result = append(result, &copy)
		}
	}

	// Sort by (slot, tx_signature, event_index)
	sortSwapEvents(result)

	return result, nil
}

// GetDistinctMintsByTimeRange returns all distinct mints with swap events in [start, end).
func (s *SwapEventStore) GetDistinctMintsByTimeRange(_ context.Context, start, end int64) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mints := make(map[string]bool)
	for _, e := range s.data {
		if e.Timestamp >= start && e.Timestamp < end {
			mints[e.Mint] = true
		}
	}

	result := make([]string, 0, len(mints))
	for mint := range mints {
		result = append(result, mint)
	}

	// Sort for deterministic ordering
	sort.Strings(result)

	return result, nil
}

// sortSwapEvents sorts events by (slot, tx_signature, event_index).
func sortSwapEvents(events []*domain.SwapEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Slot != events[j].Slot {
			return events[i].Slot < events[j].Slot
		}
		if events[i].TxSignature != events[j].TxSignature {
			return events[i].TxSignature < events[j].TxSignature
		}
		return events[i].EventIndex < events[j].EventIndex
	})
}

// Verify interface compliance at compile time.
var _ storage.SwapEventStore = (*SwapEventStore)(nil)
