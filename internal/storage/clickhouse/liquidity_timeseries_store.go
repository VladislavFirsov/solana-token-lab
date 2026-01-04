package clickhouse

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LiquidityTimeseriesStore implements storage.LiquidityTimeseriesStore using ClickHouse.
type LiquidityTimeseriesStore struct {
	conn *Conn
}

// NewLiquidityTimeseriesStore creates a new LiquidityTimeseriesStore.
func NewLiquidityTimeseriesStore(conn *Conn) *LiquidityTimeseriesStore {
	return &LiquidityTimeseriesStore{conn: conn}
}

// Compile-time interface check.
var _ storage.LiquidityTimeseriesStore = (*LiquidityTimeseriesStore)(nil)

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *LiquidityTimeseriesStore) InsertBulk(ctx context.Context, points []*domain.LiquidityTimeseriesPoint) error {
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
		INSERT INTO liquidity_timeseries (
			candidate_id, timestamp_ms, slot, liquidity, liquidity_token, liquidity_quote
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, p := range points {
		err = batch.Append(
			p.CandidateID, uint64(p.TimestampMs), uint64(p.Slot),
			p.Liquidity, p.LiquidityToken, p.LiquidityQuote,
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
func (s *LiquidityTimeseriesStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.LiquidityTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, slot, liquidity, liquidity_token, liquidity_quote
		FROM liquidity_timeseries
		WHERE candidate_id = ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("query by candidate id: %w", err)
	}
	defer rows.Close()

	return scanLiquidityTimeseries(rows)
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *LiquidityTimeseriesStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.LiquidityTimeseriesPoint, error) {
	query := `
		SELECT candidate_id, timestamp_ms, slot, liquidity, liquidity_token, liquidity_quote
		FROM liquidity_timeseries
		WHERE candidate_id = ? AND timestamp_ms >= ? AND timestamp_ms <= ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID, uint64(start), uint64(end))
	if err != nil {
		return nil, fmt.Errorf("query by time range: %w", err)
	}
	defer rows.Close()

	return scanLiquidityTimeseries(rows)
}

// exists checks if a point with the given key exists.
func (s *LiquidityTimeseriesStore) exists(ctx context.Context, candidateID string, timestampMs int64) (bool, error) {
	query := `
		SELECT count(*) FROM liquidity_timeseries
		WHERE candidate_id = ? AND timestamp_ms = ?
	`

	var count uint64
	err := s.conn.QueryRow(ctx, query, candidateID, uint64(timestampMs)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// scanLiquidityTimeseries scans multiple rows.
func scanLiquidityTimeseries(rows chRows) ([]*domain.LiquidityTimeseriesPoint, error) {
	var points []*domain.LiquidityTimeseriesPoint

	for rows.Next() {
		var p domain.LiquidityTimeseriesPoint
		var timestampMs, slot uint64

		err := rows.Scan(
			&p.CandidateID, &timestampMs, &slot,
			&p.Liquidity, &p.LiquidityToken, &p.LiquidityQuote,
		)
		if err != nil {
			return nil, fmt.Errorf("scan liquidity timeseries row: %w", err)
		}

		p.TimestampMs = int64(timestampMs)
		p.Slot = int64(slot)
		points = append(points, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate liquidity timeseries rows: %w", err)
	}

	return points, nil
}
