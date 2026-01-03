package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// VolumeTimeseriesStore is an in-memory implementation of storage.VolumeTimeseriesStore.
type VolumeTimeseriesStore struct {
	mu   sync.RWMutex
	data map[string]*domain.VolumeTimeseriesPoint // keyed by (candidate_id, timestamp_ms, interval)
}

// NewVolumeTimeseriesStore creates a new in-memory volume timeseries store.
func NewVolumeTimeseriesStore() *VolumeTimeseriesStore {
	return &VolumeTimeseriesStore{
		data: make(map[string]*domain.VolumeTimeseriesPoint),
	}
}

// volumeKey generates a unique key for a volume point.
func volumeKey(candidateID string, timestampMs int64, intervalSeconds int) string {
	return fmt.Sprintf("%s|%d|%d", candidateID, timestampMs, intervalSeconds)
}

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *VolumeTimeseriesStore) InsertBulk(_ context.Context, points []*domain.VolumeTimeseriesPoint) error {
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
		key := volumeKey(p.CandidateID, p.TimestampMs, p.IntervalSeconds)

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
		key := volumeKey(p.CandidateID, p.TimestampMs, p.IntervalSeconds)
		copy := *p
		s.data[key] = &copy
	}

	return nil
}

// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
func (s *VolumeTimeseriesStore) GetByCandidateID(_ context.Context, candidateID string) ([]*domain.VolumeTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.VolumeTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID {
			copy := *p
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TimestampMs != result[j].TimestampMs {
			return result[i].TimestampMs < result[j].TimestampMs
		}
		return result[i].IntervalSeconds < result[j].IntervalSeconds
	})

	return result, nil
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *VolumeTimeseriesStore) GetByTimeRange(_ context.Context, candidateID string, start, end int64) ([]*domain.VolumeTimeseriesPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.VolumeTimeseriesPoint
	for _, p := range s.data {
		if p.CandidateID == candidateID && p.TimestampMs >= start && p.TimestampMs <= end {
			copy := *p
			result = append(result, &copy)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TimestampMs != result[j].TimestampMs {
			return result[i].TimestampMs < result[j].TimestampMs
		}
		return result[i].IntervalSeconds < result[j].IntervalSeconds
	})

	return result, nil
}

var _ storage.VolumeTimeseriesStore = (*VolumeTimeseriesStore)(nil)
