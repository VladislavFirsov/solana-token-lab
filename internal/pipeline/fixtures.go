package pipeline

import (
	"context"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LoadFixtures populates stores with test data for Phase 1 demonstration.
// Deprecated: Use LoadCandidatesAndTrades + Aggregator for proper missing candidate detection.
func LoadFixtures(
	ctx context.Context,
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
	aggStore storage.StrategyAggregateStore,
) error {
	// Load candidates
	if err := loadCandidates(ctx, candidateStore); err != nil {
		return err
	}

	// Load trades
	if err := loadTrades(ctx, tradeStore); err != nil {
		return err
	}

	// Load aggregates
	if err := loadAggregates(ctx, aggStore); err != nil {
		return err
	}

	return nil
}

// LoadCandidatesAndTrades populates candidate and trade stores with test data.
// Use this with Aggregator.ComputeAndStore to compute aggregates with proper
// missing candidate detection.
func LoadCandidatesAndTrades(
	ctx context.Context,
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
) error {
	// Load candidates
	if err := loadCandidates(ctx, candidateStore); err != nil {
		return err
	}

	// Load trades
	if err := loadTrades(ctx, tradeStore); err != nil {
		return err
	}

	return nil
}

// LoadCandidatesOnly populates candidate store with test data (no trades).
// Use this when simulation will generate trades fresh via orchestrator.
func LoadCandidatesOnly(
	ctx context.Context,
	candidateStore storage.CandidateStore,
) error {
	return loadCandidates(ctx, candidateStore)
}

func loadCandidates(ctx context.Context, store storage.CandidateStore) error {
	candidates := []*domain.TokenCandidate{
		{
			CandidateID:  "cand_001",
			Mint:         "So11111111111111111111111111111111111111112",
			Source:       domain.SourceNewToken,
			DiscoveredAt: 1704067200000, // 2024-01-01 00:00:00 UTC
		},
		{
			CandidateID:  "cand_002",
			Mint:         "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			Source:       domain.SourceNewToken,
			DiscoveredAt: 1704153600000, // 2024-01-02 00:00:00 UTC
		},
		{
			CandidateID:  "cand_003",
			Mint:         "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
			Source:       domain.SourceActiveToken,
			DiscoveredAt: 1704240000000, // 2024-01-03 00:00:00 UTC
		},
	}

	for _, c := range candidates {
		if err := store.Insert(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func loadTrades(ctx context.Context, store storage.TradeRecordStore) error {
	trades := []*domain.TradeRecord{
		// TIME_EXIT strategy, realistic scenario, NEW_TOKEN
		{
			TradeID:          "trade_001",
			CandidateID:      "cand_001",
			StrategyID:       "TIME_EXIT",
			ScenarioID:       domain.ScenarioRealistic,
			EntrySignalTime:  1704067260000,
			EntrySignalPrice: 1.0,
			EntryActualTime:  1704067260000,
			EntryActualPrice: 1.0,
			ExitSignalTime:   1704070860000,
			ExitSignalPrice:  1.08,
			ExitActualTime:   1704070860000,
			ExitActualPrice:  1.08,
			Outcome:          0.08,
			OutcomeClass:     domain.OutcomeClassWin,
		},
		{
			TradeID:          "trade_002",
			CandidateID:      "cand_002",
			StrategyID:       "TIME_EXIT",
			ScenarioID:       domain.ScenarioRealistic,
			EntrySignalTime:  1704153660000,
			EntrySignalPrice: 1.0,
			EntryActualTime:  1704153660000,
			EntryActualPrice: 1.0,
			ExitSignalTime:   1704157260000,
			ExitSignalPrice:  1.05,
			ExitActualTime:   1704157260000,
			ExitActualPrice:  1.05,
			Outcome:          0.05,
			OutcomeClass:     domain.OutcomeClassWin,
		},
		// TIME_EXIT strategy, degraded scenario, NEW_TOKEN
		{
			TradeID:          "trade_003",
			CandidateID:      "cand_001",
			StrategyID:       "TIME_EXIT",
			ScenarioID:       domain.ScenarioDegraded,
			EntrySignalTime:  1704067260000,
			EntrySignalPrice: 1.0,
			EntryActualTime:  1704067260000,
			EntryActualPrice: 1.0,
			ExitSignalTime:   1704070860000,
			ExitSignalPrice:  1.04,
			ExitActualTime:   1704070860000,
			ExitActualPrice:  1.04,
			Outcome:          0.04,
			OutcomeClass:     domain.OutcomeClassWin,
		},
		{
			TradeID:          "trade_004",
			CandidateID:      "cand_002",
			StrategyID:       "TIME_EXIT",
			ScenarioID:       domain.ScenarioDegraded,
			EntrySignalTime:  1704153660000,
			EntrySignalPrice: 1.0,
			EntryActualTime:  1704153660000,
			EntryActualPrice: 1.0,
			ExitSignalTime:   1704157260000,
			ExitSignalPrice:  1.02,
			ExitActualTime:   1704157260000,
			ExitActualPrice:  1.02,
			Outcome:          0.02,
			OutcomeClass:     domain.OutcomeClassWin,
		},
		// ACTIVE_TOKEN trades
		{
			TradeID:          "trade_005",
			CandidateID:      "cand_003",
			StrategyID:       "TIME_EXIT",
			ScenarioID:       domain.ScenarioRealistic,
			EntrySignalTime:  1704240060000,
			EntrySignalPrice: 1.0,
			EntryActualTime:  1704240060000,
			EntryActualPrice: 1.0,
			ExitSignalTime:   1704243660000,
			ExitSignalPrice:  1.03,
			ExitActualTime:   1704243660000,
			ExitActualPrice:  1.03,
			Outcome:          0.03,
			OutcomeClass:     domain.OutcomeClassWin,
		},
	}

	for _, t := range trades {
		if err := store.Insert(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func loadAggregates(ctx context.Context, store storage.StrategyAggregateStore) error {
	aggregates := []*domain.StrategyAggregate{
		// TIME_EXIT, realistic, NEW_TOKEN - good metrics (GO case)
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioRealistic,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          100,
			TotalTokens:          80,
			WinRate:              0.12,  // 12% > 5% (trade-level)
			TokenWinRate:         0.10,  // 10% > 5% (token-level, for decision)
			OutcomeMean:          0.065,
			OutcomeMedian:        0.05, // > 0
			OutcomeP10:           -0.02,
			OutcomeP25:           0.02, // For outlier check
			OutcomeP75:           0.10, // For outlier check
			OutcomeP90:           0.15,
			MaxDrawdown:          0.08,
			MaxConsecutiveLosses: 3,
		},
		// TIME_EXIT, degraded, NEW_TOKEN
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioDegraded,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          100,
			TotalTokens:          80,
			WinRate:              0.08,
			TokenWinRate:         0.06,
			OutcomeMean:          0.04, // > 0, ratio = 0.04/0.065 = 0.62 >= 0.5
			OutcomeMedian:        0.03,
			OutcomeP10:           -0.03,
			OutcomeP25:           0.01,
			OutcomeP75:           0.06,
			OutcomeP90:           0.10,
			MaxDrawdown:          0.10,
			MaxConsecutiveLosses: 4,
		},
		// TIME_EXIT, pessimistic, NEW_TOKEN
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioPessimistic,
			EntryEventType:       "NEW_TOKEN",
			TotalTrades:          100,
			TotalTokens:          80,
			WinRate:              0.06,
			TokenWinRate:         0.05,
			OutcomeMean:          0.035,
			OutcomeMedian:        0.03, // > 0, ratio = 0.03/0.05 = 0.6 >= 0.5 (GO case)
			OutcomeP10:           -0.05,
			OutcomeP25:           0.01, // P25 > 0 for outlier check pass
			OutcomeP75:           0.06,
			OutcomeP90:           0.08,
			MaxDrawdown:          0.12,
			MaxConsecutiveLosses: 5,
		},
		// TIME_EXIT, realistic, ACTIVE_TOKEN - marginal metrics
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioRealistic,
			EntryEventType:       "ACTIVE_TOKEN",
			TotalTrades:          50,
			TotalTokens:          40,
			WinRate:              0.06, // 6% > 5% (trade-level)
			TokenWinRate:         0.06, // 6% > 5% (token-level)
			OutcomeMean:          0.03,
			OutcomeMedian:        0.02,
			OutcomeP10:           -0.04,
			OutcomeP25:           0.01,
			OutcomeP75:           0.05,
			OutcomeP90:           0.08,
			MaxDrawdown:          0.06,
			MaxConsecutiveLosses: 4,
		},
		// TIME_EXIT, degraded, ACTIVE_TOKEN
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioDegraded,
			EntryEventType:       "ACTIVE_TOKEN",
			TotalTrades:          50,
			TotalTokens:          40,
			WinRate:              0.04,
			TokenWinRate:         0.04,
			OutcomeMean:          0.015, // ratio = 0.015/0.03 = 0.5 >= 0.5
			OutcomeMedian:        0.01,
			OutcomeP10:           -0.05,
			OutcomeP25:           0.005,
			OutcomeP75:           0.03,
			OutcomeP90:           0.05,
			MaxDrawdown:          0.08,
			MaxConsecutiveLosses: 5,
		},
		// TIME_EXIT, pessimistic, ACTIVE_TOKEN
		{
			StrategyID:           "TIME_EXIT",
			ScenarioID:           domain.ScenarioPessimistic,
			EntryEventType:       "ACTIVE_TOKEN",
			TotalTrades:          50,
			TotalTokens:          40,
			WinRate:              0.03,
			TokenWinRate:         0.03,
			OutcomeMean:          0.015,
			OutcomeMedian:        0.012, // ratio = 0.012/0.02 = 0.6 >= 0.5
			OutcomeP10:           -0.06,
			OutcomeP25:           0.005,
			OutcomeP75:           0.03,
			OutcomeP90:           0.04,
			MaxDrawdown:          0.10,
			MaxConsecutiveLosses: 6,
		},
	}

	for _, a := range aggregates {
		if err := store.Insert(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

// LoadSwapsAndLiquidity populates swap and liquidity event stores with test data.
// Required for sufficiency check #5 (missing events check).
func LoadSwapsAndLiquidity(
	ctx context.Context,
	swapStore storage.SwapStore,
	liquidityStore storage.LiquidityEventStore,
) error {
	if err := loadSwaps(ctx, swapStore); err != nil {
		return err
	}
	if err := loadLiquidityEvents(ctx, liquidityStore); err != nil {
		return err
	}
	return nil
}

func loadSwaps(ctx context.Context, store storage.SwapStore) error {
	// At least one swap per candidate (cand_001, cand_002, cand_003)
	swaps := []*domain.Swap{
		{
			CandidateID: "cand_001",
			TxSignature: "swap_tx_001",
			EventIndex:  0,
			Slot:        100,
			Timestamp:   1704067200000, // 2024-01-01 00:00:00 UTC
			Side:        domain.SwapSideBuy,
			AmountIn:    1.0,
			AmountOut:   100.0,
			Price:       0.01,
		},
		{
			CandidateID: "cand_002",
			TxSignature: "swap_tx_002",
			EventIndex:  0,
			Slot:        101,
			Timestamp:   1704153600000, // 2024-01-02 00:00:00 UTC
			Side:        domain.SwapSideBuy,
			AmountIn:    2.0,
			AmountOut:   200.0,
			Price:       0.01,
		},
		{
			CandidateID: "cand_003",
			TxSignature: "swap_tx_003",
			EventIndex:  0,
			Slot:        102,
			Timestamp:   1704240000000, // 2024-01-03 00:00:00 UTC
			Side:        domain.SwapSideBuy,
			AmountIn:    1.5,
			AmountOut:   150.0,
			Price:       0.01,
		},
	}

	for _, s := range swaps {
		if err := store.Insert(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func loadLiquidityEvents(ctx context.Context, store storage.LiquidityEventStore) error {
	// At least one liquidity event per candidate (cand_001, cand_002, cand_003)
	events := []*domain.LiquidityEvent{
		{
			CandidateID:    "cand_001",
			TxSignature:    "liq_tx_001",
			EventIndex:     0,
			Slot:           100,
			Timestamp:      1704067200000, // 2024-01-01 00:00:00 UTC
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    10000.0,
			AmountQuote:    100.0,
			LiquidityAfter: 10100.0,
		},
		{
			CandidateID:    "cand_002",
			TxSignature:    "liq_tx_002",
			EventIndex:     0,
			Slot:           101,
			Timestamp:      1704153600000, // 2024-01-02 00:00:00 UTC
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    20000.0,
			AmountQuote:    200.0,
			LiquidityAfter: 20200.0,
		},
		{
			CandidateID:    "cand_003",
			TxSignature:    "liq_tx_003",
			EventIndex:     0,
			Slot:           102,
			Timestamp:      1704240000000, // 2024-01-03 00:00:00 UTC
			EventType:      domain.LiquidityEventAdd,
			AmountToken:    15000.0,
			AmountQuote:    150.0,
			LiquidityAfter: 15150.0,
		},
	}

	for _, e := range events {
		if err := store.Insert(ctx, e); err != nil {
			return err
		}
	}
	return nil
}
