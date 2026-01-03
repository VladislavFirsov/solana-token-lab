# ClickHouse Schema — Phase 1

## Overview

ClickHouse stores **aggregated time series** and **derived features** for analytics. Data is synced from PostgreSQL (source of truth). ClickHouse is optimized for:
- Fast analytical queries
- Time series aggregations
- Feature computation for simulation

---

## Tables

### price_timeseries

Aggregated price data from swaps.

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Unix timestamp in milliseconds |
| slot | UInt64 | Solana slot number |
| price | Float64 | Price at this point |
| volume | Float64 | Volume at this point |
| swap_count | UInt32 | Number of swaps aggregated |

**Engine:** MergeTree()
**Order:** (candidate_id, timestamp_ms)

---

### liquidity_timeseries

Aggregated liquidity data from liquidity events.

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Unix timestamp in milliseconds |
| slot | UInt64 | Solana slot number |
| liquidity | Float64 | Total pool liquidity |
| liquidity_token | Float64 | Token side liquidity |
| liquidity_quote | Float64 | Quote side liquidity (SOL/USDC) |

**Engine:** MergeTree()
**Order:** (candidate_id, timestamp_ms)

---

### volume_timeseries

Volume aggregated by time intervals.

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Interval start timestamp (ms) |
| interval_seconds | UInt32 | Aggregation interval: 60, 300, 3600 |
| volume | Float64 | Total volume in interval |
| swap_count | UInt32 | Number of swaps in interval |
| buy_volume | Float64 | Buy-side volume |
| sell_volume | Float64 | Sell-side volume |

**Engine:** MergeTree()
**Order:** (candidate_id, interval_seconds, timestamp_ms)

**Supported intervals:**
- 60 seconds (1 minute)
- 300 seconds (5 minutes)
- 3600 seconds (1 hour)

---

### derived_features

Computed features for simulation and analysis.

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Unix timestamp in milliseconds |
| price_delta | Float64 | Price change from previous point |
| price_velocity | Float64 | Rate of price change |
| price_acceleration | Float64 | Rate of velocity change |
| liquidity_delta | Float64 | Liquidity change from previous point |
| liquidity_velocity | Float64 | Rate of liquidity change |
| token_lifetime_ms | UInt64 | Time since first swap |
| last_swap_interval_ms | UInt64 | Time since last swap |
| last_liq_event_interval_ms | UInt64 | Time since last liquidity event |

**Engine:** MergeTree()
**Order:** (candidate_id, timestamp_ms)

---

## Derived Feature Formulas

All features are computed deterministically:

### Price Derivatives
```
price_delta[t] = price[t] - price[t-1]

price_velocity[t] = price_delta[t] / (timestamp_ms[t] - timestamp_ms[t-1])

price_acceleration[t] = (price_velocity[t] - price_velocity[t-1]) / (timestamp_ms[t] - timestamp_ms[t-1])
```

### Liquidity Derivatives
```
liquidity_delta[t] = liquidity[t] - liquidity[t-1]

liquidity_velocity[t] = liquidity_delta[t] / (timestamp_ms[t] - timestamp_ms[t-1])
```

### Lifecycle Metrics
```
token_lifetime_ms = current_timestamp_ms - first_swap_timestamp_ms

last_swap_interval_ms = current_timestamp_ms - last_swap_timestamp_ms

last_liq_event_interval_ms = current_timestamp_ms - last_liquidity_event_timestamp_ms
```

---

## Sync Procedure from PostgreSQL

ClickHouse tables are populated from PostgreSQL raw data via PostgreSQL integration.

### PostgreSQL Integration Tables

Before syncing, create ClickHouse tables that connect to PostgreSQL:

```sql
-- postgres_swaps: view into PostgreSQL swaps table
CREATE TABLE postgres_swaps (
    candidate_id    String,
    tx_signature    String,
    slot            UInt64,
    timestamp       UInt64,     -- Unix ms from PostgreSQL BIGINT
    side            String,
    amount_in       Float64,
    amount_out      Float64,
    price           Float64
) ENGINE = PostgreSQL('postgres:5432', 'solana_token_lab', 'swaps', 'user', 'password');

-- postgres_liquidity_events: view into PostgreSQL liquidity_events table
CREATE TABLE postgres_liquidity_events (
    candidate_id    String,
    tx_signature    String,
    slot            UInt64,
    timestamp       UInt64,     -- Unix ms from PostgreSQL BIGINT
    event_type      String,
    amount_token    Float64,
    amount_quote    Float64,
    liquidity_after Float64
) ENGINE = PostgreSQL('postgres:5432', 'solana_token_lab', 'liquidity_events', 'user', 'password');
```

### 1. Price Timeseries
```sql
INSERT INTO price_timeseries
SELECT
    candidate_id,
    timestamp AS timestamp_ms,      -- already UInt64 (Unix ms)
    slot,
    price,
    amount_out AS volume,
    1 AS swap_count
FROM postgres_swaps
ORDER BY candidate_id, timestamp_ms;
```

### 2. Liquidity Timeseries
```sql
INSERT INTO liquidity_timeseries
SELECT
    candidate_id,
    timestamp AS timestamp_ms,      -- already UInt64 (Unix ms)
    slot,
    liquidity_after AS liquidity,
    amount_token AS liquidity_token,
    amount_quote AS liquidity_quote
FROM postgres_liquidity_events
ORDER BY candidate_id, timestamp_ms;
```

### 3. Volume Aggregation
```sql
INSERT INTO volume_timeseries
WITH interval_start AS (
    SELECT
        candidate_id,
        toUnixTimestamp(toStartOfInterval(fromUnixTimestamp64Milli(timestamp), INTERVAL 60 SECOND)) * 1000 AS timestamp_ms,
        amount_out,
        side
    FROM postgres_swaps
)
SELECT
    candidate_id,
    timestamp_ms,
    60 AS interval_seconds,
    sum(amount_out) AS volume,
    count() AS swap_count,
    sumIf(amount_out, side = 'buy') AS buy_volume,
    sumIf(amount_out, side = 'sell') AS sell_volume
FROM interval_start
GROUP BY candidate_id, timestamp_ms;
```

---

## Migration Files

| Order | File | Description |
|-------|------|-------------|
| 1 | `001_timeseries.sql` | Price, liquidity, volume tables |
| 2 | `002_derived_features.sql` | Derived features table |

Run migrations:
```bash
clickhouse-client --query "$(cat sql/clickhouse/001_timeseries.sql)"
clickhouse-client --query "$(cat sql/clickhouse/002_derived_features.sql)"
```

---

## Query Examples

### Get price history for a token
```sql
SELECT timestamp_ms, price, volume
FROM price_timeseries
WHERE candidate_id = '<candidate_id>'
ORDER BY timestamp_ms;
```

### Get hourly volume
```sql
SELECT timestamp_ms, volume, swap_count
FROM volume_timeseries
WHERE candidate_id = '<candidate_id>'
  AND interval_seconds = 3600
ORDER BY timestamp_ms;
```

### Get derived features for simulation
```sql
SELECT
    timestamp_ms,
    price_delta,
    price_velocity,
    liquidity_delta,
    token_lifetime_ms
FROM derived_features
WHERE candidate_id = '<candidate_id>'
ORDER BY timestamp_ms;
```

---

## References

- `docs/SCHEMA_POSTGRES.md` — Source of truth schema
- `docs/MVP_CRITERIA.md` — Normalization & features criteria
- `docs/STRATEGY_CATALOG.md` — Feature usage in strategies
