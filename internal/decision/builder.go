package decision

import (
	"errors"
	"sort"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/reporting"
)

// ErrNoRealisticScenario is returned when no realistic scenario data is found.
var ErrNoRealisticScenario = errors.New("no realistic scenario data found")

// ErrMissingPessimisticScenario is returned when pessimistic scenario data is missing.
// Per spec, full scenario matrix is required for valid decision.
var ErrMissingPessimisticScenario = errors.New("missing pessimistic scenario data")

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
// PositiveOutcomePct = TokenWinRate * 100 (token-level, not trade-level)
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

	// Find corresponding pessimistic scenario for stability check (per DECISION_GATE.md)
	var pessimisticMean, pessimisticMedian float64
	for _, m := range report.StrategyMetrics {
		if m.StrategyID == strategyID &&
			m.EntryEventType == entryEventType &&
			m.ScenarioID == domain.ScenarioPessimistic {
			pessimisticMean = m.OutcomeMean
			pessimisticMedian = m.OutcomeMedian
			break
		}
	}

	// Also get degraded for backwards compatibility
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
		PositiveOutcomePct: realisticMetric.TokenWinRate * 100, // TokenWinRate is 0-1, convert to percentage (token-level)
		MedianOutcome:      realisticMetric.OutcomeMedian,
		RealisticMean:      realisticMetric.OutcomeMean,
		RealisticMedian:    realisticMetric.OutcomeMedian,
		PessimisticMean:    pessimisticMean,
		PessimisticMedian:  pessimisticMedian,
		DegradedMean:       degradedMean, // backwards compatibility
		OutcomeP10:         realisticMetric.OutcomeP10,
		OutcomeP25:         realisticMetric.OutcomeP25,
		OutcomeP50:         realisticMetric.OutcomeMedian, // P50 = median
		OutcomeP75:         realisticMetric.OutcomeP75,
		OutcomeP90:         realisticMetric.OutcomeP90,

		// Strategy implementability from explicit map
		StrategyImplementable: implementable,

		// Context
		StrategyID:     realisticMetric.StrategyID,
		EntryEventType: realisticMetric.EntryEventType,
		ScenarioID:     realisticMetric.ScenarioID,
	}

	// Validate before returning (fail fast)
	if err := input.Validate(); err != nil {
		return nil, err
	}

	return input, nil
}

// BuildAll creates DecisionInput for each realistic scenario in the report.
// Returns a slice of inputs, one per (strategy_id, entry_event_type) combination.
func (b *Builder) BuildAll(report *reporting.Report) ([]*DecisionInput, error) {
	realisticMetrics := make(map[StrategyKey]*reporting.StrategyMetricRow)
	pessimisticMetrics := make(map[StrategyKey]*reporting.StrategyMetricRow)
	degradedMetrics := make(map[StrategyKey]*reporting.StrategyMetricRow)

	for i := range report.StrategyMetrics {
		m := &report.StrategyMetrics[i]
		k := StrategyKey{StrategyID: m.StrategyID, EntryEventType: m.EntryEventType}

		switch m.ScenarioID {
		case domain.ScenarioRealistic:
			realisticMetrics[k] = m
		case domain.ScenarioPessimistic:
			pessimisticMetrics[k] = m
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

		// Get pessimistic metrics for stability check (per DECISION_GATE.md)
		// Missing pessimistic scenario is treated as insufficient data
		pessimistic, hasPessimistic := pessimisticMetrics[k]
		if !hasPessimistic {
			return nil, ErrMissingPessimisticScenario
		}
		pessimisticMean := pessimistic.OutcomeMean
		pessimisticMedian := pessimistic.OutcomeMedian

		// Get degraded for backwards compatibility
		var degradedMean float64
		if degraded, ok := degradedMetrics[k]; ok {
			degradedMean = degraded.OutcomeMean
		}

		// Look up implementability from explicit map
		implementable := b.implementable[k] // defaults to false if not in map

		input := &DecisionInput{
			PositiveOutcomePct:    realistic.TokenWinRate * 100, // TokenWinRate is 0-1, convert to percentage (token-level)
			MedianOutcome:         realistic.OutcomeMedian,
			RealisticMean:         realistic.OutcomeMean,
			RealisticMedian:       realistic.OutcomeMedian,
			PessimisticMean:       pessimisticMean,
			PessimisticMedian:     pessimisticMedian,
			DegradedMean:          degradedMean, // backwards compatibility
			OutcomeP10:            realistic.OutcomeP10,
			OutcomeP25:            realistic.OutcomeP25,
			OutcomeP50:            realistic.OutcomeMedian,
			OutcomeP75:            realistic.OutcomeP75,
			OutcomeP90:            realistic.OutcomeP90,
			StrategyImplementable: implementable,
			StrategyID:            realistic.StrategyID,
			EntryEventType:        realistic.EntryEventType,
			ScenarioID:            realistic.ScenarioID,
		}

		// Validate before adding (fail fast)
		if err := input.Validate(); err != nil {
			return nil, err
		}

		inputs = append(inputs, input)
	}

	return inputs, nil
}
