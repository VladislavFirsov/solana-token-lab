package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// TokenMetadataStore implements storage.TokenMetadataStore using PostgreSQL.
type TokenMetadataStore struct {
	pool *Pool
}

// NewTokenMetadataStore creates a new TokenMetadataStore.
func NewTokenMetadataStore(pool *Pool) *TokenMetadataStore {
	return &TokenMetadataStore{pool: pool}
}

// Compile-time interface check.
var _ storage.TokenMetadataStore = (*TokenMetadataStore)(nil)

// Insert adds new metadata. Returns ErrDuplicateKey if candidate_id exists.
func (s *TokenMetadataStore) Insert(ctx context.Context, m *domain.TokenMetadata) error {
	query := `
		INSERT INTO token_metadata (
			candidate_id, mint, name, symbol, decimals, supply, fetched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.pool.Exec(ctx, query,
		m.CandidateID,
		m.Mint,
		m.Name,
		m.Symbol,
		m.Decimals,
		m.Supply,
		m.FetchedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert token metadata: %w", err)
	}
	return nil
}

// GetByID retrieves metadata by candidate ID. Returns ErrNotFound if not exists.
func (s *TokenMetadataStore) GetByID(ctx context.Context, candidateID string) (*domain.TokenMetadata, error) {
	query := `
		SELECT candidate_id, mint, name, symbol, decimals, supply, fetched_at, created_at
		FROM token_metadata
		WHERE candidate_id = $1
	`

	row := s.pool.QueryRow(ctx, query, candidateID)
	m, err := scanTokenMetadata(row)
	if err != nil {
		if isNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get token metadata by id: %w", err)
	}
	return m, nil
}

// GetByMint retrieves metadata by mint address. Returns ErrNotFound if not exists.
func (s *TokenMetadataStore) GetByMint(ctx context.Context, mint string) (*domain.TokenMetadata, error) {
	query := `
		SELECT candidate_id, mint, name, symbol, decimals, supply, fetched_at, created_at
		FROM token_metadata
		WHERE mint = $1
		ORDER BY fetched_at DESC
		LIMIT 1
	`

	row := s.pool.QueryRow(ctx, query, mint)
	m, err := scanTokenMetadata(row)
	if err != nil {
		if isNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get token metadata by mint: %w", err)
	}
	return m, nil
}

// scanTokenMetadata scans a single row into TokenMetadata.
func scanTokenMetadata(row pgx.Row) (*domain.TokenMetadata, error) {
	var m domain.TokenMetadata

	err := row.Scan(
		&m.CandidateID,
		&m.Mint,
		&m.Name,
		&m.Symbol,
		&m.Decimals,
		&m.Supply,
		&m.FetchedAt,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &m, nil
}
