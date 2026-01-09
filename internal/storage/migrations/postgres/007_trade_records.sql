-- Trade Records Table
-- Stores simulated trade results from backtesting.
-- Append-only: trade_id is deterministic hash, duplicates rejected.

CREATE TABLE IF NOT EXISTS trade_records (
    id BIGSERIAL PRIMARY KEY,
    trade_id TEXT NOT NULL UNIQUE,
    candidate_id TEXT NOT NULL,
    strategy_id TEXT NOT NULL,
    scenario_id TEXT NOT NULL,

    -- Entry
    entry_signal_time BIGINT NOT NULL,
    entry_signal_price DOUBLE PRECISION NOT NULL,
    entry_actual_time BIGINT NOT NULL,
    entry_actual_price DOUBLE PRECISION NOT NULL,
    entry_liquidity DOUBLE PRECISION,
    position_size DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    position_value DOUBLE PRECISION NOT NULL,

    -- Exit
    exit_signal_time BIGINT NOT NULL,
    exit_signal_price DOUBLE PRECISION NOT NULL,
    exit_actual_time BIGINT NOT NULL,
    exit_actual_price DOUBLE PRECISION NOT NULL,
    exit_reason TEXT NOT NULL,

    -- Costs
    entry_cost_sol DOUBLE PRECISION NOT NULL DEFAULT 0,
    exit_cost_sol DOUBLE PRECISION NOT NULL DEFAULT 0,
    mev_cost_sol DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_cost_sol DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_cost_pct DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Outcome
    gross_return DOUBLE PRECISION NOT NULL,
    outcome DOUBLE PRECISION NOT NULL,
    outcome_class TEXT NOT NULL CHECK (outcome_class IN ('WIN', 'LOSS')),

    -- Metadata
    hold_duration_ms BIGINT NOT NULL,
    peak_price DOUBLE PRECISION,
    min_liquidity DOUBLE PRECISION,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_trade_records_candidate_id ON trade_records(candidate_id);
CREATE INDEX IF NOT EXISTS idx_trade_records_strategy_scenario ON trade_records(strategy_id, scenario_id);
CREATE INDEX IF NOT EXISTS idx_trade_records_entry_time ON trade_records(entry_signal_time);

COMMENT ON TABLE trade_records IS 'Simulated trade results from backtesting. Append-only.';
