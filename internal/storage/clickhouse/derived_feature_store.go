package clickhouse

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// DerivedFeatureStore implements storage.DerivedFeatureStore using ClickHouse.
type DerivedFeatureStore struct {
	conn *Conn
}

// NewDerivedFeatureStore creates a new DerivedFeatureStore.
func NewDerivedFeatureStore(conn *Conn) *DerivedFeatureStore {
	return &DerivedFeatureStore{conn: conn}
}

// Compile-time interface check.
var _ storage.DerivedFeatureStore = (*DerivedFeatureStore)(nil)

// InsertBulk adds multiple points. Fails entire batch on duplicate.
func (s *DerivedFeatureStore) InsertBulk(ctx context.Context, points []*domain.DerivedFeaturePoint) error {
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
		INSERT INTO derived_features (
			candidate_id, timestamp_ms,
			price_delta, price_velocity, price_acceleration,
			liquidity_delta, liquidity_velocity,
			token_lifetime_ms, last_swap_interval_ms, last_liq_event_interval_ms
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, p := range points {
		// Pass nil values directly for Nullable columns
		err = batch.Append(
			p.CandidateID, uint64(p.TimestampMs),
			p.PriceDelta, p.PriceVelocity, p.PriceAcceleration,
			p.LiquidityDelta, p.LiquidityVelocity,
			uint64(p.TokenLifetimeMs), toNullableUint64(p.LastSwapIntervalMs), toNullableUint64(p.LastLiqEventIntervalMs),
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
func (s *DerivedFeatureStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.DerivedFeaturePoint, error) {
	query := `
		SELECT
			candidate_id, timestamp_ms,
			price_delta, price_velocity, price_acceleration,
			liquidity_delta, liquidity_velocity,
			token_lifetime_ms, last_swap_interval_ms, last_liq_event_interval_ms
		FROM derived_features
		WHERE candidate_id = ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("query by candidate id: %w", err)
	}
	defer rows.Close()

	return scanDerivedFeatures(rows)
}

// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
func (s *DerivedFeatureStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.DerivedFeaturePoint, error) {
	query := `
		SELECT
			candidate_id, timestamp_ms,
			price_delta, price_velocity, price_acceleration,
			liquidity_delta, liquidity_velocity,
			token_lifetime_ms, last_swap_interval_ms, last_liq_event_interval_ms
		FROM derived_features
		WHERE candidate_id = ? AND timestamp_ms >= ? AND timestamp_ms <= ?
		ORDER BY timestamp_ms ASC
	`

	rows, err := s.conn.Query(ctx, query, candidateID, uint64(start), uint64(end))
	if err != nil {
		return nil, fmt.Errorf("query by time range: %w", err)
	}
	defer rows.Close()

	return scanDerivedFeatures(rows)
}

// exists checks if a point with the given key exists.
func (s *DerivedFeatureStore) exists(ctx context.Context, candidateID string, timestampMs int64) (bool, error) {
	query := `
		SELECT count(*) FROM derived_features
		WHERE candidate_id = ? AND timestamp_ms = ?
	`

	var count uint64
	err := s.conn.QueryRow(ctx, query, candidateID, uint64(timestampMs)).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// toNullableUint64 converts *int64 to *uint64 for ClickHouse Nullable(UInt64).
func toNullableUint64(v *int64) *uint64 {
	if v == nil {
		return nil
	}
	u := uint64(*v)
	return &u
}

// scanDerivedFeatures scans multiple rows.
func scanDerivedFeatures(rows chRows) ([]*domain.DerivedFeaturePoint, error) {
	var points []*domain.DerivedFeaturePoint

	for rows.Next() {
		var p domain.DerivedFeaturePoint
		var timestampMs, tokenLifetimeMs uint64
		var lastSwapIntervalMs, lastLiqEventIntervalMs *uint64

		err := rows.Scan(
			&p.CandidateID, &timestampMs,
			&p.PriceDelta, &p.PriceVelocity, &p.PriceAcceleration,
			&p.LiquidityDelta, &p.LiquidityVelocity,
			&tokenLifetimeMs, &lastSwapIntervalMs, &lastLiqEventIntervalMs,
		)
		if err != nil {
			return nil, fmt.Errorf("scan derived features row: %w", err)
		}

		p.TimestampMs = int64(timestampMs)
		p.TokenLifetimeMs = int64(tokenLifetimeMs)

		// Convert Nullable(UInt64) to *int64
		if lastSwapIntervalMs != nil {
			v := int64(*lastSwapIntervalMs)
			p.LastSwapIntervalMs = &v
		}
		if lastLiqEventIntervalMs != nil {
			v := int64(*lastLiqEventIntervalMs)
			p.LastLiqEventIntervalMs = &v
		}

		points = append(points, &p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate derived features rows: %w", err)
	}

	return points, nil
}
