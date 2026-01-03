package normalization

import (
	"sort"

	"solana-token-lab/internal/domain"
)

// ComputeDerivedFeatures computes derivatives from price and liquidity timeseries.
// Inputs are sorted by timestamp_ms internally to ensure correct LAG operations.
//
// Formulas per spec:
//   - price_delta = price[t] - price[t-1], NULL if first row
//   - price_velocity = price_delta / time_delta, NULL if first row or time_delta=0
//   - price_acceleration = (velocity[t] - velocity[t-1]) / time_delta, NULL if first/second row
//   - liquidity_delta = liquidity[t] - liquidity[t-1], NULL if first row or no matching timestamp
//   - liquidity_velocity = liquidity_delta / time_delta, NULL if first row or time_delta=0
//   - token_lifetime_ms = timestamp[t] - MIN(timestamp_ms) for candidate
//   - last_swap_interval_ms = timestamp[t] - timestamp[t-1], NULL if first row
//   - last_liq_event_interval_ms = computed from liquidity timeseries, NULL if no prior event
func ComputeDerivedFeatures(
	priceTS []*domain.PriceTimeseriesPoint,
	liquidityTS []*domain.LiquidityTimeseriesPoint,
) []*domain.DerivedFeaturePoint {
	if len(priceTS) == 0 {
		return nil
	}

	// Sort price timeseries by (candidateID, timestamp) for correct LAG operations
	sortedPriceTS := make([]*domain.PriceTimeseriesPoint, len(priceTS))
	copy(sortedPriceTS, priceTS)
	sort.Slice(sortedPriceTS, func(i, j int) bool {
		if sortedPriceTS[i].CandidateID != sortedPriceTS[j].CandidateID {
			return sortedPriceTS[i].CandidateID < sortedPriceTS[j].CandidateID
		}
		return sortedPriceTS[i].TimestampMs < sortedPriceTS[j].TimestampMs
	})

	// Build liquidity lookup by (candidateID, timestamp)
	liqLookup := buildLiquidityLookup(liquidityTS)

	// Track previous liquidity timestamp per candidate
	prevLiqTimestamp := make(map[string]int64)

	// Compute MIN(timestamp_ms) per candidate for token_lifetime_ms
	minTimestamp := make(map[string]int64)
	for _, p := range sortedPriceTS {
		if existing, ok := minTimestamp[p.CandidateID]; !ok || p.TimestampMs < existing {
			minTimestamp[p.CandidateID] = p.TimestampMs
		}
	}

	result := make([]*domain.DerivedFeaturePoint, len(sortedPriceTS))

	// Track previous values per candidate
	prevPrice := make(map[string]float64)
	prevTimestamp := make(map[string]int64)
	prevVelocity := make(map[string]*float64)
	isFirst := make(map[string]bool)
	isSecond := make(map[string]bool)

	for i, p := range sortedPriceTS {
		candidateID := p.CandidateID
		timestamp := p.TimestampMs
		price := p.Price

		point := &domain.DerivedFeaturePoint{
			CandidateID:     candidateID,
			TimestampMs:     timestamp,
			TokenLifetimeMs: timestamp - minTimestamp[candidateID],
		}

		if !isFirst[candidateID] {
			// First row for this candidate
			isFirst[candidateID] = true
			// All derivatives NULL
			// last_swap_interval_ms = NULL
		} else {
			// Not first row
			prevTs := prevTimestamp[candidateID]
			prevPr := prevPrice[candidateID]
			timeDelta := timestamp - prevTs

			// last_swap_interval_ms
			lastSwapInterval := timeDelta
			point.LastSwapIntervalMs = &lastSwapInterval

			// price_delta
			priceDelta := price - prevPr
			point.PriceDelta = &priceDelta

			// price_velocity
			if timeDelta > 0 {
				velocity := priceDelta / float64(timeDelta)
				point.PriceVelocity = &velocity

				// price_acceleration (need velocity from t-1)
				if isSecond[candidateID] && prevVelocity[candidateID] != nil {
					prevVel := *prevVelocity[candidateID]
					accel := (velocity - prevVel) / float64(timeDelta)
					point.PriceAcceleration = &accel
				}

				prevVelocity[candidateID] = &velocity
			}

			if !isSecond[candidateID] {
				isSecond[candidateID] = true
			}
		}

		// Liquidity features - only if matching timestamp exists
		liqKey := liqLookupKey{candidateID: candidateID, timestamp: timestamp}
		if liq, ok := liqLookup[liqKey]; ok {
			// Check if we have previous liquidity event for this candidate
			if prevLiqTs, hasPrev := prevLiqTimestamp[candidateID]; hasPrev {
				// Find previous liquidity value
				prevLiqKey := liqLookupKey{candidateID: candidateID, timestamp: prevLiqTs}
				if prevLiq, ok := liqLookup[prevLiqKey]; ok {
					liqDelta := liq.Liquidity - prevLiq.Liquidity
					point.LiquidityDelta = &liqDelta

					timeDelta := timestamp - prevLiqTs
					if timeDelta > 0 {
						liqVelocity := liqDelta / float64(timeDelta)
						point.LiquidityVelocity = &liqVelocity
					}

					// last_liq_event_interval_ms
					liqInterval := timestamp - prevLiqTs
					point.LastLiqEventIntervalMs = &liqInterval
				}
			}
			prevLiqTimestamp[candidateID] = timestamp
		}

		// Update previous values for next iteration
		prevPrice[candidateID] = price
		prevTimestamp[candidateID] = timestamp

		result[i] = point
	}

	return result
}

type liqLookupKey struct {
	candidateID string
	timestamp   int64
}

func buildLiquidityLookup(liquidityTS []*domain.LiquidityTimeseriesPoint) map[liqLookupKey]*domain.LiquidityTimeseriesPoint {
	lookup := make(map[liqLookupKey]*domain.LiquidityTimeseriesPoint)
	for _, l := range liquidityTS {
		key := liqLookupKey{candidateID: l.CandidateID, timestamp: l.TimestampMs}
		lookup[key] = l
	}
	return lookup
}
