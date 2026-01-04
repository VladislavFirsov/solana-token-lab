package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// SwapStore implements storage.SwapStore using PostgreSQL.
type SwapStore struct {
	pool *Pool
}

// NewSwapStore creates a new SwapStore.
func NewSwapStore(pool *Pool) *SwapStore {
	return &SwapStore{pool: pool}
}

// Compile-time interface check.
var _ storage.SwapStore = (*SwapStore)(nil)

// Insert adds a new swap. Returns ErrDuplicateKey if (candidate_id, tx_signature, event_index) exists.
func (s *SwapStore) Insert(ctx context.Context, swap *domain.Swap) error {
	query := `
		INSERT INTO swaps (
			candidate_id, tx_signature, event_index, slot, timestamp, side, amount_in, amount_out, price
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := s.pool.Exec(ctx, query,
		swap.CandidateID,
		swap.TxSignature,
		swap.EventIndex,
		swap.Slot,
		swap.Timestamp,
		swap.Side,
		swap.AmountIn,
		swap.AmountOut,
		swap.Price,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert swap: %w", err)
	}
	return nil
}

// InsertBulk adds multiple swaps atomically. Fails entire batch on any duplicate.
func (s *SwapStore) InsertBulk(ctx context.Context, swaps []*domain.Swap) error {
	if len(swaps) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO swaps (
			candidate_id, tx_signature, event_index, slot, timestamp, side, amount_in, amount_out, price
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	for _, swap := range swaps {
		_, err := tx.Exec(ctx, query,
			swap.CandidateID,
			swap.TxSignature,
			swap.EventIndex,
			swap.Slot,
			swap.Timestamp,
			swap.Side,
			swap.AmountIn,
			swap.AmountOut,
			swap.Price,
		)
		if err != nil {
			if isDuplicateKeyError(err) {
				return storage.ErrDuplicateKey
			}
			return fmt.Errorf("insert swap in bulk: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// GetByCandidateID retrieves all swaps for a candidate, ordered by timestamp ASC.
func (s *SwapStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.Swap, error) {
	query := `
		SELECT id, candidate_id, tx_signature, event_index, slot, timestamp, side, amount_in, amount_out, price, created_at
		FROM swaps
		WHERE candidate_id = $1
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("get swaps by candidate id: %w", err)
	}
	defer rows.Close()

	return scanSwaps(rows)
}

// GetByTimeRange retrieves swaps for a candidate within [start, end] (inclusive).
func (s *SwapStore) GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.Swap, error) {
	query := `
		SELECT id, candidate_id, tx_signature, event_index, slot, timestamp, side, amount_in, amount_out, price, created_at
		FROM swaps
		WHERE candidate_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp ASC, id ASC
	`

	rows, err := s.pool.Query(ctx, query, candidateID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get swaps by time range: %w", err)
	}
	defer rows.Close()

	return scanSwaps(rows)
}

// scanSwaps scans multiple rows into a slice of Swap.
func scanSwaps(rows pgx.Rows) ([]*domain.Swap, error) {
	var swaps []*domain.Swap

	for rows.Next() {
		var swap domain.Swap

		err := rows.Scan(
			&swap.ID,
			&swap.CandidateID,
			&swap.TxSignature,
			&swap.EventIndex,
			&swap.Slot,
			&swap.Timestamp,
			&swap.Side,
			&swap.AmountIn,
			&swap.AmountOut,
			&swap.Price,
			&swap.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan swap row: %w", err)
		}

		swaps = append(swaps, &swap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate swap rows: %w", err)
	}

	return swaps, nil
}
