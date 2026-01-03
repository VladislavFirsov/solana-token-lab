package domain

// PriceTimeseriesPoint represents aggregated price data from swaps.
// Corresponds to price_timeseries table in ClickHouse.
type PriceTimeseriesPoint struct {
	CandidateID string  // token candidate identifier
	TimestampMs int64   // Unix timestamp in milliseconds
	Slot        int64   // Solana slot number
	Price       float64 // price at this point
	Volume      float64 // volume at this point
	SwapCount   int     // number of swaps aggregated
}

// LiquidityTimeseriesPoint represents aggregated liquidity data.
// Corresponds to liquidity_timeseries table in ClickHouse.
type LiquidityTimeseriesPoint struct {
	CandidateID    string  // token candidate identifier
	TimestampMs    int64   // Unix timestamp in milliseconds
	Slot           int64   // Solana slot number
	Liquidity      float64 // total pool liquidity
	LiquidityToken float64 // token side liquidity
	LiquidityQuote float64 // quote side liquidity (SOL/USDC)
}

// VolumeTimeseriesPoint represents volume aggregated by time intervals.
// Corresponds to volume_timeseries table in ClickHouse.
type VolumeTimeseriesPoint struct {
	CandidateID     string  // token candidate identifier
	TimestampMs     int64   // interval start timestamp (ms)
	IntervalSeconds int     // aggregation interval: 60, 300, 3600
	Volume          float64 // total volume in interval
	SwapCount       int     // number of swaps in interval
	BuyVolume       float64 // buy-side volume
	SellVolume      float64 // sell-side volume
}

// Supported volume aggregation intervals (in seconds)
const (
	VolumeInterval1Min  = 60
	VolumeInterval5Min  = 300
	VolumeInterval1Hour = 3600
)
