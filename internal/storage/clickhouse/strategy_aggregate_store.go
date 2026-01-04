package clickhouse

import (
	"context"
	"fmt"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// StrategyAggregateStore implements storage.StrategyAggregateStore using ClickHouse.
type StrategyAggregateStore struct {
	conn *Conn
}

// NewStrategyAggregateStore creates a new StrategyAggregateStore.
func NewStrategyAggregateStore(conn *Conn) *StrategyAggregateStore {
	return &StrategyAggregateStore{conn: conn}
}

// Compile-time interface check.
var _ storage.StrategyAggregateStore = (*StrategyAggregateStore)(nil)

// Insert adds a new aggregate. Returns ErrDuplicateKey if key exists.
func (s *StrategyAggregateStore) Insert(ctx context.Context, a *domain.StrategyAggregate) error {
	// Check if exists (ReplacingMergeTree will replace, but we want append-only semantics)
	exists, err := s.exists(ctx, a.StrategyID, a.ScenarioID, a.EntryEventType)
	if err != nil {
		return fmt.Errorf("check exists: %w", err)
	}
	if exists {
		return storage.ErrDuplicateKey
	}

	query := `
		INSERT INTO strategy_aggregates (
			strategy_id, scenario_id, entry_event_type,
			total_trades, total_tokens, wins, losses, win_rate, token_win_rate,
			outcome_mean, outcome_median, outcome_p10, outcome_p25, outcome_p75, outcome_p90,
			outcome_min, outcome_max, outcome_stddev,
			max_drawdown, max_consecutive_losses,
			outcome_realistic, outcome_pessimistic, outcome_degraded
		) VALUES (
			?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?, ?,
			?, ?, ?,
			?, ?,
			?, ?, ?
		)
	`

	err = s.conn.Exec(ctx, query,
		a.StrategyID, a.ScenarioID, a.EntryEventType,
		a.TotalTrades, a.TotalTokens, a.Wins, a.Losses, a.WinRate, a.TokenWinRate,
		a.OutcomeMean, a.OutcomeMedian, a.OutcomeP10, a.OutcomeP25, a.OutcomeP75, a.OutcomeP90,
		a.OutcomeMin, a.OutcomeMax, a.OutcomeStddev,
		a.MaxDrawdown, a.MaxConsecutiveLosses,
		a.OutcomeRealistic, a.OutcomePessimistic, a.OutcomeDegraded,
	)
	if err != nil {
		return fmt.Errorf("insert strategy aggregate: %w", err)
	}
	return nil
}

// InsertBulk adds multiple aggregates atomically. Fails entire batch on any duplicate.
func (s *StrategyAggregateStore) InsertBulk(ctx context.Context, aggregates []*domain.StrategyAggregate) error {
	if len(aggregates) == 0 {
		return nil
	}

	// Check for intra-batch duplicates
	seen := make(map[string]struct{})
	for _, a := range aggregates {
		key := a.StrategyID + "|" + a.ScenarioID + "|" + a.EntryEventType
		if _, exists := seen[key]; exists {
			return storage.ErrDuplicateKey
		}
		seen[key] = struct{}{}
	}

	// Check for duplicates against existing DB rows
	for _, a := range aggregates {
		exists, err := s.exists(ctx, a.StrategyID, a.ScenarioID, a.EntryEventType)
		if err != nil {
			return fmt.Errorf("check exists: %w", err)
		}
		if exists {
			return storage.ErrDuplicateKey
		}
	}

	// Use batch insert
	batch, err := s.conn.PrepareBatch(ctx, `
		INSERT INTO strategy_aggregates (
			strategy_id, scenario_id, entry_event_type,
			total_trades, total_tokens, wins, losses, win_rate, token_win_rate,
			outcome_mean, outcome_median, outcome_p10, outcome_p25, outcome_p75, outcome_p90,
			outcome_min, outcome_max, outcome_stddev,
			max_drawdown, max_consecutive_losses,
			outcome_realistic, outcome_pessimistic, outcome_degraded
		)
	`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, a := range aggregates {
		err = batch.Append(
			a.StrategyID, a.ScenarioID, a.EntryEventType,
			a.TotalTrades, a.TotalTokens, a.Wins, a.Losses, a.WinRate, a.TokenWinRate,
			a.OutcomeMean, a.OutcomeMedian, a.OutcomeP10, a.OutcomeP25, a.OutcomeP75, a.OutcomeP90,
			a.OutcomeMin, a.OutcomeMax, a.OutcomeStddev,
			a.MaxDrawdown, a.MaxConsecutiveLosses,
			a.OutcomeRealistic, a.OutcomePessimistic, a.OutcomeDegraded,
		)
		if err != nil {
			return fmt.Errorf("append to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

// GetByKey retrieves an aggregate by its composite key.
func (s *StrategyAggregateStore) GetByKey(ctx context.Context, strategyID, scenarioID, entryEventType string) (*domain.StrategyAggregate, error) {
	query := `
		SELECT
			strategy_id, scenario_id, entry_event_type,
			total_trades, total_tokens, wins, losses, win_rate, token_win_rate,
			outcome_mean, outcome_median, outcome_p10, outcome_p25, outcome_p75, outcome_p90,
			outcome_min, outcome_max, outcome_stddev,
			max_drawdown, max_consecutive_losses,
			outcome_realistic, outcome_pessimistic, outcome_degraded
		FROM strategy_aggregates FINAL
		WHERE strategy_id = ? AND scenario_id = ? AND entry_event_type = ?
		LIMIT 1
	`

	row := s.conn.QueryRow(ctx, query, strategyID, scenarioID, entryEventType)

	var a domain.StrategyAggregate
	err := row.Scan(
		&a.StrategyID, &a.ScenarioID, &a.EntryEventType,
		&a.TotalTrades, &a.TotalTokens, &a.Wins, &a.Losses, &a.WinRate, &a.TokenWinRate,
		&a.OutcomeMean, &a.OutcomeMedian, &a.OutcomeP10, &a.OutcomeP25, &a.OutcomeP75, &a.OutcomeP90,
		&a.OutcomeMin, &a.OutcomeMax, &a.OutcomeStddev,
		&a.MaxDrawdown, &a.MaxConsecutiveLosses,
		&a.OutcomeRealistic, &a.OutcomePessimistic, &a.OutcomeDegraded,
	)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	return &a, nil
}

// GetByStrategy retrieves all aggregates for a strategy.
func (s *StrategyAggregateStore) GetByStrategy(ctx context.Context, strategyID string) ([]*domain.StrategyAggregate, error) {
	query := `
		SELECT
			strategy_id, scenario_id, entry_event_type,
			total_trades, total_tokens, wins, losses, win_rate, token_win_rate,
			outcome_mean, outcome_median, outcome_p10, outcome_p25, outcome_p75, outcome_p90,
			outcome_min, outcome_max, outcome_stddev,
			max_drawdown, max_consecutive_losses,
			outcome_realistic, outcome_pessimistic, outcome_degraded
		FROM strategy_aggregates FINAL
		WHERE strategy_id = ?
		ORDER BY scenario_id ASC, entry_event_type ASC
	`

	rows, err := s.conn.Query(ctx, query, strategyID)
	if err != nil {
		return nil, fmt.Errorf("query by strategy: %w", err)
	}
	defer rows.Close()

	return scanStrategyAggregates(rows)
}

// GetAll retrieves all aggregates.
func (s *StrategyAggregateStore) GetAll(ctx context.Context) ([]*domain.StrategyAggregate, error) {
	query := `
		SELECT
			strategy_id, scenario_id, entry_event_type,
			total_trades, total_tokens, wins, losses, win_rate, token_win_rate,
			outcome_mean, outcome_median, outcome_p10, outcome_p25, outcome_p75, outcome_p90,
			outcome_min, outcome_max, outcome_stddev,
			max_drawdown, max_consecutive_losses,
			outcome_realistic, outcome_pessimistic, outcome_degraded
		FROM strategy_aggregates FINAL
		ORDER BY strategy_id ASC, scenario_id ASC, entry_event_type ASC
	`

	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query all: %w", err)
	}
	defer rows.Close()

	return scanStrategyAggregates(rows)
}

// exists checks if an aggregate with the given key exists.
func (s *StrategyAggregateStore) exists(ctx context.Context, strategyID, scenarioID, entryEventType string) (bool, error) {
	query := `
		SELECT count(*) FROM strategy_aggregates FINAL
		WHERE strategy_id = ? AND scenario_id = ? AND entry_event_type = ?
	`

	var count uint64
	err := s.conn.QueryRow(ctx, query, strategyID, scenarioID, entryEventType).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Rows interface for scanning
type chRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

// scanStrategyAggregates scans multiple rows into a slice.
func scanStrategyAggregates(rows chRows) ([]*domain.StrategyAggregate, error) {
	var aggregates []*domain.StrategyAggregate

	for rows.Next() {
		var a domain.StrategyAggregate
		err := rows.Scan(
			&a.StrategyID, &a.ScenarioID, &a.EntryEventType,
			&a.TotalTrades, &a.TotalTokens, &a.Wins, &a.Losses, &a.WinRate, &a.TokenWinRate,
			&a.OutcomeMean, &a.OutcomeMedian, &a.OutcomeP10, &a.OutcomeP25, &a.OutcomeP75, &a.OutcomeP90,
			&a.OutcomeMin, &a.OutcomeMax, &a.OutcomeStddev,
			&a.MaxDrawdown, &a.MaxConsecutiveLosses,
			&a.OutcomeRealistic, &a.OutcomePessimistic, &a.OutcomeDegraded,
		)
		if err != nil {
			return nil, fmt.Errorf("scan aggregate row: %w", err)
		}
		aggregates = append(aggregates, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregate rows: %w", err)
	}

	return aggregates, nil
}
