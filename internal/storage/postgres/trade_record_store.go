package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/storage"
)

// TradeRecordStore implements storage.TradeRecordStore using PostgreSQL.
type TradeRecordStore struct {
	pool *Pool
}

// NewTradeRecordStore creates a new TradeRecordStore.
func NewTradeRecordStore(pool *Pool) *TradeRecordStore {
	return &TradeRecordStore{pool: pool}
}

// Compile-time interface check.
var _ storage.TradeRecordStore = (*TradeRecordStore)(nil)

// Insert adds a new trade. Returns ErrDuplicateKey if trade_id exists.
func (s *TradeRecordStore) Insert(ctx context.Context, t *domain.TradeRecord) error {
	query := `
		INSERT INTO trade_records (
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21,
			$22, $23, $24,
			$25, $26, $27
		)
	`

	_, err := s.pool.Exec(ctx, query,
		t.TradeID, t.CandidateID, t.StrategyID, t.ScenarioID,
		t.EntrySignalTime, t.EntrySignalPrice, t.EntryActualTime, t.EntryActualPrice,
		t.EntryLiquidity, t.PositionSize, t.PositionValue,
		t.ExitSignalTime, t.ExitSignalPrice, t.ExitActualTime, t.ExitActualPrice, t.ExitReason,
		t.EntryCostSOL, t.ExitCostSOL, t.MEVCostSOL, t.TotalCostSOL, t.TotalCostPct,
		t.GrossReturn, t.Outcome, t.OutcomeClass,
		t.HoldDurationMs, t.PeakPrice, t.MinLiquidity,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return storage.ErrDuplicateKey
		}
		return fmt.Errorf("insert trade record: %w", err)
	}
	return nil
}

// InsertBulk adds multiple trades atomically. Fails entire batch on any duplicate.
func (s *TradeRecordStore) InsertBulk(ctx context.Context, trades []*domain.TradeRecord) error {
	if len(trades) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO trade_records (
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21,
			$22, $23, $24,
			$25, $26, $27
		)
	`

	for _, t := range trades {
		_, err := tx.Exec(ctx, query,
			t.TradeID, t.CandidateID, t.StrategyID, t.ScenarioID,
			t.EntrySignalTime, t.EntrySignalPrice, t.EntryActualTime, t.EntryActualPrice,
			t.EntryLiquidity, t.PositionSize, t.PositionValue,
			t.ExitSignalTime, t.ExitSignalPrice, t.ExitActualTime, t.ExitActualPrice, t.ExitReason,
			t.EntryCostSOL, t.ExitCostSOL, t.MEVCostSOL, t.TotalCostSOL, t.TotalCostPct,
			t.GrossReturn, t.Outcome, t.OutcomeClass,
			t.HoldDurationMs, t.PeakPrice, t.MinLiquidity,
		)
		if err != nil {
			if isDuplicateKeyError(err) {
				return storage.ErrDuplicateKey
			}
			return fmt.Errorf("insert trade record in bulk: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// GetByID retrieves a trade by its ID. Returns ErrNotFound if not exists.
func (s *TradeRecordStore) GetByID(ctx context.Context, tradeID string) (*domain.TradeRecord, error) {
	query := `
		SELECT
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		FROM trade_records
		WHERE trade_id = $1
	`

	row := s.pool.QueryRow(ctx, query, tradeID)
	t, err := scanTradeRecord(row)
	if err != nil {
		if isNotFoundError(err) {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("get trade record by id: %w", err)
	}
	return t, nil
}

// GetByCandidateID retrieves all trades for a candidate.
func (s *TradeRecordStore) GetByCandidateID(ctx context.Context, candidateID string) ([]*domain.TradeRecord, error) {
	query := `
		SELECT
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		FROM trade_records
		WHERE candidate_id = $1
		ORDER BY entry_signal_time ASC, trade_id ASC
	`

	rows, err := s.pool.Query(ctx, query, candidateID)
	if err != nil {
		return nil, fmt.Errorf("get trade records by candidate id: %w", err)
	}
	defer rows.Close()

	return scanTradeRecords(rows)
}

// GetByStrategyScenario retrieves all trades for a strategy/scenario combination.
func (s *TradeRecordStore) GetByStrategyScenario(ctx context.Context, strategyID, scenarioID string) ([]*domain.TradeRecord, error) {
	query := `
		SELECT
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		FROM trade_records
		WHERE strategy_id = $1 AND scenario_id = $2
		ORDER BY entry_signal_time ASC, trade_id ASC
	`

	rows, err := s.pool.Query(ctx, query, strategyID, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("get trade records by strategy/scenario: %w", err)
	}
	defer rows.Close()

	return scanTradeRecords(rows)
}

// GetAll retrieves all trades.
func (s *TradeRecordStore) GetAll(ctx context.Context) ([]*domain.TradeRecord, error) {
	query := `
		SELECT
			trade_id, candidate_id, strategy_id, scenario_id,
			entry_signal_time, entry_signal_price, entry_actual_time, entry_actual_price,
			entry_liquidity, position_size, position_value,
			exit_signal_time, exit_signal_price, exit_actual_time, exit_actual_price, exit_reason,
			entry_cost_sol, exit_cost_sol, mev_cost_sol, total_cost_sol, total_cost_pct,
			gross_return, outcome, outcome_class,
			hold_duration_ms, peak_price, min_liquidity
		FROM trade_records
		ORDER BY entry_signal_time ASC, trade_id ASC
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get all trade records: %w", err)
	}
	defer rows.Close()

	return scanTradeRecords(rows)
}

