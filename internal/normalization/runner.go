package normalization

import (
	"context"
)

// NormalizeCandidate processes a single candidate and generates all timeseries and features.
// Steps:
//  1. Load swaps and liquidity events from stores
//  2. Sort by (slot, tx_signature, event_index)
//  3. Generate price_timeseries -> store
//  4. Generate liquidity_timeseries -> store
//  5. Generate volume_timeseries for all intervals -> store
//  6. Compute derived_features -> store
func (r *Runner) NormalizeCandidate(ctx context.Context, candidateID string) error {
	// 1. Load raw events
	swaps, err := r.swapStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return err
	}

	liquidityEvents, err := r.liquidityStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return err
	}

	// 2. Sort by canonical order
	SortSwaps(swaps)
	SortLiquidityEvents(liquidityEvents)

	// 3. Generate price timeseries
	priceTS := GeneratePriceTimeseries(swaps)
	if len(priceTS) > 0 {
		if err := r.priceTimeseriesStore.InsertBulk(ctx, priceTS); err != nil {
			return err
		}
	}

	// 4. Generate liquidity timeseries
	liquidityTS := GenerateLiquidityTimeseries(liquidityEvents)
	if len(liquidityTS) > 0 {
		if err := r.liquidityTimeseriesStore.InsertBulk(ctx, liquidityTS); err != nil {
			return err
		}
	}

	// 5. Generate volume timeseries for all intervals
	volumeTS := GenerateAllVolumeTimeseries(swaps)
	if len(volumeTS) > 0 {
		if err := r.volumeTimeseriesStore.InsertBulk(ctx, volumeTS); err != nil {
			return err
		}
	}

	// 6. Compute derived features
	derivedFeatures := ComputeDerivedFeatures(priceTS, liquidityTS)
	if len(derivedFeatures) > 0 {
		if err := r.derivedFeatureStore.InsertBulk(ctx, derivedFeatures); err != nil {
			return err
		}
	}

	return nil
}

// NormalizeBatch processes multiple candidates.
func (r *Runner) NormalizeBatch(ctx context.Context, candidateIDs []string) error {
	for _, candidateID := range candidateIDs {
		if err := r.NormalizeCandidate(ctx, candidateID); err != nil {
			return err
		}
	}
	return nil
}
