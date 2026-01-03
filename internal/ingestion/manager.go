package ingestion

import (
	"context"

	"solana-token-lab/internal/storage"
)

// Manager orchestrates ingestion from sources to storage.
// It enforces deterministic ordering and uses storage layer for duplicate rejection.
type Manager struct {
	swapSource      SwapSource
	liquiditySource LiquidityEventSource
	metadataSource  MetadataSource

	swapStore      storage.SwapStore
	liquidityStore storage.LiquidityEventStore
	metadataStore  storage.TokenMetadataStore
}

// ManagerOptions contains configuration for creating a Manager.
type ManagerOptions struct {
	SwapSource      SwapSource
	LiquiditySource LiquidityEventSource
	MetadataSource  MetadataSource

	SwapStore      storage.SwapStore
	LiquidityStore storage.LiquidityEventStore
	MetadataStore  storage.TokenMetadataStore
}

// NewManager creates a new ingestion manager with the provided sources and stores.
func NewManager(opts ManagerOptions) *Manager {
	return &Manager{
		swapSource:      opts.SwapSource,
		liquiditySource: opts.LiquiditySource,
		metadataSource:  opts.MetadataSource,
		swapStore:       opts.SwapStore,
		liquidityStore:  opts.LiquidityStore,
		metadataStore:   opts.MetadataStore,
	}
}

// IngestSwaps fetches swaps from source and stores them.
// Enforces deterministic ordering by (slot, tx_signature, event_index).
// Returns count of ingested swaps and any error.
// Duplicates are rejected by the storage layer (ErrDuplicateKey).
func (m *Manager) IngestSwaps(ctx context.Context, candidateID string, from, to int64) (int, error) {
	if m.swapSource == nil || m.swapStore == nil {
		return 0, nil
	}

	swaps, err := m.swapSource.Fetch(ctx, candidateID, from, to)
	if err != nil {
		return 0, err
	}

	if len(swaps) == 0 {
		return 0, nil
	}

	// Enforce deterministic ordering
	SortSwaps(swaps)

	// Store via bulk insert - storage layer handles duplicates
	if err := m.swapStore.InsertBulk(ctx, swaps); err != nil {
		return 0, err
	}

	return len(swaps), nil
}

// IngestLiquidityEvents fetches liquidity events from source and stores them.
// Enforces deterministic ordering by (slot, tx_signature, event_index).
// Returns count of ingested events and any error.
func (m *Manager) IngestLiquidityEvents(ctx context.Context, candidateID string, from, to int64) (int, error) {
	if m.liquiditySource == nil || m.liquidityStore == nil {
		return 0, nil
	}

	events, err := m.liquiditySource.Fetch(ctx, candidateID, from, to)
	if err != nil {
		return 0, err
	}

	if len(events) == 0 {
		return 0, nil
	}

	// Enforce deterministic ordering
	SortLiquidityEvents(events)

	// Store via bulk insert - storage layer handles duplicates
	if err := m.liquidityStore.InsertBulk(ctx, events); err != nil {
		return 0, err
	}

	return len(events), nil
}

// IngestMetadata fetches token metadata and stores it.
// If metadata for candidate_id or mint already exists, storage rejects as duplicate.
func (m *Manager) IngestMetadata(ctx context.Context, candidateID, mint string) error {
	if m.metadataSource == nil || m.metadataStore == nil {
		return nil
	}

	meta, err := m.metadataSource.Fetch(ctx, mint)
	if err != nil {
		return err
	}

	if meta == nil {
		return nil
	}

	// Set candidate ID on the fetched metadata
	meta.CandidateID = candidateID

	// Store - storage layer handles duplicates
	return m.metadataStore.Insert(ctx, meta)
}
