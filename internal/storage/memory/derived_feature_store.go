package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// DerivedFeatureStore is an in-memory implementation of storage.DerivedFeatureStore.
type DerivedFeatureStore struct {
	mu   sync.RWMutex
	data map[string]*domain.DerivedFeaturePoint // keyed by (candidate_id, timestamp_ms)
}

// NewDerivedFeatureStore creates a new in-memory derived feature store.
func NewDerivedFeatureStore() *DerivedFeatureStore {
	return &DerivedFeatureStore{
		data: make(map[string]*domain.DerivedFeaturePoint),
	}
}

// derivedFeatureKey generates a unique key for a feature point.
func derivedFeatureKey(candidateID string, timestampMs int64) string {
	return fmt.Sprintf("%s|%d", candidateID, timestampMs)
}

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *DerivedFeatureStore) InsertBulk(_ context.Context, points []*domain.DerivedFeaturePoint) error {
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
		key := derivedFeatureKey(p.CandidateID, p.TimestampMs)

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
		key := derivedFeatureKey(p.CandidateID, p.TimestampMs)
		featureCopy := *p
		s.data[key] = &featureCopy
	}

	return nil
}

// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
func (s *DerivedFeatureStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.DerivedFeaturePoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.DerivedFeaturePoint
	for _, p := range s.data {
		if p.CandidateID == candidateID {
			featureCopy := *p
			result = append(result, &featureCopy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *DerivedFeatureStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.DerivedFeaturePoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.DerivedFeaturePoint
	for _, p := range s.data {
		if p.CandidateID == candidateID && p.TimestampMs >= start && p.TimestampMs <= end {
			featureCopy := *p
			result = append(result, &featureCopy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TimestampMs < result[j].TimestampMs
	})

	return result, nil
}

var _ storage.DerivedFeatureStore = (*DerivedFeatureStore)(nil)
