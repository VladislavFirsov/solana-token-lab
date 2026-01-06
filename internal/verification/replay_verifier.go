package verification

import (
	"context"
	"errors"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/lookup"
	"solana-token-lab/internal/storage"
	"solana-token-lab/internal/strategy"
)

var (
	// ErrTradeNotFound is returned when trade ID doesn't exist.
	ErrTradeNotFound = errors.New("trade not found")

	// ErrCandidateNotFound is returned when candidate ID doesn't exist.
	ErrCandidateNotFound = errors.New("candidate not found")
)

// ReplayVerifier implements Verifier interface.
type ReplayVerifier struct {
	tradeStore     storage.TradeRecordStore
	candidateStore storage.CandidateStore
	priceStore     storage.PriceTimeseriesStore
	liquidityStore storage.LiquidityTimeseriesStore

	// strategyConfigs maps strategy ID to its configuration.
	// Must be pre-populated with all known strategy configs.
	strategyConfigs map[string]domain.StrategyConfig

	// scenarioConfigs maps scenario ID to its configuration.
	// Must be pre-populated with all known scenario configs.
	scenarioConfigs map[string]domain.ScenarioConfig
}

// ReplayVerifierOptions contains configuration for creating a ReplayVerifier.
type ReplayVerifierOptions struct {
	TradeStore      storage.TradeRecordStore
	CandidateStore  storage.CandidateStore
	PriceStore      storage.PriceTimeseriesStore
	LiquidityStore  storage.LiquidityTimeseriesStore
	StrategyConfigs map[string]domain.StrategyConfig
	ScenarioConfigs map[string]domain.ScenarioConfig
}

// NewReplayVerifier creates a new ReplayVerifier.
func NewReplayVerifier(opts ReplayVerifierOptions) *ReplayVerifier {
	return &ReplayVerifier{
		tradeStore:      opts.TradeStore,
		candidateStore:  opts.CandidateStore,
		priceStore:      opts.PriceStore,
		liquidityStore:  opts.LiquidityStore,
		strategyConfigs: opts.StrategyConfigs,
		scenarioConfigs: opts.ScenarioConfigs,
	}
}

// VerifyTrade verifies a single trade by replaying simulation.
func (v *ReplayVerifier) VerifyTrade(ctx context.Context, tradeID string) (*VerificationResult, error) {
	// 1. Load stored trade
	stored, err := v.tradeStore.GetByID(ctx, tradeID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrTradeNotFound
		}
		return nil, err
	}

	// 2. Replay simulation
	replayed, err := v.replayTrade(ctx, stored)
	if err != nil {
		return nil, err
	}

	// 3. Compare results
	divergences := CompareTradeRecords(stored, replayed)

	return &VerificationResult{
		TradeID:         tradeID,
		Match:           len(divergences) == 0,
		Divergences:     divergences,
		StoredOutcome:   stored.Outcome,
		ReplayedOutcome: replayed.Outcome,
	}, nil
}

// VerifyAll verifies all stored trades.
func (v *ReplayVerifier) VerifyAll(ctx context.Context) (*VerificationReport, error) {
	// Load all trades
	trades, err := v.tradeStore.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	report := &VerificationReport{
		TotalTrades: len(trades),
		Results:     make([]VerificationResult, 0, len(trades)),
	}

	for _, trade := range trades {
		result, err := v.VerifyTrade(ctx, trade.TradeID)
		if err != nil {
			// Record error as divergence
			report.Results = append(report.Results, VerificationResult{
				TradeID:       trade.TradeID,
				Match:         false,
				StoredOutcome: trade.Outcome,
				Divergences: []FieldDivergence{
					{Field: "Error", Expected: nil, Actual: err.Error()},
				},
			})
			report.DivergentTrades++
			continue
		}

		report.Results = append(report.Results, *result)
		if result.Match {
			report.MatchedTrades++
		} else {
			report.DivergentTrades++
		}
	}

	return report, nil
}

// replayTrade re-executes simulation with stored trade's parameters.
func (v *ReplayVerifier) replayTrade(ctx context.Context, stored *domain.TradeRecord) (*domain.TradeRecord, error) {
	// 1. Load candidate
	candidate, err := v.candidateStore.GetByID(ctx, stored.CandidateID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrCandidateNotFound
		}
		return nil, err
	}

	// 2. Get strategy config
	strategyCfg, ok := v.strategyConfigs[stored.StrategyID]
	if !ok {
		return nil, errors.New("unknown strategy ID: " + stored.StrategyID)
	}

	// 2.1 Validate entry event source matches strategy config per REPLAY_PROTOCOL.md Section 4.1
	if strategyCfg.EntryEventType != "" && string(candidate.Source) != strategyCfg.EntryEventType {
		return nil, errors.New("candidate source " + string(candidate.Source) +
			" does not match strategy entry type " + strategyCfg.EntryEventType)
	}

	// 3. Get scenario config
	scenarioCfg, ok := v.scenarioConfigs[stored.ScenarioID]
	if !ok {
		return nil, errors.New("unknown scenario ID: " + stored.ScenarioID)
	}

	// 4. Build strategy
	strat, err := strategy.FromConfig(strategyCfg)
	if err != nil {
		return nil, err
	}

	// 5. Load price/liquidity timeseries
	prices, err := v.priceStore.GetByCandidateID(ctx, stored.CandidateID)
	if err != nil {
		return nil, err
	}

	liquidity, err := v.liquidityStore.GetByCandidateID(ctx, stored.CandidateID)
	if err != nil {
		return nil, err
	}

	// 6. Compute entry signal values per REPLAY_PROTOCOL.md
	entrySignalTime := candidate.DiscoveredAt
	entrySignalPrice, err := lookup.PriceAt(entrySignalTime, prices)
	if err != nil {
		return nil, err
	}
	entryLiquidity, err := lookup.LiquidityAt(entrySignalTime, liquidity)
	if err != nil {
		return nil, err
	}

	// 7. Build StrategyInput
	input := &strategy.StrategyInput{
		CandidateID:         stored.CandidateID,
		EntrySignalTime:     entrySignalTime,
		EntrySignalPrice:    entrySignalPrice,
		EntryLiquidity:      entryLiquidity,
		PriceTimeseries:     prices,
		LiquidityTimeseries: liquidity,
		Scenario:            scenarioCfg,
	}

	// 8. Validate input
	if err := input.Validate(); err != nil {
		return nil, err
	}

	// 9. Execute strategy
	replayed, err := strat.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	return replayed, nil
}
