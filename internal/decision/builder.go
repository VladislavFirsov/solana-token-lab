package decision

import (
	"errors"
	"sort"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/reporting"
)

// ErrNoRealisticScenario is returned when no realistic scenario data is found.
var ErrNoRealisticScenario = errors.New("no realistic scenario data found")

// StrategyKey identifies a strategy for implementability lookup.
type StrategyKey struct {
	StrategyID     string
	EntryEventType string
}

// Builder constructs DecisionInput from Report.
type Builder struct {
	// implementable maps strategy keys to implementability status.
	// Must be set explicitly - we don't infer from TotalTrades.
	implementable map[StrategyKey]bool
}

// NewBuilder creates a new decision input builder.
// implementable maps (StrategyID, EntryEventType) -> whether strategy is implementable.
func NewBuilder(implementable map[StrategyKey]bool) *Builder {
	return &Builder{implementable: implementable}
}

// ErrStrategyNotFound is returned when the specified strategy is not in the report.
var ErrStrategyNotFound = errors.New("strategy not found in report")

// Build creates DecisionInput for a specific strategy from reporting.Report.
// Uses REALISTIC scenario aggregates only.
// PositiveOutcomePct = WinRate * 100
// MedianOutcome = OutcomeMedian (realistic)
// RealisticMean from realistic aggregate; DegradedMean from degraded aggregate for same strategy/entry type.
// strategyID and entryEventType must be specified explicitly.
func (b *Builder) Build(report *reporting.Report, strategyID, entryEventType string) (*DecisionInput, error) {
	// Find specific realistic scenario metric
	var realisticMetric *reporting.StrategyMetricRow
	for i := range report.StrategyMetrics {
		m := &report.StrategyMetrics[i]
		if m.ScenarioID == domain.ScenarioRealistic &&
			m.StrategyID == strategyID &&
			m.EntryEventType == entryEventType {
			realisticMetric = m
			break
		}
	}

	if realisticMetric == nil {
		return nil, ErrStrategyNotFound
	}

	// Find corresponding degraded scenario for stability check
	var degradedMean float64
	for _, m := range report.StrategyMetrics {
		if m.StrategyID == strategyID &&
			m.EntryEventType == entryEventType &&
			m.ScenarioID == domain.ScenarioDegraded {
			degradedMean = m.OutcomeMean
			break
		}
	}

	// Look up implementability from explicit map
	key := StrategyKey{StrategyID: strategyID, EntryEventType: entryEventType}
	implementable := b.implementable[key] // defaults to false if not in map

	// Build DecisionInput
	input := &DecisionInput{
		PositiveOutcomePct: realisticMetric.WinRate * 100, // WinRate is 0-1, convert to percentage
		MedianOutcome:      realisticMetric.OutcomeMedian,
		RealisticMean:      realisticMetric.OutcomeMean,
		DegradedMean:       degradedMean,
		OutcomeP10:         realisticMetric.OutcomeP10,
		OutcomeP50:         realisticMetric.OutcomeMedian, // P50 = median
		OutcomeP90:         realisticMetric.OutcomeP90,

		// Strategy implementability from explicit map
		StrategyImplementable: implementable,

		// Context
		StrategyID:     realisticMetric.StrategyID,
		EntryEventType: realisticMetric.EntryEventType,
		ScenarioID:     realisticMetric.ScenarioID,
	}

	return input, nil
}

// BuildAll creates DecisionInput for each realistic scenario in the report.
// Returns a slice of inputs, one per (strategy_id, entry_event_type) combination.
func (b *Builder) BuildAll(report *reporting.Report) ([]*DecisionInput, error) {
	realisticMetrics := make(map[StrategyKey]*reporting.StrategyMetricRow)
	degradedMetrics := make(map[StrategyKey]*reporting.StrategyMetricRow)

	for i := range report.StrategyMetrics {
		m := &report.StrategyMetrics[i]
		k := StrategyKey{StrategyID: m.StrategyID, EntryEventType: m.EntryEventType}

		switch m.ScenarioID {
		case domain.ScenarioRealistic:
			realisticMetrics[k] = m
		case domain.ScenarioDegraded:
			degradedMetrics[k] = m
		}
	}

	if len(realisticMetrics) == 0 {
		return nil, ErrNoRealisticScenario
	}

	// Extract and sort keys for deterministic output order
	keys := make([]StrategyKey, 0, len(realisticMetrics))
	for k := range realisticMetrics {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].StrategyID != keys[j].StrategyID {
			return keys[i].StrategyID < keys[j].StrategyID
		}
		return keys[i].EntryEventType < keys[j].EntryEventType
	})

	inputs := make([]*DecisionInput, 0, len(keys))
	for _, k := range keys {
		realistic := realisticMetrics[k]
		degradedMean := 0.0
		if degraded, ok := degradedMetrics[k]; ok {
			degradedMean = degraded.OutcomeMean
		}

		// Look up implementability from explicit map
		implementable := b.implementable[k] // defaults to false if not in map

		input := &DecisionInput{
			PositiveOutcomePct:    realistic.WinRate * 100,
			MedianOutcome:         realistic.OutcomeMedian,
			RealisticMean:         realistic.OutcomeMean,
			DegradedMean:          degradedMean,
			OutcomeP10:            realistic.OutcomeP10,
			OutcomeP50:            realistic.OutcomeMedian,
			OutcomeP90:            realistic.OutcomeP90,
			StrategyImplementable: implementable,
			StrategyID:            realistic.StrategyID,
			EntryEventType:        realistic.EntryEventType,
			ScenarioID:            realistic.ScenarioID,
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}
