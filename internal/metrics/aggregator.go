package metrics

import (
	"context"
	"errors"

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
}

// NewAggregator creates a new metrics aggregator.
func NewAggregator(tradeStore storage.TradeRecordStore, aggStore storage.StrategyAggregateStore, candidateStore storage.CandidateStore) *Aggregator {
	return &Aggregator{
		tradeRecordStore: tradeStore,
		strategyAggStore: aggStore,
		candidateStore:   candidateStore,
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
func (a *Aggregator) filterByEntryEventType(ctx context.Context, trades []*domain.TradeRecord, entryEventType string) ([]*domain.TradeRecord, error) {
	var filtered []*domain.TradeRecord

	for _, trade := range trades {
		candidate, err := a.candidateStore.GetByID(ctx, trade.CandidateID)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				// Skip trades with missing candidates
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
