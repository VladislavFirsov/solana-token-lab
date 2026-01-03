package domain

// DerivedFeaturePoint represents computed features from time series data.
// Corresponds to derived_features table in ClickHouse.
type DerivedFeaturePoint struct {
	CandidateID            string   // token candidate identifier
	TimestampMs            int64    // Unix timestamp in milliseconds
	PriceDelta             *float64 // price[t] - price[t-1], NULL if first row
	PriceVelocity          *float64 // price_delta / time_delta, NULL if first row
	PriceAcceleration      *float64 // velocity change rate, NULL if first/second row
	LiquidityDelta         *float64 // liquidity[t] - liquidity[t-1], NULL if first row
	LiquidityVelocity      *float64 // liquidity_delta / time_delta, NULL if first row
	TokenLifetimeMs        int64    // time since first swap (ms)
	LastSwapIntervalMs     *int64   // time since previous swap, NULL if first row
	LastLiqEventIntervalMs *int64   // time since previous liquidity event, NULL if none
}
