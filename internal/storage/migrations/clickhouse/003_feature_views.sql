-- Migration: 003_feature_views
-- Description: Deterministic views for computing derived features from time series
-- Requires: 001_timeseries.sql, 002_derived_features.sql

USE solana_lab;

-- ============================================================================
-- Price Derivatives View
-- ============================================================================
-- Computes price_delta, price_velocity, price_acceleration per candidate

CREATE OR REPLACE VIEW v_price_derivatives AS
SELECT
    candidate_id,
    timestamp_ms,
    slot,
    price,
    volume,
    swap_count,

    -- Previous values using window functions
    lagInFrame(timestamp_ms, 1) OVER w AS prev_timestamp_ms,
    lagInFrame(price, 1) OVER w AS prev_price,

    -- price_delta = price[t] - price[t-1], NULL if first row
    if(
        row_number() OVER w > 1,
        price - lagInFrame(price, 1) OVER w,
        NULL
    ) AS price_delta,

    -- time_delta for velocity/acceleration
    if(
        row_number() OVER w > 1,
        timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w,
        NULL
    ) AS time_delta_ms,

    -- price_velocity = price_delta / time_delta_ms, NULL if first row or time_delta = 0
    if(
        row_number() OVER w > 1 AND (timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w) > 0,
        (price - lagInFrame(price, 1) OVER w) / (timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w),
        NULL
    ) AS price_velocity

FROM price_timeseries
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);

-- ============================================================================
-- Price Acceleration View
-- ============================================================================
-- Computes price_acceleration from price_velocity (requires nested window)

CREATE OR REPLACE VIEW v_price_acceleration AS
SELECT
    candidate_id,
    timestamp_ms,
    price,
    price_delta,
    price_velocity,
    time_delta_ms,

    -- price_acceleration = (velocity[t] - velocity[t-1]) / time_delta_ms
    -- NULL if first or second row
    if(
        row_number() OVER w > 1 AND price_velocity IS NOT NULL AND lagInFrame(price_velocity, 1) OVER w IS NOT NULL AND time_delta_ms > 0,
        (price_velocity - lagInFrame(price_velocity, 1) OVER w) / time_delta_ms,
        NULL
    ) AS price_acceleration

FROM v_price_derivatives
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);

-- ============================================================================
-- Liquidity Derivatives View
-- ============================================================================
-- Computes liquidity_delta, liquidity_velocity per candidate

CREATE OR REPLACE VIEW v_liquidity_derivatives AS
SELECT
    candidate_id,
    timestamp_ms,
    slot,
    liquidity,
    liquidity_token,
    liquidity_quote,

    -- Previous values
    lagInFrame(timestamp_ms, 1) OVER w AS prev_timestamp_ms,
    lagInFrame(liquidity, 1) OVER w AS prev_liquidity,

    -- liquidity_delta = liquidity[t] - liquidity[t-1], NULL if first row
    if(
        row_number() OVER w > 1,
        liquidity - lagInFrame(liquidity, 1) OVER w,
        NULL
    ) AS liquidity_delta,

    -- time_delta for velocity
    if(
        row_number() OVER w > 1,
        timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w,
        NULL
    ) AS time_delta_ms,

    -- liquidity_velocity = liquidity_delta / time_delta_ms, NULL if first row or time_delta = 0
    if(
        row_number() OVER w > 1 AND (timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w) > 0,
        (liquidity - lagInFrame(liquidity, 1) OVER w) / (timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w),
        NULL
    ) AS liquidity_velocity

FROM liquidity_timeseries
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);

-- ============================================================================
-- Token Lifecycle View
-- ============================================================================
-- Aggregates first/last timestamps and lifetime per candidate

CREATE OR REPLACE VIEW v_token_lifecycle AS
SELECT
    candidate_id,
    min(timestamp_ms) AS first_swap_ts,
    max(timestamp_ms) AS last_swap_ts,
    max(timestamp_ms) - min(timestamp_ms) AS total_lifetime_ms,
    count() AS total_swaps
FROM price_timeseries
GROUP BY candidate_id;

