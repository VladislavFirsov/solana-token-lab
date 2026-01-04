-- Migration: 002_derived_features
-- Description: Derived features computed from time series data
-- Engine: MergeTree for efficient analytical queries

CREATE TABLE IF NOT EXISTS derived_features (
    candidate_id                String,
    timestamp_ms                UInt64,             -- Unix timestamp (ms)

    -- Price derivatives (Nullable for first row where t-1 doesn't exist)
    price_delta                 Nullable(Float64),  -- price[t] - price[t-1]
    price_velocity              Nullable(Float64),  -- price_delta / time_delta (price change rate)
    price_acceleration          Nullable(Float64),  -- velocity[t] - velocity[t-1] (rate of velocity change)

    -- Liquidity derivatives (Nullable for first row)
    liquidity_delta             Nullable(Float64),  -- liquidity[t] - liquidity[t-1]
    liquidity_velocity          Nullable(Float64),  -- liquidity_delta / time_delta

    -- Token lifecycle metrics
    token_lifetime_ms           UInt64,             -- timestamp_ms - first_swap_timestamp_ms

    -- Event interval metrics (Nullable if no previous event)
    last_swap_interval_ms       Nullable(UInt64),   -- time since last swap
    last_liq_event_interval_ms  Nullable(UInt64)    -- time since last liquidity event

) ENGINE = MergeTree()
ORDER BY (candidate_id, timestamp_ms)
SETTINGS index_granularity = 8192;

-- Feature computation formulas (for documentation):
--
-- price_delta = price[t] - price[t-1]
-- price_velocity = price_delta / (timestamp_ms[t] - timestamp_ms[t-1])
-- price_acceleration = (velocity[t] - velocity[t-1]) / (timestamp_ms[t] - timestamp_ms[t-1])
--
-- liquidity_delta = liquidity[t] - liquidity[t-1]
-- liquidity_velocity = liquidity_delta / (timestamp_ms[t] - timestamp_ms[t-1])
--
-- token_lifetime_ms = current_timestamp_ms - first_swap_timestamp_ms
-- last_swap_interval_ms = current_timestamp_ms - last_swap_timestamp_ms
-- last_liq_event_interval_ms = current_timestamp_ms - last_liquidity_event_timestamp_ms
