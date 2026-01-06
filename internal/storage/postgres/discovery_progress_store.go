package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/storage"
)

// DiscoveryProgressStore is a PostgreSQL implementation of storage.DiscoveryProgressStore.
// Uses two tables:
//   - discovery_progress: single row with (slot, signature)
//   - discovery_seen_mints: set of processed mint addresses
type DiscoveryProgressStore struct {
	pool *Pool
}

// NewDiscoveryProgressStore creates a new PostgreSQL discovery progress store.
func NewDiscoveryProgressStore(pool *Pool) *DiscoveryProgressStore {
	return &DiscoveryProgressStore{pool: pool}
}

// GetLastProcessed returns the last processed slot and signature.
func (s *DiscoveryProgressStore) GetLastProcessed(ctx context.Context) (*storage.DiscoveryProgress, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT slot, signature
		FROM discovery_progress
		LIMIT 1
	`)

	var progress storage.DiscoveryProgress
	err := row.Scan(&progress.Slot, &progress.Signature)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}

	return &progress, nil
}

// SetLastProcessed saves the last processed slot and signature.
// Uses upsert to handle initial insert and subsequent updates.
func (s *DiscoveryProgressStore) SetLastProcessed(ctx context.Context, progress *storage.DiscoveryProgress) error {
	if progress == nil {
		return storage.ErrInvalidInput
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO discovery_progress (id, slot, signature, updated_at)
		VALUES (1, $1, $2, NOW())
		ON CONFLICT (id) DO UPDATE
		SET slot = EXCLUDED.slot,
		    signature = EXCLUDED.signature,
		    updated_at = NOW()
	`, progress.Slot, progress.Signature)

	return err
}

// IsMintSeen checks if a mint address has been processed.
func (s *DiscoveryProgressStore) IsMintSeen(ctx context.Context, mint string) (bool, error) {
	if mint == "" {
		return false, storage.ErrInvalidInput
	}

	row := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM discovery_seen_mints WHERE mint = $1)
	`, mint)

	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// MarkMintSeen records that a mint address has been processed.
func (s *DiscoveryProgressStore) MarkMintSeen(ctx context.Context, mint string) error {
	if mint == "" {
		return storage.ErrInvalidInput
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO discovery_seen_mints (mint, seen_at)
		VALUES ($1, NOW())
		ON CONFLICT (mint) DO NOTHING
	`, mint)

	return err
}

// LoadSeenMints returns all seen mints.
func (s *DiscoveryProgressStore) LoadSeenMints(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT mint FROM discovery_seen_mints
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mints []string
	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return nil, err
		}
		mints = append(mints, mint)
	}

	return mints, rows.Err()
}
