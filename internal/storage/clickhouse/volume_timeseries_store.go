package clickhouse

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// VolumeTimeseriesStore implements storage.VolumeTimeseriesStore using ClickHouse.
type VolumeTimeseriesStore struct {
	conn *Conn
}

// NewVolumeTimeseriesStore creates a new VolumeTimeseriesStore.
func NewVolumeTimeseriesStore(conn *Conn) *VolumeTimeseriesStore {
	return &VolumeTimeseriesStore{conn: conn}
}

// Compile-time interface check.
var _ storage.VolumeTimeseriesStore = (*VolumeTimeseriesStore)(nil)

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *VolumeTimeseriesStore) InsertBulk(ctx context.Context, points []*domain.VolumeTimeseriesPoint) error {
	if len(points) == 0 {
		return nil
	}

	// Check for intra-batch duplicates
	type key struct {
		candidateID     string
		timestampMs     int64
		intervalSeconds int
	}
	seen := make(map[key]struct{})
	for _, p := range points {
		k := key{p.CandidateID, p.TimestampMs, p.IntervalSeconds}
		if _, exists := seen[k]; exists {
			return storage.ErrDuplicateKey
		}
		seen[k] = struct{}{}
	}

	// Check for duplicates against existing DB rows
	for _, p := range points {
		exists, err := s.exists(ctx, p.CandidateID, p.TimestampMs, p.IntervalSeconds)
		if err != nil {
			return fmt.Errorf("check exists: %w", err)
		}
		if exists {
			return storage.ErrDuplicateKey
		}
	}

	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO volume_timeseries (
			candidate_id, timestamp_ms, interval_seconds, volume, swap_count, buy_volume, sell_volume
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, p := range points {
		err = batch.Append(
			p.CandidateID, uint64(p.TimestampMs), uint32(p.IntervalSeconds),
			p.Volume, uint32(p.SwapCount), p.BuyVolume, p.SellVolume,
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
func (s *VolumeTimeseriesStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.VolumeTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, interval_seconds, volume, swap_count, buy_volume, sell_volume
		FROM volume_timeseries
		WHERE candidate_id = ?
		ORDER BY interval_seconds ASC, timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("query by candidate id: %w", err)
	}
	defer rows.Close()

	return scanVolumeTimeseries(rows)
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *VolumeTimeseriesStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.VolumeTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, interval_seconds, volume, swap_count, buy_volume, sell_volume
		FROM volume_timeseries
		WHERE candidate_id = ? AND timestamp_ms >= ? AND timestamp_ms <= ?
		ORDER BY interval_seconds ASC, timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID, uint64(start), uint64(end))
	if err != nil {
		return nil, fmt.Errorf("query by time range: %w", err)
	}
	defer rows.Close()

	return scanVolumeTimeseries(rows)
}

// exists checks if a point with the given key exists.
func (s *VolumeTimeseriesStore) exists(ctx context.Context, candidateID string, timestampMs int64, intervalSeconds int) (bool, error) {
	query := `
		SELECT count(*) FROM volume_timeseries
		WHERE candidate_id = ? AND timestamp_ms = ? AND interval_seconds = ?
	`

	var count uint64
	err := s.conn.QueryRow(ctx, query, candidateID, uint64(timestampMs), uint32(intervalSeconds)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// scanVolumeTimeseries scans multiple rows.
func scanVolumeTimeseries(rows chRows) ([]*domain.VolumeTimeseriesPoint, error) {
	var points []*domain.VolumeTimeseriesPoint

	for rows.Next() {
		var p domain.VolumeTimeseriesPoint
		var timestampMs uint64
		var intervalSeconds, swapCount uint32

		err := rows.Scan(
			&p.CandidateID, &timestampMs, &intervalSeconds,
			&p.Volume, &swapCount, &p.BuyVolume, &p.SellVolume,
		)
		if err != nil {
			return nil, fmt.Errorf("scan volume timeseries row: %w", err)
		}

		p.TimestampMs = int64(timestampMs)
		p.IntervalSeconds = int(intervalSeconds)
		p.SwapCount = int(swapCount)
		points = append(points, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate volume timeseries rows: %w", err)
	}

	return points, nil
}
