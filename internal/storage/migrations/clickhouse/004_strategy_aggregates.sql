-- Strategy Aggregates Table
-- Stores pre-computed aggregate metrics per strategy/scenario/entry_event_type.
-- Append-only: composite key (strategy_id, scenario_id, entry_event_type) must be unique.

USE solana_lab;

CREATE TABLE IF NOT EXISTS strategy_aggregates (
    strategy_id String,
    scenario_id String,
    entry_event_type String,

    -- Counts
    total_trades UInt32,
    total_tokens UInt32,
    wins UInt32,
    losses UInt32,
    win_rate Float64,
    token_win_rate Float64,

    -- Outcome Distribution
    outcome_mean Float64,
    outcome_median Float64,
    outcome_p10 Float64,
    outcome_p25 Float64,
    outcome_p75 Float64,
    outcome_p90 Float64,
    outcome_min Float64,
    outcome_max Float64,
    outcome_stddev Float64,

    -- Drawdown
    max_drawdown Float64,
    max_consecutive_losses UInt32,

    -- Sensitivity (cross-scenario comparison)
    outcome_realistic Nullable(Float64),
    outcome_pessimistic Nullable(Float64),
    outcome_degraded Nullable(Float64),

    created_at DateTime DEFAULT now()
)
ENGINE = ReplacingMergeTree(created_at)
ORDER BY (strategy_id, scenario_id, entry_event_type)
SETTINGS index_granularity = 8192;

-- Comment
-- This table uses ReplacingMergeTree to handle updates while maintaining
-- append-only semantics for the same composite key.
