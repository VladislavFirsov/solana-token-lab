package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// SwapEventStore implements storage.SwapEventStore using PostgreSQL.
type SwapEventStore struct {
	pool *Pool
}

// NewSwapEventStore creates a new SwapEventStore.
func NewSwapEventStore(pool *Pool) *SwapEventStore {
	return &SwapEventStore{pool: pool}
}

// Compile-time interface check.
var _ storage.SwapEventStore = (*SwapEventStore)(nil)

// Insert adds a new swap event. Returns ErrDuplicateKey if (mint, tx_signature, event_index) exists.
func (s *SwapEventStore) Insert(ctx context.Context, e *domain.SwapEvent) error {
	query := `
		INSERT INTO swap_events (
			mint, pool, tx_signature, event_index, slot, timestamp, amount_out
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.pool.Exec(ctx, query,
		e.Mint,
		e.Pool,
		e.TxSignature,
		e.EventIndex,
		e.Slot,
		e.Timestamp,
		e.AmountOut,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert swap event: %w", err)
	}
	return nil
}

// InsertBulk adds multiple swap events atomically. Fails entire batch on any duplicate.
func (s *SwapEventStore) InsertBulk(ctx context.Context, events []*domain.SwapEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO swap_events (
			mint, pool, tx_signature, event_index, slot, timestamp, amount_out
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	for _, e := range events {
		_, err := tx.Exec(ctx, query,
			e.Mint,
			e.Pool,
			e.TxSignature,
			e.EventIndex,
			e.Slot,
			e.Timestamp,
			e.AmountOut,
		)
		if err != nil {
			if isDuplicateKeyError(err) {
				return storage.ErrDuplicateKey
			}
			return fmt.Errorf("insert swap event in bulk: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// GetByTimeRange retrieves swap events within [start, end) (inclusive start, exclusive end).
func (s *SwapEventStore) GetByTimeRange(ctx context.Context, start, end int64) ([]*domain.SwapEvent, error) {
	query := `
		SELECT mint, pool, tx_signature, event_index, slot, timestamp, amount_out
		FROM swap_events
		WHERE timestamp >= $1 AND timestamp < $2
		ORDER BY timestamp ASC, mint ASC, tx_signature ASC, event_index ASC
	`

	rows, err := s.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get swap events by time range: %w", err)
	}
	defer rows.Close()

	return scanSwapEvents(rows)
}

// GetByMintTimeRange retrieves swap events for a mint within [start, end).
func (s *SwapEventStore) GetByMintTimeRange(ctx context.Context, mint string, start, end int64) ([]*domain.SwapEvent, error) {
	query := `
		SELECT mint, pool, tx_signature, event_index, slot, timestamp, amount_out
		FROM swap_events
		WHERE mint = $1 AND timestamp >= $2 AND timestamp < $3
		ORDER BY timestamp ASC, tx_signature ASC, event_index ASC
	`

	rows, err := s.pool.Query(ctx, query, mint, start, end)
	if err != nil {
		return nil, fmt.Errorf("get swap events by mint/time range: %w", err)
	}
	defer rows.Close()

	return scanSwapEvents(rows)
}

// GetDistinctMintsByTimeRange returns all distinct mints with swap events in [start, end).
func (s *SwapEventStore) GetDistinctMintsByTimeRange(ctx context.Context, start, end int64) ([]string, error) {
	query := `
		SELECT DISTINCT mint
		FROM swap_events
		WHERE timestamp >= $1 AND timestamp < $2
		ORDER BY mint ASC
	`

	rows, err := s.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get distinct mints by time range: %w", err)
	}
	defer rows.Close()

	var mints []string
	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return nil, fmt.Errorf("scan mint: %w", err)
		}
		mints = append(mints, mint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mint rows: %w", err)
	}

	return mints, nil
}

// scanSwapEvents scans multiple rows into a slice of SwapEvent.
func scanSwapEvents(rows pgx.Rows) ([]*domain.SwapEvent, error) {
	var events []*domain.SwapEvent

	for rows.Next() {
		var e domain.SwapEvent

		err := rows.Scan(
			&e.Mint,
			&e.Pool,
			&e.TxSignature,
			&e.EventIndex,
			&e.Slot,
			&e.Timestamp,
			&e.AmountOut,
		)
		if err != nil {
			return nil, fmt.Errorf("scan swap event row: %w", err)
		}

		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate swap event rows: %w", err)
	}

	return events, nil
}
