package pipeline

import (
	"context"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// LoadFixtures populates stores with test data for Phase 1 demonstration.
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
			WinRate:              0.12, // 12% > 5%
			OutcomeMean:          0.065,
			OutcomeMedian:        0.05, // > 0
			OutcomeP10:           -0.02,
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
			WinRate:              0.08,
			OutcomeMean:          0.04, // > 0, ratio = 0.04/0.065 = 0.62 >= 0.5
			OutcomeMedian:        0.03,
			OutcomeP10:           -0.03,
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
			WinRate:              0.06,
			OutcomeMean:          0.02,
			OutcomeMedian:        0.01,
			OutcomeP10:           -0.05,
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
			WinRate:              0.06, // 6% > 5%
			OutcomeMean:          0.03,
			OutcomeMedian:        0.02,
			OutcomeP10:           -0.04,
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
			WinRate:              0.04,
			OutcomeMean:          0.015, // ratio = 0.015/0.03 = 0.5 >= 0.5
			OutcomeMedian:        0.01,
			OutcomeP10:           -0.05,
			OutcomeP90:           0.05,
			MaxDrawdown:          0.08,
			MaxConsecutiveLosses: 5,
		},
	}

	for _, a := range aggregates {
		if err := store.Insert(ctx, a); err != nil {
			return err
		}
	}
	return nil
}
