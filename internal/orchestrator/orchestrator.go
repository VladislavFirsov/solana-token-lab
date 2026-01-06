// Package orchestrator provides E2E pipeline orchestration.
// It coordinates: normalization → simulation → metrics → reporting
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/metrics"
	"solana-token-lab/internal/normalization"
	"solana-token-lab/internal/simulation"
	"solana-token-lab/internal/storage"
)

// Orchestrator coordinates the E2E pipeline execution.
// Flow: normalization → simulation → metrics aggregation
type Orchestrator struct {
	// Stores
	candidateStore           storage.CandidateStore
	swapStore                storage.SwapStore
	liquidityEventStore      storage.LiquidityEventStore
	priceTimeseriesStore     storage.PriceTimeseriesStore
	liquidityTimeseriesStore storage.LiquidityTimeseriesStore
	volumeTimeseriesStore    storage.VolumeTimeseriesStore
	derivedFeatureStore      storage.DerivedFeatureStore
	tradeRecordStore         storage.TradeRecordStore
	strategyAggregateStore   storage.StrategyAggregateStore

	// Configs
	strategyConfigs []domain.StrategyConfig
	scenarioConfigs []domain.ScenarioConfig

	// Options
	skipNormalization bool
	verbose           bool
}

// Options for creating Orchestrator.
type Options struct {
	// Required stores
	CandidateStore           storage.CandidateStore
	SwapStore                storage.SwapStore
	LiquidityEventStore      storage.LiquidityEventStore
	PriceTimeseriesStore     storage.PriceTimeseriesStore
	LiquidityTimeseriesStore storage.LiquidityTimeseriesStore
	VolumeTimeseriesStore    storage.VolumeTimeseriesStore
	DerivedFeatureStore      storage.DerivedFeatureStore
	TradeRecordStore         storage.TradeRecordStore
	StrategyAggregateStore   storage.StrategyAggregateStore

	// Strategy and scenario configs
	StrategyConfigs []domain.StrategyConfig
	ScenarioConfigs []domain.ScenarioConfig

	// Options
	SkipNormalization bool // Skip if timeseries already exist
	Verbose           bool
}

// New creates a new Orchestrator.
func New(opts Options) *Orchestrator {
	return &Orchestrator{
		candidateStore:           opts.CandidateStore,
		swapStore:                opts.SwapStore,
		liquidityEventStore:      opts.LiquidityEventStore,
		priceTimeseriesStore:     opts.PriceTimeseriesStore,
		liquidityTimeseriesStore: opts.LiquidityTimeseriesStore,
		volumeTimeseriesStore:    opts.VolumeTimeseriesStore,
		derivedFeatureStore:      opts.DerivedFeatureStore,
		tradeRecordStore:         opts.TradeRecordStore,
		strategyAggregateStore:   opts.StrategyAggregateStore,
		strategyConfigs:          opts.StrategyConfigs,
		scenarioConfigs:          opts.ScenarioConfigs,
		skipNormalization:        opts.SkipNormalization,
		verbose:                  opts.Verbose,
	}
}

// RunResult contains results from orchestrator execution.
type RunResult struct {
	CandidatesProcessed int
	TradesCreated       int
	AggregatesCreated   int
	Errors              []string
}

// Run executes the full E2E pipeline.
// Phases:
//  1. Load candidates
//  2. Normalize each candidate (create timeseries)
//  3. Simulate each (candidate, strategy, scenario) combination
//  4. Aggregate metrics
func (o *Orchestrator) Run(ctx context.Context) (*RunResult, error) {
	result := &RunResult{}

	// Phase 1: Load all candidates
	o.log("Phase 1: Loading candidates...")
	candidates, err := o.loadCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("phase 1 (load candidates) failed: %w", err)
	}
	result.CandidatesProcessed = len(candidates)
	o.log("  Found %d candidates", len(candidates))

	if len(candidates) == 0 {
		return result, nil
	}

	// Phase 2: Normalization
	if !o.skipNormalization {
		o.log("Phase 2: Normalizing candidates...")
		if err := o.runNormalization(ctx, candidates); err != nil {
			return nil, fmt.Errorf("phase 2 (normalization) failed: %w", err)
		}
		o.log("  Normalized %d candidates", len(candidates))
	} else {
		o.log("Phase 2: Skipping normalization (skipNormalization=true)")
	}

	// Phase 3: Simulation
	o.log("Phase 3: Running simulations...")
	tradesCreated, simErrors := o.runSimulations(ctx, candidates)
	result.TradesCreated = tradesCreated
	result.Errors = append(result.Errors, simErrors...)
	o.log("  Created %d trades (%d errors)", tradesCreated, len(simErrors))

	// Phase 4: Metrics Aggregation
	o.log("Phase 4: Computing aggregates...")
	aggsCreated, aggErrors := o.runAggregation(ctx)
	result.AggregatesCreated = aggsCreated
	result.Errors = append(result.Errors, aggErrors...)
	o.log("  Created %d aggregates (%d errors)", aggsCreated, len(aggErrors))

	o.log("Pipeline completed: %d candidates, %d trades, %d aggregates",
		result.CandidatesProcessed, result.TradesCreated, result.AggregatesCreated)

	return result, nil
}

