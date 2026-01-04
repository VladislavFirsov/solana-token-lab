package reporting

import (
	"context"
	"sort"
	"time"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// Generator produces reports from stored data.
type Generator struct {
	candidateStore   storage.CandidateStore
	tradeRecordStore storage.TradeRecordStore
	aggregateStore   storage.StrategyAggregateStore
	now              func() time.Time // Injectable clock for deterministic output
}

// NewGenerator creates a new report generator.
func NewGenerator(
	candidateStore storage.CandidateStore,
	tradeStore storage.TradeRecordStore,
	aggStore storage.StrategyAggregateStore,
) *Generator {
	return &Generator{
		candidateStore:   candidateStore,
		tradeRecordStore: tradeStore,
		aggregateStore:   aggStore,
		now:              func() time.Time { return time.Now().UTC() },
	}
}

// WithClock sets a custom clock function for deterministic output.
func (g *Generator) WithClock(now func() time.Time) *Generator {
	g.now = now
	return g
}

// Generate produces a complete Phase 1 report.
func (g *Generator) Generate(ctx context.Context) (*Report, error) {
	// Load all aggregates
	aggs, err := g.aggregateStore.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	// Generate data summary
	dataSummary, err := g.generateDataSummary(ctx, aggs)
	if err != nil {
		return nil, err
	}

	// Generate strategy metrics
	metrics := g.generateStrategyMetrics(aggs)

	// Generate source comparison
	sourceComparison := g.generateSourceComparison(aggs)

	// Generate scenario sensitivity
	sensitivity := g.generateScenarioSensitivity(aggs)

	// Generate replay references
	replayRefs, err := g.generateReplayReferences(ctx, aggs)
	if err != nil {
		return nil, err
	}

	// Count unique strategies and scenarios
	strategySet := make(map[string]struct{})
	scenarioSet := make(map[string]struct{})
	for _, agg := range aggs {
		strategySet[agg.StrategyID] = struct{}{}
		scenarioSet[agg.ScenarioID] = struct{}{}
	}

	return &Report{
		GeneratedAt:         g.now(),
		StrategyCount:       len(strategySet),
		ScenarioCount:       len(scenarioSet),
		DataSummary:         *dataSummary,
		StrategyMetrics:     metrics,
		SourceComparison:    sourceComparison,
		ScenarioSensitivity: sensitivity,
		ReplayReferences:    replayRefs,
	}, nil
}

// generateDataSummary computes data summary from candidates and aggregates.
func (g *Generator) generateDataSummary(ctx context.Context, aggs []*domain.StrategyAggregate) (*DataSummary, error) {
	// Load candidates by source
	newTokenCandidates, err := g.candidateStore.GetBySource(ctx, domain.SourceNewToken)
	if err != nil {
		return nil, err
	}

	activeTokenCandidates, err := g.candidateStore.GetBySource(ctx, domain.SourceActiveToken)
	if err != nil {
		return nil, err
	}

	// Sum total trades from aggregates
	totalTrades := 0
	for _, agg := range aggs {
		totalTrades += agg.TotalTrades
	}

	// Find date range from candidates
	var dateRangeStart, dateRangeEnd int64
	allCandidates := append(newTokenCandidates, activeTokenCandidates...)
	if len(allCandidates) > 0 {
		dateRangeStart = allCandidates[0].DiscoveredAt
		dateRangeEnd = allCandidates[0].DiscoveredAt
		for _, c := range allCandidates {
			if c.DiscoveredAt < dateRangeStart {
				dateRangeStart = c.DiscoveredAt
			}
			if c.DiscoveredAt > dateRangeEnd {
				dateRangeEnd = c.DiscoveredAt
			}
		}
	}

	return &DataSummary{
		TotalCandidates:       len(newTokenCandidates) + len(activeTokenCandidates),
		NewTokenCandidates:    len(newTokenCandidates),
		ActiveTokenCandidates: len(activeTokenCandidates),
		TotalTrades:           totalTrades,
		DateRangeStart:        dateRangeStart,
		DateRangeEnd:          dateRangeEnd,
	}, nil
}

// generateStrategyMetrics loads aggregates and builds sorted rows.
func (g *Generator) generateStrategyMetrics(aggs []*domain.StrategyAggregate) []StrategyMetricRow {
	rows := make([]StrategyMetricRow, len(aggs))
	for i, agg := range aggs {
		rows[i] = StrategyMetricRow{
			StrategyID:           agg.StrategyID,
			ScenarioID:           agg.ScenarioID,
			EntryEventType:       agg.EntryEventType,
			TotalTrades:          agg.TotalTrades,
			TotalTokens:          agg.TotalTokens,
			Wins:                 agg.Wins,
			Losses:               agg.Losses,
			WinRate:              agg.WinRate,
			TokenWinRate:         agg.TokenWinRate,
			OutcomeMean:          agg.OutcomeMean,
			OutcomeMedian:        agg.OutcomeMedian,
			OutcomeP10:           agg.OutcomeP10,
			OutcomeP25:           agg.OutcomeP25,
			OutcomeP75:           agg.OutcomeP75,
			OutcomeP90:           agg.OutcomeP90,
			OutcomeMin:           agg.OutcomeMin,
			OutcomeMax:           agg.OutcomeMax,
			OutcomeStddev:        agg.OutcomeStddev,
			MaxDrawdown:          agg.MaxDrawdown,
			MaxConsecutiveLosses: agg.MaxConsecutiveLosses,
		}
	}

	// Sort by (strategy_id, scenario_id, entry_event_type)
	sortStrategyMetrics(rows)
	return rows
}

// generateSourceComparison builds NEW_TOKEN vs ACTIVE_TOKEN comparison.
// Per REPORTING_SPEC.md: only Realistic scenario, with delta columns.
func (g *Generator) generateSourceComparison(aggs []*domain.StrategyAggregate) []SourceComparisonRow {
	// Group by strategy_id (Realistic scenario only)
	groups := make(map[string]map[string]*domain.StrategyAggregate)

	for _, agg := range aggs {
		// Filter to Realistic scenario only per REPORTING_SPEC.md
		if agg.ScenarioID != domain.ScenarioRealistic {
			continue
		}
		if groups[agg.StrategyID] == nil {
			groups[agg.StrategyID] = make(map[string]*domain.StrategyAggregate)
		}
		groups[agg.StrategyID][agg.EntryEventType] = agg
	}

	// Build comparison rows
	var rows []SourceComparisonRow
	for strategyID, entryTypes := range groups {
		newAgg := entryTypes["NEW_TOKEN"]
		activeAgg := entryTypes["ACTIVE_TOKEN"]

		// Only include if at least one exists
		if newAgg == nil && activeAgg == nil {
			continue
		}

		row := SourceComparisonRow{
			StrategyID: strategyID,
			ScenarioID: domain.ScenarioRealistic,
		}

		if newAgg != nil {
			row.NewTokenWinRate = newAgg.WinRate
			row.NewTokenMedian = newAgg.OutcomeMedian
		}
		if activeAgg != nil {
			row.ActiveTokenWinRate = activeAgg.WinRate
			row.ActiveTokenMedian = activeAgg.OutcomeMedian
		}

		// Calculate deltas (per REPORTING_SPEC.md)
		row.DeltaWinRate = row.NewTokenWinRate - row.ActiveTokenWinRate
		row.DeltaMedian = row.NewTokenMedian - row.ActiveTokenMedian

		rows = append(rows, row)
	}

	// Sort by strategy_id
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].StrategyID < rows[j].StrategyID
	})

	return rows
}

