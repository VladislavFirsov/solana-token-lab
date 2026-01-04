package lookup

import (
	"errors"

	"solana-token-lab/internal/domain"
)

// Errors returned by lookup functions.
var (
	ErrNoPriceData     = errors.New("no price data available")
	ErrNoLiquidityData = errors.New("no liquidity data available")
)

// PriceAt returns price at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest price at or before target_time.
// If no price before target, returns first available price.
// Returns ErrNoPriceData if slice is empty.
func PriceAt(target int64, prices []*domain.PriceTimeseriesPoint) (float64, error) {
	if len(prices) == 0 {
		return 0, ErrNoPriceData
	}

	// Find closest price at or before target
	for i := len(prices) - 1; i >= 0; i-- {
		if prices[i].TimestampMs <= target {
			return prices[i].Price, nil
		}
	}

	// If no price before target, use first available
	return prices[0].Price, nil
}

// LiquidityAt returns liquidity at or before target timestamp.
// Per REPLAY_PROTOCOL.md: find closest liquidity at or before target_time.
// Returns (nil, nil) if no liquidity event before target (valid case).
// Returns ErrNoLiquidityData if slice is empty.
func LiquidityAt(target int64, liq []*domain.LiquidityTimeseriesPoint) (*float64, error) {
	if len(liq) == 0 {
		return nil, ErrNoLiquidityData
	}

	// Find closest liquidity at or before target
	for i := len(liq) - 1; i >= 0; i-- {
		if liq[i].TimestampMs <= target {
			return &liq[i].Liquidity, nil
		}
	}

	// If no liquidity event before target, return nil (valid case)
	return nil, nil
}
