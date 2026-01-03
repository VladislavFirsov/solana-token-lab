package normalization

import (
	"context"

	"solana-token-lab/internal/storage"
)

// NormalizationEngine defines the main normalization interface.
type NormalizationEngine interface {
	// NormalizeCandidate processes a single candidate and generates all timeseries and features.
	NormalizeCandidate(ctx context.Context, candidateID string) error
}

// Runner implements NormalizationEngine.
type Runner struct {
	swapStore                storage.SwapStore
	liquidityStore           storage.LiquidityEventStore
	priceTimeseriesStore     storage.PriceTimeseriesStore
	liquidityTimeseriesStore storage.LiquidityTimeseriesStore
	volumeTimeseriesStore    storage.VolumeTimeseriesStore
	derivedFeatureStore      storage.DerivedFeatureStore
}

// NewRunner creates a new normalization runner.
func NewRunner(
	swapStore storage.SwapStore,
	liquidityStore storage.LiquidityEventStore,
	priceTS storage.PriceTimeseriesStore,
	liquidityTS storage.LiquidityTimeseriesStore,
	volumeTS storage.VolumeTimeseriesStore,
	derivedFS storage.DerivedFeatureStore,
) *Runner {
	return &Runner{
		swapStore:                swapStore,
		liquidityStore:           liquidityStore,
		priceTimeseriesStore:     priceTS,
		liquidityTimeseriesStore: liquidityTS,
		volumeTimeseriesStore:    volumeTS,
		derivedFeatureStore:      derivedFS,
	}
}
