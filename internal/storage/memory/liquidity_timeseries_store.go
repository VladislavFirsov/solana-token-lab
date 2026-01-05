package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LiquidityTimeseriesStore is an in-memory implementation of storage.LiquidityTimeseriesStore.
type LiquidityTimeseriesStore struct {
	mu   sync.RWMutex
	data map[string]*domain.LiquidityTimeseriesPoint // keyed by (candidate_id, timestamp_ms)
}

// NewLiquidityTimeseriesStore creates a new in-memory liquidity timeseries store.
func NewLiquidityTimeseriesStore() *LiquidityTimeseriesStore {
	return &LiquidityTimeseriesStore{
		data: make(map[string]*domain.LiquidityTimeseriesPoint),
	}
}

// liquidityTsKey generates a unique key for a liquidity point.
func liquidityTsKey(candidateID string, timestampMs int64) string {
	return fmt.Sprintf("%s|%d", candidateID, timestampMs)
}

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *LiquidityTimeseriesStore) InsertBulk(_ context.Context, points []*domain.LiquidityTimeseriesPoint) error {
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
		key := liquidityTsKey(p.CandidateID, p.TimestampMs)

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
		key := liquidityTsKey(p.CandidateID, p.TimestampMs)
		copy := *p
		s.data[key] = &copy
	}

	return nil
}

// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
func (s *LiquidityTimeseriesStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.LiquidityTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.LiquidityTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID {
			copy := *p
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *LiquidityTimeseriesStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.LiquidityTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.LiquidityTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID && p.TimestampMs >= start && p.TimestampMs <= end {
			copy := *p
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

// GetGlobalTimeRange returns min and max timestamps across all data.
func (s *LiquidityTimeseriesStore) GetGlobalTimeRange(_ context.Context) (minTs, maxTs int64, err error) {
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

var _ storage.LiquidityTimeseriesStore = (*LiquidityTimeseriesStore)(nil)
