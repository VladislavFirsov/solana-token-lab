-- Migration: 001_timeseries
-- Description: Time series tables for price, liquidity, and volume aggregates
-- Engine: MergeTree for efficient time-series queries

USE solana_lab;

-- Price time series (aggregated from swaps)
CREATE TABLE IF NOT EXISTS price_timeseries (
    candidate_id        String,
    timestamp_ms        UInt64,                     -- Unix timestamp (ms)
    slot                UInt64,
    price               Float64,
    volume              Float64,
    swap_count          UInt32
) ENGINE = MergeTree()
ORDER BY (candidate_id, timestamp_ms)
SETTINGS index_granularity = 8192;

-- Liquidity time series (aggregated from liquidity_events)
CREATE TABLE IF NOT EXISTS liquidity_timeseries (
    candidate_id        String,
    timestamp_ms        UInt64,                     -- Unix timestamp (ms)
    slot                UInt64,
    liquidity           Float64,                    -- total pool liquidity
    liquidity_token     Float64,                    -- token side liquidity
    liquidity_quote     Float64                     -- quote side liquidity (SOL/USDC)
) ENGINE = MergeTree()
ORDER BY (candidate_id, timestamp_ms)
SETTINGS index_granularity = 8192;

-- Volume time series (aggregated by interval)
CREATE TABLE IF NOT EXISTS volume_timeseries (
    candidate_id        String,
    timestamp_ms        UInt64,                     -- Unix timestamp (ms) of interval start
    interval_seconds    UInt32,                     -- aggregation interval: 60, 300, 3600
    volume              Float64,                    -- total volume in interval
    swap_count          UInt32,                     -- number of swaps in interval
    buy_volume          Float64,                    -- buy-side volume
    sell_volume         Float64                     -- sell-side volume
) ENGINE = MergeTree()
ORDER BY (candidate_id, interval_seconds, timestamp_ms)
SETTINGS index_granularity = 8192;
