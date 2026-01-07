package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"solana-token-lab/internal/storage"
)

func TestDiscoveryProgressStore_SetAndGetLastProcessed(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	progress := &storage.DiscoveryProgress{
		Slot:      uint64(12345),
		Signature: "TestSignature123",
	}

	// Set
	err := store.SetLastProcessed(ctx, progress)
	require.NoError(t, err)

	// Get
	retrieved, err := store.GetLastProcessed(ctx)
	require.NoError(t, err)

	assert.Equal(t, progress.Slot, retrieved.Slot)
	assert.Equal(t, progress.Signature, retrieved.Signature)
}

func TestDiscoveryProgressStore_GetLastProcessedNotFound(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Get without setting should return ErrNotFound
	_, err := store.GetLastProcessed(ctx)
	assert.ErrorIs(t, err, storage.ErrNotFound)
}

func TestDiscoveryProgressStore_SetLastProcessedUpsert(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// First set
	progress1 := &storage.DiscoveryProgress{
		Slot:      uint64(100),
		Signature: "Sig100",
	}

	err := store.SetLastProcessed(ctx, progress1)
	require.NoError(t, err)

	// Second set (upsert)
	progress2 := &storage.DiscoveryProgress{
		Slot:      uint64(200),
		Signature: "Sig200",
	}

	err = store.SetLastProcessed(ctx, progress2)
	require.NoError(t, err)

	// Get should return latest
	retrieved, err := store.GetLastProcessed(ctx)
	require.NoError(t, err)

	assert.Equal(t, progress2.Slot, retrieved.Slot)
	assert.Equal(t, progress2.Signature, retrieved.Signature)
}

func TestDiscoveryProgressStore_SetLastProcessedNil(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	err := store.SetLastProcessed(ctx, nil)
	assert.ErrorIs(t, err, storage.ErrInvalidInput)
}

func TestDiscoveryProgressStore_MarkMintSeenAndIsMintSeen(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	mint := "TestMint123"

	// Initially not seen
	seen, err := store.IsMintSeen(ctx, mint)
	require.NoError(t, err)
	assert.False(t, seen)

	// Mark as seen
	err = store.MarkMintSeen(ctx, mint)
	require.NoError(t, err)

	// Now should be seen
	seen, err = store.IsMintSeen(ctx, mint)
	require.NoError(t, err)
	assert.True(t, seen)
}

func TestDiscoveryProgressStore_MarkMintSeenIdempotent(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	mint := "IdempotentMint"

	// Mark twice should not error (ON CONFLICT DO NOTHING)
	err := store.MarkMintSeen(ctx, mint)
	require.NoError(t, err)

	err = store.MarkMintSeen(ctx, mint)
	require.NoError(t, err)

	seen, err := store.IsMintSeen(ctx, mint)
	require.NoError(t, err)
	assert.True(t, seen)
}

func TestDiscoveryProgressStore_MarkMintSeenEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	err := store.MarkMintSeen(ctx, "")
	assert.ErrorIs(t, err, storage.ErrInvalidInput)
}

func TestDiscoveryProgressStore_IsMintSeenEmpty(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	_, err := store.IsMintSeen(ctx, "")
	assert.ErrorIs(t, err, storage.ErrInvalidInput)
}

func TestDiscoveryProgressStore_LoadSeenMints(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Initially empty
	mints, err := store.LoadSeenMints(ctx)
	require.NoError(t, err)
	assert.Empty(t, mints)

	// Add some mints
	testMints := []string{"Mint1", "Mint2", "Mint3"}
	for _, m := range testMints {
		err := store.MarkMintSeen(ctx, m)
		require.NoError(t, err)
	}

	// Load all mints
	mints, err = store.LoadSeenMints(ctx)
	require.NoError(t, err)

	assert.Len(t, mints, 3)
	for _, m := range testMints {
		assert.Contains(t, mints, m)
	}
}

func TestDiscoveryProgressStore_ManyMints(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Add many mints
	numMints := 100
	for i := 0; i < numMints; i++ {
		mint := "Mint" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		err := store.MarkMintSeen(ctx, mint)
		require.NoError(t, err)
	}

	// Load and verify count
	mints, err := store.LoadSeenMints(ctx)
	require.NoError(t, err)
	assert.Len(t, mints, numMints)
}

func TestDiscoveryProgressStore_ProgressAndMintsIndependent(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Set progress
	progress := &storage.DiscoveryProgress{
		Slot:      uint64(500),
		Signature: "ProgressSig500",
	}
	err := store.SetLastProcessed(ctx, progress)
	require.NoError(t, err)

	// Add mints
	err = store.MarkMintSeen(ctx, "IndependentMint1")
	require.NoError(t, err)

	err = store.MarkMintSeen(ctx, "IndependentMint2")
	require.NoError(t, err)

	// Verify both work independently
	retrieved, err := store.GetLastProcessed(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(500), retrieved.Slot)

	mints, err := store.LoadSeenMints(ctx)
	require.NoError(t, err)
	assert.Len(t, mints, 2)

	// Update progress
	progress2 := &storage.DiscoveryProgress{
		Slot:      uint64(600),
		Signature: "ProgressSig600",
	}
	err = store.SetLastProcessed(ctx, progress2)
	require.NoError(t, err)

	// Mints should be unchanged
	mints, err = store.LoadSeenMints(ctx)
	require.NoError(t, err)
	assert.Len(t, mints, 2)
}

func TestDiscoveryProgressStore_LargeSlot(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Large slot number (typical Solana slot)
	progress := &storage.DiscoveryProgress{
		Slot:      uint64(250000000), // ~250 million
		Signature: "LargeSlotSig",
	}

	err := store.SetLastProcessed(ctx, progress)
	require.NoError(t, err)

	retrieved, err := store.GetLastProcessed(ctx)
	require.NoError(t, err)

	assert.Equal(t, progress.Slot, retrieved.Slot)
}

func TestDiscoveryProgressStore_LongSignature(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	store := NewDiscoveryProgressStore(pool)

	// Typical Solana signature is 88 characters base58
	longSignature := "5VERv8NMvzbJMEkV8xnrLkEaWRtSz9CosKDYjCJjBRnbJLgp8uirBgmQpjKhoR4tjF3ZpRzrFmBV6UjKdiSZkQUW"

	progress := &storage.DiscoveryProgress{
		Slot:      uint64(100),
		Signature: longSignature,
	}

	err := store.SetLastProcessed(ctx, progress)
	require.NoError(t, err)

	retrieved, err := store.GetLastProcessed(ctx)
	require.NoError(t, err)

	assert.Equal(t, longSignature, retrieved.Signature)
}
