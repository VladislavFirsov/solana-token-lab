package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// PriceTimeseriesStore is an in-memory implementation of storage.PriceTimeseriesStore.
type PriceTimeseriesStore struct {
	mu   sync.RWMutex
	data map[string]*domain.PriceTimeseriesPoint // keyed by (candidate_id, timestamp_ms)
}

// NewPriceTimeseriesStore creates a new in-memory price timeseries store.
func NewPriceTimeseriesStore() *PriceTimeseriesStore {
	return &PriceTimeseriesStore{
		data: make(map[string]*domain.PriceTimeseriesPoint),
	}
}

// priceKey generates a unique key for a price point.
func priceKey(candidateID string, timestampMs int64) string {
	return fmt.Sprintf("%s|%d", candidateID, timestampMs)
}

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *PriceTimeseriesStore) InsertBulk(_ context.Context, points []*domain.PriceTimeseriesPoint) error {
	if len(points) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track keys in this batch to detect intra-batch duplicates
	batchKeys := make(map[string]struct{}, len(points))

	// First pass: check for duplicates (existing + intra-batch)
	for _, p := range points {
		if p == nil || p.CandidateID == "" {
			return storage.ErrInvalidInput
		}
		key := priceKey(p.CandidateID, p.TimestampMs)

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
	for _, p := range points {
		key := priceKey(p.CandidateID, p.TimestampMs)
		pointCopy := *p
		s.data[key] = &pointCopy
	}

	return nil
}

// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
func (s *PriceTimeseriesStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.PriceTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.PriceTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID {
			pointCopy := *p
			result = append(result, &pointCopy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *PriceTimeseriesStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.PriceTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.PriceTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID && p.TimestampMs >= start && p.TimestampMs <= end {
			pointCopy := *p
			result = append(result, &pointCopy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

// GetGlobalTimeRange returns min and max timestamps across all data.
func (s *PriceTimeseriesStore) GetGlobalTimeRange(_ context.Context) (minTs, maxTs int64, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.data) == 0 {
		return 0, 0, nil
	}

	first := true
	for _, p := range s.data {
		if first {
			minTs = p.TimestampMs
			maxTs = p.TimestampMs
			first = false
		} else {
			if p.TimestampMs < minTs {
				minTs = p.TimestampMs
			}
			if p.TimestampMs > maxTs {
				maxTs = p.TimestampMs
			}
		}
	}

	return minTs, maxTs, nil
}

var _ storage.PriceTimeseriesStore = (*PriceTimeseriesStore)(nil)
