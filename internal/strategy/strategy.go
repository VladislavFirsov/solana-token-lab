package strategy

import (
	"context"
	"errors"

	"solana-token-lab/internal/domain"
)

// Validation errors for StrategyInput.
var (
	ErrEmptyCandidateID     = errors.New("candidate_id is empty")
	ErrInvalidSignalTime    = errors.New("entry_signal_time must be positive")
	ErrInvalidSignalPrice   = errors.New("entry_signal_price must be positive")
	ErrEmptyPriceTimeseries = errors.New("price_timeseries is empty")
	ErrEmptyScenarioID      = errors.New("scenario_id is empty")
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

// Validate checks StrategyInput fields and returns error on invalid input.
func (s *StrategyInput) Validate() error {
	if s == nil {
		return errors.New("strategy input is nil")
	}
	if s.CandidateID == "" {
		return ErrEmptyCandidateID
	}
	if s.EntrySignalTime <= 0 {
		return ErrInvalidSignalTime
	}
	if s.EntrySignalPrice <= 0 {
		return ErrInvalidSignalPrice
	}
	if len(s.PriceTimeseries) == 0 {
		return ErrEmptyPriceTimeseries
	}
	if s.Scenario.ScenarioID == "" {
		return ErrEmptyScenarioID
	}
	return nil
}
