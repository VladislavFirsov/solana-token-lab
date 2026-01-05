package metrics

import (
	"math"
	"sort"

	"solana-token-lab/internal/domain"
)

// computeFromTrades calculates all metrics from a slice of trades.
// Trades must be pre-filtered by (strategy_id, scenario_id, entry_event_type).
// Trades are sorted by EntrySignalTime ASC, TradeID ASC before computing
// order-dependent metrics (MaxDrawdown, MaxConsecutiveLosses).
func computeFromTrades(trades []*domain.TradeRecord, entryEventType string) *domain.StrategyAggregate {
	n := len(trades)
	if n == 0 {
		return &domain.StrategyAggregate{
			EntryEventType: entryEventType,
		}
	}

	// Sort trades deterministically by EntrySignalTime ASC, TradeID ASC
	sortedTrades := make([]*domain.TradeRecord, n)
	copy(sortedTrades, trades)
	sort.Slice(sortedTrades, func(i, j int) bool {
		if sortedTrades[i].EntrySignalTime != sortedTrades[j].EntrySignalTime {
			return sortedTrades[i].EntrySignalTime < sortedTrades[j].EntrySignalTime
		}
		return sortedTrades[i].TradeID < sortedTrades[j].TradeID
	})

	// Count wins/losses
	wins := 0
	losses := 0
	for _, t := range sortedTrades {
		if t.OutcomeClass == domain.OutcomeClassWin {
			wins++
		} else {
			losses++
		}
	}

	// Extract outcomes in sorted order for order-dependent calculations
	outcomes := make([]float64, n)
	for i, t := range sortedTrades {
		outcomes[i] = t.Outcome
	}

	// Sort outcomes for percentile calculations
	sortedOutcomes := make([]float64, n)
	copy(sortedOutcomes, outcomes)
	sort.Float64s(sortedOutcomes)

	// Compute statistics
	mean := computeMean(outcomes)
	stddev := computeStddev(outcomes, mean)

	// Compute token-level win rate
	totalTokens, tokenWinRate := computeTokenWinRate(sortedTrades)

	agg := &domain.StrategyAggregate{
		EntryEventType: entryEventType,

		// Counts
		TotalTrades:  n,
		TotalTokens:  totalTokens,
		Wins:         wins,
		Losses:       losses,
		WinRate:      computeWinRate(wins, n),
		TokenWinRate: tokenWinRate,

		// Outcome Distribution
		OutcomeMean:   mean,
		OutcomeMedian: computePercentile(sortedOutcomes, 0.50),
		OutcomeP10:    computePercentile(sortedOutcomes, 0.10),
		OutcomeP25:    computePercentile(sortedOutcomes, 0.25),
		OutcomeP75:    computePercentile(sortedOutcomes, 0.75),
		OutcomeP90:    computePercentile(sortedOutcomes, 0.90),
		OutcomeMin:    sortedOutcomes[0],
		OutcomeMax:    sortedOutcomes[n-1],
		OutcomeStddev: stddev,

		// Drawdown (order-dependent, uses sortedTrades order)
		MaxDrawdown:          computeMaxDrawdown(outcomes),
		MaxConsecutiveLosses: computeMaxConsecutiveLosses(sortedTrades),
	}

	return agg
}

// computeTokenWinRate calculates token-level win rate.
// Groups trades by CandidateID, checks if at least one trade has positive outcome,
// returns (totalTokens, tokensWithPositiveOutcome / totalTokens).
// Per MVP_CRITERIA.md: a token is considered "winning" if it has at least one positive outcome.
func computeTokenWinRate(trades []*domain.TradeRecord) (int, float64) {
	if len(trades) == 0 {
		return 0, 0
	}

	// Group outcomes by candidate_id
	candidateOutcomes := make(map[string][]float64)
	for _, t := range trades {
		candidateOutcomes[t.CandidateID] = append(candidateOutcomes[t.CandidateID], t.Outcome)
	}

	totalTokens := len(candidateOutcomes)
	tokensWithPositiveOutcome := 0

	for _, outcomes := range candidateOutcomes {
		// Check if at least one outcome is positive (not mean)
		hasPositive := false
		for _, outcome := range outcomes {
			if outcome > 0 {
				hasPositive = true
				break
			}
		}
		if hasPositive {
			tokensWithPositiveOutcome++
		}
	}

	return totalTokens, float64(tokensWithPositiveOutcome) / float64(totalTokens)
}

// computeWinRate calculates win rate as wins / total.
func computeWinRate(wins, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(wins) / float64(total)
}

// computeMean calculates arithmetic mean of outcomes.
func computeMean(outcomes []float64) float64 {
	if len(outcomes) == 0 {
		return 0
	}
	sum := 0.0
	for _, o := range outcomes {
		sum += o
	}
	return sum / float64(len(outcomes))
}

// computeStddev calculates sample standard deviation (n-1 denominator).
// Per spec: uses sample formula for unbiased estimator.
func computeStddev(outcomes []float64, mean float64) float64 {
	n := len(outcomes)
	if n < 2 {
		return 0 // Need at least 2 samples for sample stddev
	}
	sumSq := 0.0
	for _, o := range outcomes {
		diff := o - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(n-1))
}

// computePercentile uses linear interpolation.
// sorted must be pre-sorted ASC.
// p is percentile (0.10 = 10th percentile).
func computePercentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	// Index for percentile (0-based, continuous)
	idx := p * float64(n-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}

	// Linear interpolation
	frac := idx - float64(lower)
	return sorted[lower] + frac*(sorted[upper]-sorted[lower])
}

// computeMaxDrawdown calculates worst peak-to-trough on cumulative outcomes.
// max_drawdown = MAX(peak_cumulative - trough_cumulative)
// Outcomes must be in chronological order.
func computeMaxDrawdown(outcomes []float64) float64 {
	if len(outcomes) == 0 {
		return 0
	}

	cumulative := 0.0
	peak := 0.0
	maxDrawdown := 0.0

	for _, o := range outcomes {
		cumulative += o
		if cumulative > peak {
			peak = cumulative
		}
		drawdown := peak - cumulative
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}
	}
	return maxDrawdown
}

// computeMaxConsecutiveLosses finds longest streak of outcome <= 0.
// Trades must be in chronological order.
func computeMaxConsecutiveLosses(trades []*domain.TradeRecord) int {
	maxStreak := 0
	currentStreak := 0

	for _, t := range trades {
		if t.Outcome <= 0 {
			currentStreak++
			if currentStreak > maxStreak {
				maxStreak = currentStreak
			}
		} else {
			currentStreak = 0
		}
	}
	return maxStreak
}

// setSensitivityFields sets OutcomeRealistic/Pessimistic/Degraded based on scenarioID.
func setSensitivityFields(agg *domain.StrategyAggregate) {
	switch agg.ScenarioID {
	case domain.ScenarioRealistic:
		agg.OutcomeRealistic = &agg.OutcomeMean
	case domain.ScenarioPessimistic:
		agg.OutcomePessimistic = &agg.OutcomeMean
	case domain.ScenarioDegraded:
		agg.OutcomeDegraded = &agg.OutcomeMean
	}
}