// loadCandidates loads all candidates from store.
func (o *Orchestrator) loadCandidates(ctx context.Context) ([]*domain.TokenCandidate, error) {
	newTokens, err := o.candidateStore.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return nil, err
	}

	activeTokens, err := o.candidateStore.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return nil, err
	}

	// Combine and return
	all := make([]*domain.TokenCandidate, 0, len(newTokens)+len(activeTokens))
	all = append(all, newTokens...)
	all = append(all, activeTokens...)
	return all, nil
}

// runNormalization normalizes all candidates.
func (o *Orchestrator) runNormalization(ctx context.Context, candidates []*domain.TokenCandidate) error {
	runner := normalization.NewRunner(
		o.swapStore,
		o.liquidityEventStore,
		o.priceTimeseriesStore,
		o.liquidityTimeseriesStore,
		o.volumeTimeseriesStore,
		o.derivedFeatureStore,
	)

	for _, c := range candidates {
		if err := runner.NormalizeCandidate(ctx, c.CandidateID); err != nil {
			// Skip duplicate key errors (already normalized)
			if errors.Is(err, storage.ErrDuplicateKey) {
				continue
			}
			return fmt.Errorf("normalize candidate %s: %w", c.CandidateID, err)
		}
	}
	return nil
}

// runSimulations runs all strategy/scenario combinations for all candidates.
func (o *Orchestrator) runSimulations(ctx context.Context, candidates []*domain.TokenCandidate) (int, []string) {
	runner := simulation.NewRunner(simulation.RunnerOptions{
		CandidateStore:       o.candidateStore,
		PriceTimeseriesStore: o.priceTimeseriesStore,
		LiqTimeseriesStore:   o.liquidityTimeseriesStore,
		TradeRecordStore:     o.tradeRecordStore,
	})

	var tradesCreated int
	var errs []string

	for _, candidate := range candidates {
		for _, strategyCfg := range o.strategyConfigs {
			// Skip if entry event type doesn't match candidate source
			if !sourceMatches(candidate.Source, strategyCfg.EntryEventType) {
				continue
			}

			for _, scenarioCfg := range o.scenarioConfigs {
				_, err := runner.Run(ctx, candidate.CandidateID, strategyCfg, scenarioCfg)
				if err != nil {
					// Skip duplicate key errors (already simulated)
					if errors.Is(err, storage.ErrDuplicateKey) {
						continue
					}
					// Skip source mismatch (expected for some combinations)
					if errors.Is(err, simulation.ErrSourceMismatch) {
						continue
					}
					errs = append(errs, fmt.Sprintf("simulate %s/%s/%s: %v",
						candidate.CandidateID, strategyCfg.StrategyType, scenarioCfg.ScenarioID, err))
					continue
				}
				tradesCreated++
			}
		}
	}

	return tradesCreated, errs
}

// runAggregation computes aggregates for all strategy/scenario/entry combinations.
func (o *Orchestrator) runAggregation(ctx context.Context) (int, []string) {
	aggregator := metrics.NewAggregator(
		o.tradeRecordStore,
		o.strategyAggregateStore,
		o.candidateStore,
	)

	var aggsCreated int
	var errs []string

	entryTypes := []string{"NEW_TOKEN", "ACTIVE_TOKEN"}

	for _, strategyCfg := range o.strategyConfigs {
		for _, scenarioCfg := range o.scenarioConfigs {
			for _, entryType := range entryTypes {
				_, err := aggregator.ComputeAndStore(ctx, strategyCfg.StrategyType, scenarioCfg.ScenarioID, entryType)
				if err != nil {
					// Skip duplicate key errors (already aggregated)
					if errors.Is(err, storage.ErrDuplicateKey) {
						continue
					}
					// Skip no trades (expected for some combinations)
					if errors.Is(err, metrics.ErrNoTrades) {
						continue
					}
					errs = append(errs, fmt.Sprintf("aggregate %s/%s/%s: %v",
						strategyCfg.StrategyType, scenarioCfg.ScenarioID, entryType, err))
					continue
				}
				aggsCreated++
			}
		}
	}

	return aggsCreated, errs
}

// sourceMatches checks if candidate source matches entry event type.
func sourceMatches(source domain.Source, entryEventType string) bool {
	switch entryEventType {
	case "NEW_TOKEN":
		return source == domain.SourceNewToken
	case "ACTIVE_TOKEN":
		return source == domain.SourceActiveToken
	default:
		return false
	}
}

func (o *Orchestrator) log(format string, args ...interface{}) {
	if o.verbose {
		log.Printf("[orchestrator] "+format, args...)
	}
}