// scanTradeRecord scans a single row into a TradeRecord.
func scanTradeRecord(row pgx.Row) (*domain.TradeRecord, error) {
	var t domain.TradeRecord

	err := row.Scan(
		&t.TradeID, &t.CandidateID, &t.StrategyID, &t.ScenarioID,
		&t.EntrySignalTime, &t.EntrySignalPrice, &t.EntryActualTime, &t.EntryActualPrice,
		&t.EntryLiquidity, &t.PositionSize, &t.PositionValue,
		&t.ExitSignalTime, &t.ExitSignalPrice, &t.ExitActualTime, &t.ExitActualPrice, &t.ExitReason,
		&t.EntryCostSOL, &t.ExitCostSOL, &t.MEVCostSOL, &t.TotalCostSOL, &t.TotalCostPct,
		&t.GrossReturn, &t.Outcome, &t.OutcomeClass,
		&t.HoldDurationMs, &t.PeakPrice, &t.MinLiquidity,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// scanTradeRecords scans multiple rows into a slice of TradeRecord.
func scanTradeRecords(rows pgx.Rows) ([]*domain.TradeRecord, error) {
	var trades []*domain.TradeRecord

	for rows.Next() {
		var t domain.TradeRecord

		err := rows.Scan(
			&t.TradeID, &t.CandidateID, &t.StrategyID, &t.ScenarioID,
			&t.EntrySignalTime, &t.EntrySignalPrice, &t.EntryActualTime, &t.EntryActualPrice,
			&t.EntryLiquidity, &t.PositionSize, &t.PositionValue,
			&t.ExitSignalTime, &t.ExitSignalPrice, &t.ExitActualTime, &t.ExitActualPrice, &t.ExitReason,
			&t.EntryCostSOL, &t.ExitCostSOL, &t.MEVCostSOL, &t.TotalCostSOL, &t.TotalCostPct,
			&t.GrossReturn, &t.Outcome, &t.OutcomeClass,
			&t.HoldDurationMs, &t.PeakPrice, &t.MinLiquidity,
		)
		if err != nil {
			return nil, fmt.Errorf("scan trade record row: %w", err)
		}

		trades = append(trades, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trade record rows: %w", err)
	}

	return trades, nil
}
