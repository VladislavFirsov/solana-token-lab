package clickhouse

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// PriceTimeseriesStore implements storage.PriceTimeseriesStore using ClickHouse.
type PriceTimeseriesStore struct {
	conn *Conn
}

// NewPriceTimeseriesStore creates a new PriceTimeseriesStore.
func NewPriceTimeseriesStore(conn *Conn) *PriceTimeseriesStore {
	return &PriceTimeseriesStore{conn: conn}
}

// Compile-time interface check.
var _ storage.PriceTimeseriesStore = (*PriceTimeseriesStore)(nil)

// InsertBulk adds multiple points. Fails entire batch on duplicate (candidate_id, timestamp_ms).
func (s *PriceTimeseriesStore) InsertBulk(ctx context.Context, points []*domain.PriceTimeseriesPoint) error {
	if len(points) == 0 {
		return nil
	}

	// Check for intra-batch duplicates
	type key struct {
		candidateID string
		timestampMs int64
	}
	seen := make(map[key]struct{})
	for _, p := range points {
		k := key{p.CandidateID, p.TimestampMs}
		if _, exists := seen[k]; exists {
			return storage.ErrDuplicateKey
		}
		seen[k] = struct{}{}
	}

	// Check for duplicates against existing DB rows
	for _, p := range points {
		exists, err := s.exists(ctx, p.CandidateID, p.TimestampMs)
		if err != nil {
			return fmt.Errorf("check exists: %w", err)
		}
		if exists {
			return storage.ErrDuplicateKey
		}
	}

	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO price_timeseries (
			candidate_id, timestamp_ms, slot, price, volume, swap_count
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, p := range points {
		err = batch.Append(
			p.CandidateID, uint64(p.TimestampMs), uint64(p.Slot),
			p.Price, p.Volume, uint32(p.SwapCount),
		)
		if err != nil {
			return fmt.Errorf("append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
func (s *PriceTimeseriesStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.PriceTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, slot, price, volume, swap_count
		FROM price_timeseries
		WHERE candidate_id = ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("query by candidate id: %w", err)
	}
	defer rows.Close()

	return scanPriceTimeseries(rows)
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *PriceTimeseriesStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.PriceTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, slot, price, volume, swap_count
		FROM price_timeseries
		WHERE candidate_id = ? AND timestamp_ms >= ? AND timestamp_ms <= ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID, uint64(start), uint64(end))
	if err != nil {
		return nil, fmt.Errorf("query by time range: %w", err)
	}
	defer rows.Close()

	return scanPriceTimeseries(rows)
}

// exists checks if a point with the given key exists.
func (s *PriceTimeseriesStore) exists(ctx context.Context, candidateID string, timestampMs int64) (bool, error) {
	query := `
		SELECT count(*) FROM price_timeseries
		WHERE candidate_id = ? AND timestamp_ms = ?
	`

	var count uint64
	err := s.conn.QueryRow(ctx, query, candidateID, uint64(timestampMs)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// scanPriceTimeseries scans multiple rows.
func scanPriceTimeseries(rows chRows) ([]*domain.PriceTimeseriesPoint, error) {
	var points []*domain.PriceTimeseriesPoint

	for rows.Next() {
		var p domain.PriceTimeseriesPoint
		var timestampMs, slot uint64
		var swapCount uint32

		err := rows.Scan(
			&p.CandidateID, &timestampMs, &slot,
			&p.Price, &p.Volume, &swapCount,
		)
		if err != nil {
			return nil, fmt.Errorf("scan price timeseries row: %w", err)
		}

		p.TimestampMs = int64(timestampMs)
		p.Slot = int64(slot)
		p.SwapCount = int(swapCount)
		points = append(points, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate price timeseries rows: %w", err)
	}

	return points, nil
}