// generateScenarioSensitivity builds scenario sensitivity comparison.
// Per REPORTING_SPEC.md: uses median (not mean) and includes all 4 scenarios.
func (g *Generator) generateScenarioSensitivity(aggs []*domain.StrategyAggregate) []ScenarioSensitivityRow {
	// Group by (strategy_id, entry_event_type)
	type key struct {
		StrategyID     string
		EntryEventType string
	}
	groups := make(map[key]map[string]*domain.StrategyAggregate)

	for _, agg := range aggs {
		k := key{StrategyID: agg.StrategyID, EntryEventType: agg.EntryEventType}
		if groups[k] == nil {
			groups[k] = make(map[string]*domain.StrategyAggregate)
		}
		groups[k][agg.ScenarioID] = agg
	}

	// Build sensitivity rows using median (per REPORTING_SPEC.md)
	var rows []ScenarioSensitivityRow
	for k, scenarios := range groups {
		row := ScenarioSensitivityRow{
			StrategyID:     k.StrategyID,
			EntryEventType: k.EntryEventType,
		}

		// Use median instead of mean per REPORTING_SPEC.md
		if optimistic := scenarios[domain.ScenarioOptimistic]; optimistic != nil {
			row.OptimisticMedian = optimistic.OutcomeMedian
		}
		if realistic := scenarios[domain.ScenarioRealistic]; realistic != nil {
			row.RealisticMedian = realistic.OutcomeMedian
		}
		if pessimistic := scenarios[domain.ScenarioPessimistic]; pessimistic != nil {
			row.PessimisticMedian = pessimistic.OutcomeMedian
		}
		if degraded := scenarios[domain.ScenarioDegraded]; degraded != nil {
			row.DegradedMedian = degraded.OutcomeMedian
		}

		// Calculate degradation percentage: (realistic - pessimistic) / realistic * 100
		// Per DECISION_GATE.md: sensitivity uses Realistic vs Pessimistic
		if row.RealisticMedian != 0 {
			row.DegradationPct = (row.RealisticMedian - row.PessimisticMedian) / row.RealisticMedian * 100
		}

		rows = append(rows, row)
	}

	// Sort by (strategy_id, entry_event_type)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StrategyID != rows[j].StrategyID {
			return rows[i].StrategyID < rows[j].StrategyID
		}
		return rows[i].EntryEventType < rows[j].EntryEventType
	})

	return rows
}

