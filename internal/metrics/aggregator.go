package metrics

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// ErrNoTrades is returned when no trades are available for aggregation.
var ErrNoTrades = errors.New("no trades available for aggregation")

// Aggregator computes strategy aggregates from trade records.
type Aggregator struct {
	tradeRecordStore storage.TradeRecordStore
	strategyAggStore storage.StrategyAggregateStore
	candidateStore   storage.CandidateStore

	// MissingCandidates tracks trade_ids with missing candidates (for data quality reporting).
	// Key: candidate_id, Value: count of trades referencing it.
	MissingCandidates map[string]int
}

// NewAggregator creates a new metrics aggregator.
func NewAggregator(tradeStore storage.TradeRecordStore, aggStore storage.StrategyAggregateStore, candidateStore storage.CandidateStore) *Aggregator {
	return &Aggregator{
		tradeRecordStore:  tradeStore,
		strategyAggStore:  aggStore,
		candidateStore:    candidateStore,
		MissingCandidates: make(map[string]int),
	}
}

// ComputeAggregate computes aggregate for a specific (strategy_id, scenario_id, entry_event_type).
// Loads trades matching the key, filters by candidate source, computes all metrics, returns aggregate.
// Returns ErrNoTrades if no trades match the criteria.
func (a *Aggregator) ComputeAggregate(ctx context.Context, strategyID, scenarioID, entryEventType string) (*domain.StrategyAggregate, error) {
	// Load all trades for strategy/scenario combination
	trades, err := a.tradeRecordStore.GetByStrategyScenario(ctx, strategyID, scenarioID)
	if err != nil {
		return nil, err
	}

	// Filter trades by entry_event_type using candidate source
	filteredTrades, err := a.filterByEntryEventType(ctx, trades, entryEventType)
	if err != nil {
		return nil, err
	}

	if len(filteredTrades) == 0 {
		return nil, ErrNoTrades
	}

	// Compute aggregate from filtered trades
	agg := computeFromTrades(filteredTrades, entryEventType)

	// Set strategy and scenario IDs
	agg.StrategyID = strategyID
	agg.ScenarioID = scenarioID

	// Set sensitivity fields based on scenario
	setSensitivityFields(agg)

	return agg, nil
}

// filterByEntryEventType filters trades by matching candidate source to entry event type.
// Tracks missing candidates in a.MissingCandidates instead of silently skipping.
func (a *Aggregator) filterByEntryEventType(ctx context.Context, trades []*domain.TradeRecord, entryEventType string) ([]*domain.TradeRecord, error) {
	var filtered []*domain.TradeRecord

	for _, trade := range trades {
		candidate, err := a.candidateStore.GetByID(ctx, trade.CandidateID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// Record missing candidate (don't silently skip)
				a.MissingCandidates[trade.CandidateID]++
				continue
			}
			return nil, err
		}

		// Match candidate source to entry event type
		if sourceMatchesEntryEventType(candidate.Source, entryEventType) {
			filtered = append(filtered, trade)
		}
	}

	return filtered, nil
}

// GetMissingCandidateErrors returns data quality errors for missing candidates.
// Returns slice of error messages sorted by candidate_id for deterministic output.
func (a *Aggregator) GetMissingCandidateErrors() []string {
	if len(a.MissingCandidates) == 0 {
		return nil
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(a.MissingCandidates))
	for k := range a.MissingCandidates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	errors := make([]string, len(keys))
	for i, candidateID := range keys {
		count := a.MissingCandidates[candidateID]
		errors[i] = fmt.Sprintf("missing candidate %s referenced by %d trade(s)", candidateID, count)
	}
	return errors
}

// sourceMatchesEntryEventType checks if candidate source matches entry event type.
func sourceMatchesEntryEventType(source domain.Source, entryEventType string) bool {
	switch entryEventType {
	case "NEW_TOKEN":
		return source == domain.SourceNewToken
	case "ACTIVE_TOKEN":
		return source == domain.SourceActiveToken
	default:
		return false
	}
}

// ComputeAndStore computes and persists aggregate.
// Returns storage.ErrDuplicateKey if aggregate already exists (append-only).
func (a *Aggregator) ComputeAndStore(ctx context.Context, strategyID, scenarioID, entryEventType string) (*domain.StrategyAggregate, error) {
	agg, err := a.ComputeAggregate(ctx, strategyID, scenarioID, entryEventType)
	if err != nil {
		return nil, err
	}

	// Persist aggregate (append-only, returns ErrDuplicateKey on duplicate)
	if err := a.strategyAggStore.Insert(ctx, agg); err != nil {
		return nil, err
	}

	return agg, nil
}
