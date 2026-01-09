package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LiquidityEventStore implements storage.LiquidityEventStore using PostgreSQL.
type LiquidityEventStore struct {
	pool *Pool
}

// NewLiquidityEventStore creates a new LiquidityEventStore.
func NewLiquidityEventStore(pool *Pool) *LiquidityEventStore {
	return &LiquidityEventStore{pool: pool}
}

// Compile-time interface check.
var _ storage.LiquidityEventStore = (*LiquidityEventStore)(nil)

// Insert adds a new liquidity event. Returns ErrDuplicateKey if exists.
func (s *LiquidityEventStore) Insert(ctx context.Context, e *domain.LiquidityEvent) error {
	query := `
		INSERT INTO liquidity_events (
			candidate_id, tx_signature, event_index, slot, timestamp, event_type, amount_token, amount_quote, liquidity_after, pool, mint
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	// Convert empty strings to nil for nullable columns
	var candidateID, pool, mint interface{}
	if e.CandidateID != "" {
		candidateID = e.CandidateID
	}
	if e.Pool != "" {
		pool = e.Pool
	}
	if e.Mint != "" {
		mint = e.Mint
	}

	_, err := s.pool.Exec(ctx, query,
		candidateID,
		e.TxSignature,
		e.EventIndex,
		e.Slot,
		e.Timestamp,
		e.EventType,
		e.AmountToken,
		e.AmountQuote,
		e.LiquidityAfter,
		pool,
		mint,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert liquidity event: %w", err)
	}
	return nil
}

// InsertBulk adds multiple events atomically. Fails entire batch on any duplicate.
func (s *LiquidityEventStore) InsertBulk(ctx context.Context, events []*domain.LiquidityEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO liquidity_events (
			candidate_id, tx_signature, event_index, slot, timestamp, event_type, amount_token, amount_quote, liquidity_after, pool, mint
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	for _, e := range events {
		// Convert empty strings to nil for nullable columns
		var candidateID, pool, mint interface{}
		if e.CandidateID != "" {
			candidateID = e.CandidateID
		}
		if e.Pool != "" {
			pool = e.Pool
		}
		if e.Mint != "" {
			mint = e.Mint
		}

		_, err := tx.Exec(ctx, query,
			candidateID,
			e.TxSignature,
			e.EventIndex,
			e.Slot,
			e.Timestamp,
			e.EventType,
			e.AmountToken,
			e.AmountQuote,
			e.LiquidityAfter,
			pool,
			mint,
		)
		if err != nil {
			if isDuplicateKeyError(err) {
				return storage.ErrDuplicateKey
			}
			return fmt.Errorf("insert liquidity event in bulk: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// GetByCandidateID retrieves all events for a candidate, ordered by timestamp ASC.
func (s *LiquidityEventStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.LiquidityEvent, error) {
	query := `
		SELECT id, candidate_id, tx_signature, event_index, slot, timestamp, event_type, amount_token, amount_quote, liquidity_after, created_at, pool, mint
		FROM liquidity_events
		WHERE candidate_id = $1
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("get liquidity events by candidate id: %w", err)
	}
	defer rows.Close()

	return scanLiquidityEvents(rows)
}

// GetByTimeRange retrieves events for a candidate within [start, end] (inclusive).
func (s *LiquidityEventStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.LiquidityEvent, error) {
	query := `
		SELECT id, candidate_id, tx_signature, event_index, slot, timestamp, event_type, amount_token, amount_quote, liquidity_after, created_at, pool, mint
		FROM liquidity_events
		WHERE candidate_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, candidateID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get liquidity events by time range: %w", err)
	}
	defer rows.Close()

	return scanLiquidityEvents(rows)
}

// GetByMintTimeRange retrieves events by mint within [start, end) (end exclusive).
// Used for pre-candidate spike detection (ACTIVE_TOKEN discovery).
func (s *LiquidityEventStore) GetByMintTimeRange(ctx context.Context, mint string, start, end int64) ([]*domain.LiquidityEvent, error) {
	query := `
		SELECT id, candidate_id, tx_signature, event_index, slot, timestamp, event_type, amount_token, amount_quote, liquidity_after, created_at, pool, mint
		FROM liquidity_events
		WHERE mint = $1 AND timestamp >= $2 AND timestamp < $3
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, mint, start, end)
	if err != nil {
		return nil, fmt.Errorf("get liquidity events by mint time range: %w", err)
	}
	defer rows.Close()

	return scanLiquidityEvents(rows)
}

// scanLiquidityEvents scans multiple rows into a slice of LiquidityEvent.
func scanLiquidityEvents(rows pgx.Rows) ([]*domain.LiquidityEvent, error) {
	var events []*domain.LiquidityEvent

	for rows.Next() {
		var e domain.LiquidityEvent
		var pool, mint *string

		err := rows.Scan(
			&e.ID,
			&e.CandidateID,
			&e.TxSignature,
			&e.EventIndex,
			&e.Slot,
			&e.Timestamp,
			&e.EventType,
			&e.AmountToken,
			&e.AmountQuote,
			&e.LiquidityAfter,
			&e.CreatedAt,
			&pool,
			&mint,
		)
		if err != nil {
			return nil, fmt.Errorf("scan liquidity event row: %w", err)
		}
		if pool != nil {
			e.Pool = *pool
		}
		if mint != nil {
			e.Mint = *mint
		}

		events = append(events, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate liquidity event rows: %w", err)
	}

	return events, nil
}
