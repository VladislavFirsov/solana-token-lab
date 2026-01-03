package storage

import (
	"context"

	"solana-token-lab/internal/domain"
)

// CandidateStore provides access to token_candidates storage.
type CandidateStore interface {
	// Insert adds a new candidate. Returns ErrDuplicateKey if candidate_id exists.
	Insert(ctx context.Context, c *domain.TokenCandidate) error

	// GetByID retrieves a candidate by its ID. Returns ErrNotFound if not exists.
	GetByID(ctx context.Context, candidateID string) (*domain.TokenCandidate, error)

	// GetByMint retrieves all candidates for a given mint address.
	GetByMint(ctx context.Context, mint string) ([]*domain.TokenCandidate, error)

	// GetByTimeRange retrieves candidates discovered within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, start, end int64) ([]*domain.TokenCandidate, error)

	// GetBySource retrieves all candidates of a given source type.
	GetBySource(ctx context.Context, source domain.Source) ([]*domain.TokenCandidate, error)
}

// SwapStore provides access to swaps storage.
type SwapStore interface {
	// Insert adds a new swap. Returns ErrDuplicateKey if (candidate_id, tx_signature, event_index) exists.
	Insert(ctx context.Context, s *domain.Swap) error

	// InsertBulk adds multiple swaps atomically. Fails entire batch on any duplicate.
	InsertBulk(ctx context.Context, swaps []*domain.Swap) error

	// GetByCandidateID retrieves all swaps for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.Swap, error)

	// GetByTimeRange retrieves swaps for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.Swap, error)
}

// LiquidityEventStore provides access to liquidity_events storage.
type LiquidityEventStore interface {
	// Insert adds a new liquidity event. Returns ErrDuplicateKey if exists.
	Insert(ctx context.Context, e *domain.LiquidityEvent) error

	// InsertBulk adds multiple events atomically. Fails entire batch on any duplicate.
	InsertBulk(ctx context.Context, events []*domain.LiquidityEvent) error

	// GetByCandidateID retrieves all events for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.LiquidityEvent, error)

	// GetByTimeRange retrieves events for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.LiquidityEvent, error)
}

// TokenMetadataStore provides access to token_metadata storage.
type TokenMetadataStore interface {
	// Insert adds new metadata. Returns ErrDuplicateKey if candidate_id exists.
	Insert(ctx context.Context, m *domain.TokenMetadata) error

	// GetByID retrieves metadata by candidate ID. Returns ErrNotFound if not exists.
	GetByID(ctx context.Context, candidateID string) (*domain.TokenMetadata, error)

	// GetByMint retrieves metadata by mint address. Returns ErrNotFound if not exists.
	GetByMint(ctx context.Context, mint string) (*domain.TokenMetadata, error)
}

// PriceTimeseriesStore provides access to price_timeseries storage.
type PriceTimeseriesStore interface {
	// InsertBulk adds multiple points. Fails entire batch on duplicate (candidate_id, timestamp_ms).
	InsertBulk(ctx context.Context, points []*domain.PriceTimeseriesPoint) error

	// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.PriceTimeseriesPoint, error)

	// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.PriceTimeseriesPoint, error)
}

// LiquidityTimeseriesStore provides access to liquidity_timeseries storage.
type LiquidityTimeseriesStore interface {
	// InsertBulk adds multiple points. Fails entire batch on duplicate.
	InsertBulk(ctx context.Context, points []*domain.LiquidityTimeseriesPoint) error

	// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.LiquidityTimeseriesPoint, error)

	// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.LiquidityTimeseriesPoint, error)
}

// VolumeTimeseriesStore provides access to volume_timeseries storage.
type VolumeTimeseriesStore interface {
	// InsertBulk adds multiple points. Fails entire batch on duplicate.
	InsertBulk(ctx context.Context, points []*domain.VolumeTimeseriesPoint) error

	// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.VolumeTimeseriesPoint, error)

	// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.VolumeTimeseriesPoint, error)
}

// DerivedFeatureStore provides access to derived_features storage.
type DerivedFeatureStore interface {
	// InsertBulk adds multiple points. Fails entire batch on duplicate.
	InsertBulk(ctx context.Context, points []*domain.DerivedFeaturePoint) error

	// GetByCandidateID retrieves all points for a candidate, ordered by timestamp ASC.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.DerivedFeaturePoint, error)

	// GetByTimeRange retrieves points for a candidate within [start, end] (inclusive).
	GetByTimeRange(ctx context.Context, candidateID string, start, end int64) ([]*domain.DerivedFeaturePoint, error)
}

// TradeRecordStore provides access to trade_records storage.
type TradeRecordStore interface {
	// Insert adds a new trade. Returns ErrDuplicateKey if trade_id exists.
	Insert(ctx context.Context, t *domain.TradeRecord) error

	// InsertBulk adds multiple trades atomically. Fails entire batch on any duplicate.
	InsertBulk(ctx context.Context, trades []*domain.TradeRecord) error

	// GetByID retrieves a trade by its ID. Returns ErrNotFound if not exists.
	GetByID(ctx context.Context, tradeID string) (*domain.TradeRecord, error)

	// GetByCandidateID retrieves all trades for a candidate.
	GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.TradeRecord, error)

	// GetByStrategyScenario retrieves all trades for a strategy/scenario combination.
	GetByStrategyScenario(ctx context.Context, strategyID, scenarioID string) ([]*domain.TradeRecord, error)
}

// StrategyAggregateStore provides access to strategy_aggregates storage.
type StrategyAggregateStore interface {
	// Insert adds a new aggregate. Returns ErrDuplicateKey if key exists.
	Insert(ctx context.Context, a *domain.StrategyAggregate) error

	// InsertBulk adds multiple aggregates atomically. Fails entire batch on any duplicate.
	InsertBulk(ctx context.Context, aggregates []*domain.StrategyAggregate) error

	// GetByKey retrieves an aggregate by its composite key.
	GetByKey(ctx context.Context, strategyID, scenarioID, entryEventType string) (*domain.StrategyAggregate, error)

	// GetByStrategy retrieves all aggregates for a strategy.
	GetByStrategy(ctx context.Context, strategyID string) ([]*domain.StrategyAggregate, error)

	// GetAll retrieves all aggregates.
	GetAll(ctx context.Context) ([]*domain.StrategyAggregate, error)
}
