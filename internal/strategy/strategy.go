package strategy

import (
	"context"

	"solana-token-lab/internal/domain"
)

// Strategy produces trades from time series data.
type Strategy interface {
	// Execute runs the strategy on price/liquidity time series.
	// Returns a deterministic trade record.
	Execute(ctx context.Context, input *StrategyInput) (*domain.TradeRecord, error)

	// ID returns strategy identifier (includes parameters).
	ID() string
}

// StrategyInput holds all data needed for strategy execution.
type StrategyInput struct {
	CandidateID         string
	EntrySignalTime     int64
	EntrySignalPrice    float64
	EntryLiquidity      *float64
	PriceTimeseries     []*domain.PriceTimeseriesPoint
	LiquidityTimeseries []*domain.LiquidityTimeseriesPoint
	Scenario            domain.ScenarioConfig
}
