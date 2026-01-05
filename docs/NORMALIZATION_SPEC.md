# Normalization Specification — Phase 1

## Overview

This document defines deterministic rules for transforming raw events (PostgreSQL) into time series and derived features (ClickHouse). All transformations are reproducible and formula-based.

---

## 1. Event Ordering & Tie-Breaking

### Canonical Order

All events are ordered by:

```
1. slot ASC
2. tx_signature ASC
3. event_index ASC
```

### Determinism Guarantees

- Same raw events → same time series output
- No randomness or time-dependent behavior
- All timestamps in Unix milliseconds (UInt64)
- Ordering is consistent across replays

---

## 2. Time Series Transformations

### 2.1 Price Time Series (price_t)

**Source:** `swaps` table (PostgreSQL)

**Transformation:**

```
FOR each swap ordered by (slot, tx_signature, event_index):
    emit row:
        candidate_id    = swap.candidate_id
        timestamp_ms    = swap.timestamp
        slot            = swap.slot
        price           = swap.price
        volume          = swap.amount_out
        swap_count      = 1
```

**Aggregation (same timestamp):**

If multiple swaps have the same `(candidate_id, timestamp_ms)`:

```
price      = LAST(price) by event order
volume     = SUM(amount_out)
swap_count = COUNT(*)
slot       = LAST(slot) by event order
```

**Note:** The `slot` field uses LAST() semantics to maintain consistency with the `price` field, which also uses LAST(). This ensures the slot corresponds to the final swap that determined the aggregated price.

**Output Schema:**

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Unix timestamp in milliseconds |
| slot | UInt64 | Solana slot number |
| price | Float64 | Price at this point |
| volume | Float64 | Volume at this point |
| swap_count | UInt32 | Number of swaps aggregated |

---

### 2.2 Liquidity Time Series (liquidity_t)

**Source:** `liquidity_events` table (PostgreSQL)

**Transformation:**

```
FOR each liquidity_event ordered by (slot, tx_signature, event_index):
    emit row:
        candidate_id      = event.candidate_id
        timestamp_ms      = event.timestamp
        slot              = event.slot
        liquidity         = event.liquidity_after
        liquidity_token   = event.amount_token
        liquidity_quote   = event.amount_quote
```

**Aggregation (same timestamp):**

If multiple events have the same `(candidate_id, timestamp_ms)`:

```
liquidity       = LAST(liquidity_after) by event order
liquidity_token = LAST(amount_token)
liquidity_quote = LAST(amount_quote)
slot            = LAST(slot) by event order
```

**Note:** The `slot` field uses LAST() semantics to maintain consistency with the `liquidity` field. This ensures the slot corresponds to the final event that determined the aggregated liquidity state.

**Output Schema:**

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Unix timestamp in milliseconds |
| slot | UInt64 | Solana slot number |
| liquidity | Float64 | Total pool liquidity |
| liquidity_token | Float64 | Token side liquidity |
| liquidity_quote | Float64 | Quote side liquidity (SOL/USDC) |

---

### 2.3 Volume Time Series (volume_t)

**Source:** `swaps` table (PostgreSQL)

**Supported Intervals:**

| interval_seconds | Duration |
|------------------|----------|
| 60 | 1 minute |
| 300 | 5 minutes |
| 3600 | 1 hour |

**Interval Alignment Formula:**

```
interval_ms    = interval_seconds * 1000
interval_start = floor(timestamp_ms / interval_ms) * interval_ms
```

**Aggregation:**

```
FOR each (candidate_id, interval_start, interval_seconds):
    volume      = SUM(amount_out)
    swap_count  = COUNT(*)
    buy_volume  = SUM(amount_out) WHERE side = 'buy'
    sell_volume = SUM(amount_out) WHERE side = 'sell'
```

**Output Schema:**

| Column | Type | Description |
|--------|------|-------------|
| candidate_id | String | Token candidate identifier |
| timestamp_ms | UInt64 | Interval start timestamp (ms) |
| interval_seconds | UInt32 | Aggregation interval |
| volume | Float64 | Total volume in interval |
| swap_count | UInt32 | Number of swaps in interval |
| buy_volume | Float64 | Buy-side volume |
| sell_volume | Float64 | Sell-side volume |

---

## 3. Time Alignment Policy

### Interval Boundaries

```
Window: [interval_start, interval_start + interval_ms)
- Inclusive start
- Exclusive end
```

### Alignment Examples

| timestamp_ms | interval_seconds | interval_start |
|--------------|------------------|----------------|
| 1704067234567 | 60 | 1704067200000 |
| 1704067234567 | 300 | 1704067200000 |
| 1704067234567 | 3600 | 1704067200000 |

### Price/Liquidity Timestamp Alignment

Price events (swaps) and liquidity events occur independently and may not share the same timestamps. When computing derived features that combine both:

**Alignment Rule:**

```
JOIN ON exact (candidate_id, timestamp_ms) match
IF no matching liquidity event at price timestamp:
    liquidity_delta = NULL
    liquidity_velocity = NULL
    last_liq_event_interval_ms = NULL
```

**Rationale:**

- Price and liquidity events are different event types with independent timing
- Interpolation or "last known value" would introduce non-deterministic behavior
- NULL explicitly signals "no liquidity event at this price timestamp"
- Consumers can handle NULL appropriately (ignore, interpolate, or use as signal)

**Example:**