// generateReplayReferences builds replay references from aggregates + trades.
func (g *Generator) generateReplayReferences(ctx context.Context, aggs []*domain.StrategyAggregate) ([]ReplayReferenceRow, error) {
	// Collect unique (strategy_id, scenario_id) pairs
	type key struct {
		StrategyID string
		ScenarioID string
	}
	seen := make(map[key]struct{})
	for _, agg := range aggs {
		seen[key{StrategyID: agg.StrategyID, ScenarioID: agg.ScenarioID}] = struct{}{}
	}

	// For each pair, load trades and collect candidate IDs
	var rows []ReplayReferenceRow
	candidateSeen := make(map[string]struct{})

	for k := range seen {
		trades, err := g.tradeRecordStore.GetByStrategyScenario(ctx, k.StrategyID, k.ScenarioID)
		if err != nil {
			return nil, err
		}

		for _, trade := range trades {
			refKey := k.StrategyID + "|" + k.ScenarioID + "|" + trade.CandidateID
			if _, exists := candidateSeen[refKey]; exists {
				continue
			}
			candidateSeen[refKey] = struct{}{}

			rows = append(rows, ReplayReferenceRow{
				StrategyID:  k.StrategyID,
				ScenarioID:  k.ScenarioID,
				CandidateID: trade.CandidateID,
			})
		}
	}

	// Sort by (strategy_id, scenario_id, candidate_id)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StrategyID != rows[j].StrategyID {
			return rows[i].StrategyID < rows[j].StrategyID
		}
		if rows[i].ScenarioID != rows[j].ScenarioID {
			return rows[i].ScenarioID < rows[j].ScenarioID
		}
		return rows[i].CandidateID < rows[j].CandidateID
	})

	return rows, nil
}

// sortStrategyMetrics sorts rows by (strategy_id, scenario_id, entry_event_type).
func sortStrategyMetrics(rows []StrategyMetricRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StrategyID != rows[j].StrategyID {
			return rows[i].StrategyID < rows[j].StrategyID
		}
		if rows[i].ScenarioID != rows[j].ScenarioID {
			return rows[i].ScenarioID < rows[j].ScenarioID
		}
		return rows[i].EntryEventType < rows[j].EntryEventType
	})
}
