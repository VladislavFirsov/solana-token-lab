package stub

import (
	"context"

	"solana-token-lab/internal/domain"
)

// StubSwapSource returns fixed in-memory swaps for testing.
// Events can be intentionally unordered to test sorting.
// Implements ingestion.SwapSource interface.
type StubSwapSource struct {
	swaps []*domain.Swap
}

// NewStubSwapSource creates a new stub swap source with the given swaps.
func NewStubSwapSource(swaps []*domain.Swap) *StubSwapSource {
	return &StubSwapSource{swaps: swaps}
}

// Fetch returns swaps matching the candidate ID and time range.
// Returns copies to prevent mutation.
func (s *StubSwapSource) Fetch(_ context.Context, candidateID string, from, to int64) ([]*domain.Swap, error) {
	var result []*domain.Swap
	for _, swap := range s.swaps {
		if swap.CandidateID == candidateID && swap.Timestamp >= from && swap.Timestamp <= to {
			copy := *swap
			result = append(result, &copy)
		}
	}
	return result, nil
}

// StubLiquidityEventSource returns fixed in-memory liquidity events for testing.
// Implements ingestion.LiquidityEventSource interface.
type StubLiquidityEventSource struct {
	events []*domain.LiquidityEvent
}

// NewStubLiquidityEventSource creates a new stub liquidity event source.
func NewStubLiquidityEventSource(events []*domain.LiquidityEvent) *StubLiquidityEventSource {
	return &StubLiquidityEventSource{events: events}
}

// Fetch returns events matching the candidate ID and time range.
func (s *StubLiquidityEventSource) Fetch(_ context.Context, candidateID string, from, to int64) ([]*domain.LiquidityEvent, error) {
	var result []*domain.LiquidityEvent
	for _, event := range s.events {
		if event.CandidateID == candidateID && event.Timestamp >= from && event.Timestamp <= to {
			copy := *event
			result = append(result, &copy)
		}
	}
	return result, nil
}

// StubMetadataSource returns fixed in-memory metadata for testing.
// Implements ingestion.MetadataSource interface.
type StubMetadataSource struct {
	metadata map[string]*domain.TokenMetadata // keyed by mint
}

// NewStubMetadataSource creates a new stub metadata source.
func NewStubMetadataSource(metadata []*domain.TokenMetadata) *StubMetadataSource {
	m := make(map[string]*domain.TokenMetadata)
	for _, meta := range metadata {
		m[meta.Mint] = meta
	}
	return &StubMetadataSource{metadata: m}
}

// Fetch returns metadata for the given mint address.
func (s *StubMetadataSource) Fetch(_ context.Context, mint string) (*domain.TokenMetadata, error) {
	meta, exists := s.metadata[mint]
	if !exists {
		return nil, nil
	}
	copy := *meta
	return &copy, nil
}
