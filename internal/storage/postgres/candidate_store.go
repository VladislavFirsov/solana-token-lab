package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// CandidateStore implements storage.CandidateStore using PostgreSQL.
type CandidateStore struct {
	pool *Pool
}

// NewCandidateStore creates a new CandidateStore.
func NewCandidateStore(pool *Pool) *CandidateStore {
	return &CandidateStore{pool: pool}
}

// Compile-time interface check.
var _ storage.CandidateStore = (*CandidateStore)(nil)

// Insert adds a new candidate. Returns ErrDuplicateKey if candidate_id exists.
func (s *CandidateStore) Insert(ctx context.Context, c *domain.TokenCandidate) error {
	query := `
		INSERT INTO token_candidates (
			candidate_id, source, mint, pool, tx_signature, event_index, slot, discovered_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.pool.Exec(ctx, query,
		c.CandidateID,
		string(c.Source),
		c.Mint,
		c.Pool,
		c.TxSignature,
		c.EventIndex,
		c.Slot,
		c.DiscoveredAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert candidate: %w", err)
	}
	return nil
}

// GetByID retrieves a candidate by its ID. Returns ErrNotFound if not exists.
func (s *CandidateStore) GetByID(ctx context.Context, candidateID string) (*domain.TokenCandidate, error) {
	query := `
		SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, discovered_at, created_at
		FROM token_candidates
		WHERE candidate_id = $1
	`

	row := s.pool.QueryRow(ctx, query, candidateID)
	c, err := scanCandidate(row)
	if err != nil {
		if isNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get candidate by id: %w", err)
	}
	return c, nil
}

// GetByMint retrieves all candidates for a given mint address.
func (s *CandidateStore) GetByMint(ctx context.Context, mint string) ([]*domain.TokenCandidate, error) {
	query := `
		SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, discovered_at, created_at
		FROM token_candidates
		WHERE mint = $1
		ORDER BY discovered_at ASC, candidate_id ASC
	`

	rows, err := s.pool.Query(ctx, query, mint)
	if err != nil {
		return nil, fmt.Errorf("get candidates by mint: %w", err)
	}
	defer rows.Close()

	return scanCandidates(rows)
}

// GetByTimeRange retrieves candidates discovered within [start, end] (inclusive).
func (s *CandidateStore) GetByTimeRange(ctx context.Context, start, end int64) ([]*domain.TokenCandidate, error) {
	query := `
		SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, discovered_at, created_at
		FROM token_candidates
		WHERE discovered_at >= $1 AND discovered_at <= $2
		ORDER BY discovered_at ASC, candidate_id ASC
	`

	rows, err := s.pool.Query(ctx, query, start, end)
	if err != nil {
		return nil, fmt.Errorf("get candidates by time range: %w", err)
	}
	defer rows.Close()

	return scanCandidates(rows)
}

// GetBySource retrieves all candidates of a given source type.
func (s *CandidateStore) GetBySource(ctx context.Context, source domain.Source) ([]*domain.TokenCandidate, error) {
	query := `
		SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, discovered_at, created_at
		FROM token_candidates
		WHERE source = $1
		ORDER BY discovered_at ASC, candidate_id ASC
	`

	rows, err := s.pool.Query(ctx, query, string(source))
	if err != nil {
		return nil, fmt.Errorf("get candidates by source: %w", err)
	}
	defer rows.Close()

	return scanCandidates(rows)
}

// scanCandidate scans a single row into a TokenCandidate.
func scanCandidate(row pgx.Row) (*domain.TokenCandidate, error) {
	var c domain.TokenCandidate
	var sourceStr string

	err := row.Scan(
		&c.CandidateID,
		&sourceStr,
		&c.Mint,
		&c.Pool,
		&c.TxSignature,
		&c.EventIndex,
		&c.Slot,
		&c.DiscoveredAt,
		&c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	c.Source = domain.Source(sourceStr)
	return &c, nil
}

// scanCandidates scans multiple rows into a slice of TokenCandidate.
func scanCandidates(rows pgx.Rows) ([]*domain.TokenCandidate, error) {
	var candidates []*domain.TokenCandidate

	for rows.Next() {
		var c domain.TokenCandidate
		var sourceStr string

		err := rows.Scan(
			&c.CandidateID,
			&sourceStr,
			&c.Mint,
			&c.Pool,
			&c.TxSignature,
			&c.EventIndex,
			&c.Slot,
			&c.DiscoveredAt,
			&c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan candidate row: %w", err)
		}

		c.Source = domain.Source(sourceStr)
		candidates = append(candidates, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candidate rows: %w", err)
	}

	return candidates, nil
}
