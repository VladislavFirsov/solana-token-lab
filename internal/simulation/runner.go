package simulation

import (
	"context"
	"errors"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/strategy"
)

// Runner errors
var (
	ErrSourceMismatch = errors.New("candidate source does not match strategy entry event type")
)

// Runner executes simulations for candidates.
type Runner struct {
	candidateStore       storage.CandidateStore
	priceTimeseriesStore storage.PriceTimeseriesStore
	liqTimeseriesStore   storage.LiquidityTimeseriesStore
	tradeRecordStore     storage.TradeRecordStore
}

// RunnerOptions contains configuration for creating a Runner.
type RunnerOptions struct {
	CandidateStore       storage.CandidateStore
	PriceTimeseriesStore storage.PriceTimeseriesStore
	LiqTimeseriesStore   storage.LiquidityTimeseriesStore
	TradeRecordStore     storage.TradeRecordStore
}

// NewRunner creates a simulation runner.
func NewRunner(opts RunnerOptions) *Runner {
	return &Runner{
		candidateStore:       opts.CandidateStore,
		priceTimeseriesStore: opts.PriceTimeseriesStore,
		liqTimeseriesStore:   opts.LiqTimeseriesStore,
		tradeRecordStore:     opts.TradeRecordStore,
	}
}

// Run executes a simulation for a candidate with strategy and scenario.
// Steps:
//  1. Load candidate by ID
//  2. Build strategy via strategy.FromConfig(cfg)
//  3. Validate candidate.Source matches cfg.EntryEventType
//  4. Load price/liquidity time series
//  5. Compute entry signal values per REPLAY_PROTOCOL.md
//  6. Build StrategyInput
//  7. Execute strategy
//  8. Persist TradeRecord
func (r *Runner) Run(ctx context.Context, candidateID string, cfg domain.StrategyConfig, scenario domain.ScenarioConfig) (*domain.TradeRecord, error) {
	// 1. Load candidate by ID
	candidate, err := r.candidateStore.GetByID(ctx, candidateID)
	if err != nil {
		return nil, err // propagates storage.ErrNotFound
	}

	// 2. Build strategy via factory
	strat, err := strategy.FromConfig(cfg)
	if err != nil {
		return nil, err
	}

	// 3. Validate candidate source matches entry event type
	if !sourceMatches(candidate.Source, cfg.EntryEventType) {
		return nil, ErrSourceMismatch
	}

	// 4. Load price/liquidity time series
	prices, err := r.priceTimeseriesStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return nil, err
	}

	liquidity, err := r.liqTimeseriesStore.GetByCandidateID(ctx, candidateID)
	if err != nil {
		return nil, err
	}

	// 5. Compute entry signal values per REPLAY_PROTOCOL.md
	entrySignalTime := candidate.DiscoveredAt
	entrySignalPrice, err := lookup.PriceAt(entrySignalTime, prices)
	if err != nil {
		return nil, err
	}
	entryLiquidity, err := lookup.LiquidityAt(entrySignalTime, liquidity)
	if err != nil {
		return nil, err
	}

	// 6. Build StrategyInput
	input := &strategy.StrategyInput{
		CandidateID:         candidateID,
		EntrySignalTime:     entrySignalTime,
		EntrySignalPrice:    entrySignalPrice,
		EntryLiquidity:      entryLiquidity,
		PriceTimeseries:     prices,
		LiquidityTimeseries: liquidity,
		Scenario:            scenario,
	}

	// 6.1 Validate input at package boundary
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// 7. Execute strategy
	trade, err := strat.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	// 8. Persist TradeRecord
	if r.tradeRecordStore != nil {
		if err := r.tradeRecordStore.Insert(ctx, trade); err != nil {
			return nil, err
		}
	}

	return trade, nil
}

// sourceMatches checks if candidate source matches entry event type.
func sourceMatches(candidateSource domain.Source, entryEventType string) bool {
	switch entryEventType {
	case "NEW_TOKEN":
		return candidateSource == domain.SourceNewToken
	case "ACTIVE_TOKEN":
		return candidateSource == domain.SourceActiveToken
	default:
		return false
	}
}
