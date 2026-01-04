package simulation

import (
	"solana-token-lab/internal/domain"
)

// priceAt returns price at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest price at or before target_time.
// If no price before target, returns first available price.
// Returns 0 if no prices available.
func priceAt(target int64, prices []*domain.PriceTimeseriesPoint) float64 {
	if len(prices) == 0 {
		return 0
	}

	// Find closest price at or before target
	for i := len(prices) - 1; i >= 0; i-- {
		if prices[i].TimestampMs <= target {
			return prices[i].Price
		}
	}

	// If no price before target, use first available
	return prices[0].Price
}

// liquidityAt returns liquidity at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest liquidity at or before target_time.
// Returns nil if no liquidity event before target.
func liquidityAt(target int64, liq []*domain.LiquidityTimeseriesPoint) *float64 {
	if len(liq) == 0 {
		return nil
	}

	// Find closest liquidity at or before target
	for i := len(liq) - 1; i >= 0; i-- {
		if liq[i].TimestampMs <= target {
			return &liq[i].Liquidity
		}
	}

	// If no liquidity event before target, return nil
	return nil
}