| price timestamp_ms | liquidity timestamp_ms | liquidity_delta |
|--------------------|------------------------|-----------------|
| 1704067234567 | 1704067234567 | computed value |
| 1704067234890 | (no match) | NULL |
| 1704067235123 | 1704067235123 | computed value |

---

## 4. Missing Data & Empty Intervals

### Policy

- Empty intervals are **NOT emitted** (no zero-fill)
- Time series are **sparse** (only emit when events occur)
- Derived features compute from available data only

### Edge Cases

| Case | Handling |
|------|----------|
| No swaps in interval | No row emitted in volume_t |
| No liquidity events | liquidity_t has no rows for candidate |
| First event for candidate | delta/velocity/acceleration = NULL |
| Gap in data | Compute from last available point |

---

## 5. Derived Feature Formulas

### 5.1 Price Derivatives

**price_delta:**

```
price_delta[t] = price[t] - price[t-1]

WHERE:
    t-1 = previous row for same candidate_id ordered by timestamp_ms

Edge case:
    = NULL if t is first row
```

**price_velocity:**

```
price_velocity[t] = price_delta[t] / time_delta_ms

WHERE:
    time_delta_ms = timestamp_ms[t] - timestamp_ms[t-1]

Edge cases:
    = NULL if time_delta_ms = 0
    = NULL if t is first row

Units: price change per millisecond
```

**price_acceleration:**

```
price_acceleration[t] = (price_velocity[t] - price_velocity[t-1]) / time_delta_ms

WHERE:
    time_delta_ms = timestamp_ms[t] - timestamp_ms[t-1]

Edge cases:
    = NULL if t is first or second row

Units: velocity change per millisecond
```

---

### 5.2 Liquidity Derivatives

**liquidity_delta:**

```
liquidity_delta[t] = liquidity[t] - liquidity[t-1]

WHERE:
    t-1 = previous row for same candidate_id ordered by timestamp_ms

Edge case:
    = NULL if t is first row
```

**liquidity_velocity:**

```
liquidity_velocity[t] = liquidity_delta[t] / time_delta_ms

WHERE:
    time_delta_ms = timestamp_ms[t] - timestamp_ms[t-1]

Edge cases:
    = NULL if time_delta_ms = 0
    = NULL if t is first row

Units: liquidity change per millisecond
```

---

### 5.3 Lifecycle Metrics

**token_lifetime_ms:**

```
token_lifetime_ms[t] = timestamp_ms[t] - first_swap_timestamp_ms

WHERE:
    first_swap_timestamp_ms = MIN(timestamp_ms) for candidate_id in price_timeseries

Units: milliseconds since first swap
```

**last_swap_interval_ms:**

```
last_swap_interval_ms[t] = timestamp_ms[t] - prev_swap_timestamp_ms

WHERE:
    prev_swap_timestamp_ms = LAG(timestamp_ms) OVER (
        PARTITION BY candidate_id
        ORDER BY timestamp_ms
    )

Edge case:
    = NULL if t is first row

Units: milliseconds since previous swap
```

**last_liq_event_interval_ms:**

```
last_liq_event_interval_ms[t] = timestamp_ms[t] - prev_liq_event_timestamp_ms

WHERE:
    prev_liq_event_timestamp_ms = LAG(timestamp_ms) OVER (
        PARTITION BY candidate_id
        ORDER BY timestamp_ms
    ) in liquidity_timeseries

Edge case:
    = NULL if no prior liquidity events

Units: milliseconds since previous liquidity event
```

---

### 5.4 Edge Case Summary

| Case | price_delta | price_velocity | price_acceleration |
|------|-------------|----------------|-------------------|
| First row | NULL | NULL | NULL |
| Second row | computed | computed | NULL |
| time_delta = 0 | NULL | NULL | NULL |
| Gap > 1h | Normal calc | Normal calc | Normal calc |

---

## 6. ClickHouse Implementation

### Window Functions

ClickHouse uses `lagInFrame` for LAG operations:

```sql
lagInFrame(column, offset, default) OVER (
    PARTITION BY candidate_id
    ORDER BY timestamp_ms
)
```

### NULL Handling

- Use `if(condition, value, NULL)` for conditional NULL
- Use `nullIf(value, 0)` to convert zero to NULL
- Nullable types: `Nullable(Float64)`, `Nullable(UInt64)`

---

## 7. Replayability

### Requirements

1. **Deterministic output:** Same raw events → same derived features
2. **No external dependencies:** All data from PostgreSQL/ClickHouse
3. **Consistent ordering:** Events ordered by (slot, tx_signature, event_index)
4. **Parameterized intervals:** interval_seconds is explicit

### Replay Procedure

```
1. Load raw events from PostgreSQL:
   - swaps table
   - liquidity_events table

2. Order events by:
   - slot ASC
   - tx_signature ASC
   - event_index ASC

3. Transform to time series:
   - price_timeseries
   - liquidity_timeseries
   - volume_timeseries

4. Compute derived features:
   - Apply formulas per candidate_id
   - Use window functions for LAG operations

5. Output must match original derived_features table
```

---

## References

- `docs/SCHEMA_CLICKHOUSE.md` — ClickHouse schema
- `docs/MVP_CRITERIA.md` — Section 3: Normalization & Features
- `sql/clickhouse/001_timeseries.sql` — Time series tables
- `sql/clickhouse/002_derived_features.sql` — Derived features table
- `sql/clickhouse/003_feature_views.sql` — Feature computation views