-- ============================================================================
-- Token Lifecycle (Liquidity) View
-- ============================================================================
-- Aggregates first/last liquidity event timestamps per candidate

CREATE OR REPLACE VIEW v_token_lifecycle_liquidity AS
SELECT
    candidate_id,
    min(timestamp_ms) AS first_liq_event_ts,
    max(timestamp_ms) AS last_liq_event_ts,
    count() AS total_liq_events
FROM liquidity_timeseries
GROUP BY candidate_id;

-- ============================================================================
-- Last Swap Interval View
-- ============================================================================
-- Computes last_swap_interval_ms per row (time since previous swap)

CREATE OR REPLACE VIEW v_last_swap_interval AS
SELECT
    candidate_id,
    timestamp_ms,

    -- last_swap_interval_ms = timestamp_ms - prev_timestamp_ms, NULL if first row
    if(
        row_number() OVER w > 1,
        timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w,
        NULL
    ) AS last_swap_interval_ms

FROM price_timeseries
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);

-- ============================================================================
-- Last Liquidity Event Interval View
-- ============================================================================
-- Computes last_liq_event_interval_ms per row (time since previous liquidity event)

CREATE OR REPLACE VIEW v_last_liq_event_interval AS
SELECT
    candidate_id,
    timestamp_ms,

    -- last_liq_event_interval_ms = timestamp_ms - prev_timestamp_ms, NULL if first row
    if(
        row_number() OVER w > 1,
        timestamp_ms - lagInFrame(timestamp_ms, 1) OVER w,
        NULL
    ) AS last_liq_event_interval_ms

FROM liquidity_timeseries
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);

-- ============================================================================
-- Derived Features Computed View
-- ============================================================================
-- Joins all derivatives and lifecycle metrics into complete derived features
-- Alignment Policy: liquidity features joined on exact (candidate_id, timestamp_ms)
--                   If no matching liquidity event at price timestamp, values are NULL

CREATE OR REPLACE VIEW v_derived_features_computed AS
SELECT
    p.candidate_id,
    p.timestamp_ms,

    -- Price derivatives
    p.price_delta,
    p.price_velocity,
    pa.price_acceleration,

    -- Liquidity derivatives (exact timestamp match, otherwise NULL)
    ld.liquidity_delta,
    ld.liquidity_velocity,

    -- Token lifetime
    p.timestamp_ms - lc.first_swap_ts AS token_lifetime_ms,

    -- Last swap interval
    si.last_swap_interval_ms,

    -- Last liquidity event interval (exact timestamp match, otherwise NULL)
    li.last_liq_event_interval_ms

FROM v_price_derivatives p
LEFT JOIN v_price_acceleration pa
    ON p.candidate_id = pa.candidate_id
    AND p.timestamp_ms = pa.timestamp_ms
LEFT JOIN v_token_lifecycle lc
    ON p.candidate_id = lc.candidate_id
LEFT JOIN v_last_swap_interval si
    ON p.candidate_id = si.candidate_id
    AND p.timestamp_ms = si.timestamp_ms
LEFT JOIN v_liquidity_derivatives ld
    ON p.candidate_id = ld.candidate_id
    AND p.timestamp_ms = ld.timestamp_ms
LEFT JOIN v_last_liq_event_interval li
    ON p.candidate_id = li.candidate_id
    AND p.timestamp_ms = li.timestamp_ms;

-- ============================================================================
-- Feature Validation View
-- ============================================================================
-- Validates computed features against expected formulas

CREATE OR REPLACE VIEW v_feature_validation AS
SELECT
    candidate_id,
    timestamp_ms,
    price_delta,
    price_velocity,

    -- Validate: price_velocity * time_delta_ms should equal price_delta
    if(
        price_velocity IS NOT NULL AND time_delta_ms > 0,
        abs(price_velocity * time_delta_ms - price_delta) < 0.0001,
        true
    ) AS velocity_valid,

    -- Validate: first row should have NULL derivatives
    if(
        row_number() OVER w = 1,
        price_delta IS NULL AND price_velocity IS NULL,
        true
    ) AS first_row_valid

FROM v_price_derivatives
WINDOW w AS (PARTITION BY candidate_id ORDER BY timestamp_ms ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW);
