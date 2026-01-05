# Discovery Specification — Phase 1

## Overview

This document defines deterministic discovery rules for NEW_TOKEN and ACTIVE_TOKEN candidates. All definitions are formal and reproducible.

---

## 1. NEW_TOKEN Discovery

### Definition

A NEW_TOKEN is discovered when the **first swap transaction** is observed for a token mint (first on-chain swap for that mint).

### Detection Criteria

```
trigger = first_swap WHERE:
    is_first_swap_for_mint = true
    pool = captured if available, otherwise NULL
```

### Captured Fields

| Field | Type | Description |
|-------|------|-------------|
| mint | TEXT | Token mint address (from swap instruction) |
| pool | TEXT | Pool address (if available, otherwise NULL) |
| tx_signature | TEXT | Signature of first swap transaction |
| event_index | INTEGER | Swap event index within transaction |
| slot | BIGINT | Solana slot of first swap |
| timestamp | BIGINT | Block timestamp of first swap (Unix ms) |
| source | TEXT | `'NEW_TOKEN'` |
| candidate_id | TEXT | Deterministic hash (see formula below) |

### Detection Logic

```
FOR each swap event ordered by (slot, tx_signature, event_index):
    IF mint NOT IN seen_mints:
        emit NEW_TOKEN candidate
        add mint to seen_mints
```

---

## 2. ACTIVE_TOKEN Discovery

### Definition

An ACTIVE_TOKEN is discovered when an existing token exhibits a volume or swap activity spike above baseline.

### Spike Criteria (Formal)

```
volume_spike = volume_1h > K_vol * volume_avg
swaps_spike = swaps_1h > K_swaps * swaps_avg

WHERE:
    K_vol = 3.0
    K_swaps = 5.0

    # 1-hour window metrics
    volume_1h = SUM(amount_out)
                FOR swaps WHERE timestamp IN [now_ms - 3600000, now_ms]

    swaps_1h = COUNT(swaps)
               WHERE timestamp IN [now_ms - 3600000, now_ms]

    # History normalization
    first_swap_time = MIN(timestamp) FOR mint
    actual_history_ms = MIN(now_ms - first_swap_time, 86400000)
    actual_history_hours = actual_history_ms / 3600000

    # Minimum history requirement (prevents false positives)
    IF actual_history_hours < 1:
        SKIP token (insufficient data)

    # 24-hour totals (capped at available history)
    volume_24h = SUM(amount_out)
                 FOR swaps WHERE timestamp IN [now_ms - actual_history_ms, now_ms]

    swaps_24h = COUNT(swaps)
                WHERE timestamp IN [now_ms - actual_history_ms, now_ms]

    # Hourly averages normalized by actual history
    volume_avg = volume_24h / actual_history_hours
    swaps_avg = swaps_24h / actual_history_hours

TRIGGER:
    IF (volume_spike OR swaps_spike) AND NOT already_discovered:
        emit ACTIVE_TOKEN candidate
```

**Rationale for History Normalization:**

The implementation normalizes by actual available history (capped at 24 hours) rather than using a fixed divisor of 24. This prevents false positive spikes for tokens with less than 24 hours of history.

**Example:**
- Token with 6 hours of history and 600 volume
- Fixed divisor: `volume_avg = 600 / 24 = 25` → false spike detection
- Dynamic divisor: `volume_avg = 600 / 6 = 100` → accurate baseline

### Captured Fields

| Field | Type | Description |
|-------|------|-------------|
| mint | TEXT | Token mint address |
| pool | TEXT | Pool address |
| tx_signature | TEXT | Signature of swap that triggered spike |
| event_index | INTEGER | Swap event index within transaction |
| slot | BIGINT | Solana slot of trigger |
| timestamp | BIGINT | Block timestamp of trigger (Unix ms) |
| source | TEXT | `'ACTIVE_TOKEN'` |
| candidate_id | TEXT | Deterministic hash (see formula below) |

### Detection Logic

```
FOR each evaluation point (per swap or periodic):
    FOR each mint with swaps in last 24h:
        IF mint already in token_candidates:
            SKIP

        COMPUTE volume_1h, volume_24h_avg, swaps_1h, swaps_24h_avg

        IF volume_1h > 3.0 * volume_24h_avg OR swaps_1h > 5.0 * swaps_24h_avg:
            emit ACTIVE_TOKEN candidate with triggering swap details
```

---

## 3. Candidate ID Formula

### Formula

```
candidate_id = SHA256(
    mint || '|' ||
    COALESCE(pool, '') || '|' ||
    source || '|' ||
    tx_signature || '|' ||
    event_index || '|' ||
    discovery_slot
)
```

### Components

| Component | Format | Description |
|-----------|--------|-------------|
| mint | base58 string | Token mint address |
| pool | base58 string or empty | Pool address, empty string if NULL |
| source | string | `'NEW_TOKEN'` or `'ACTIVE_TOKEN'` |
| tx_signature | base58 string | Discovery transaction signature |
| event_index | decimal string | Event index as integer string |
| discovery_slot | decimal string | Slot number as integer string |

### Output

- Format: hex-encoded SHA256
- Length: 64 characters

### Example

```
Input:
  mint = "TokenMint123..."
  pool = "PoolAddr456..."
  source = "NEW_TOKEN"
  tx_signature = "TxSig789..."
  event_index = "0"
  discovery_slot = "12345678"

Concatenated:
  "TokenMint123...|PoolAddr456...|NEW_TOKEN|TxSig789...|0|12345678"

Output:
  candidate_id = "a1b2c3d4..." (64 hex chars)
```

### PostgreSQL Implementation

```sql
encode(digest(
    (mint || '|' || COALESCE(pool, '') || '|' || source || '|' ||
     tx_signature || '|' || event_index::text || '|' || slot::text
    )::bytea,
    'sha256'
), 'hex')
```

---

## 4. Rolling Window Definitions

### 1-Hour Window

```
start_ms = current_timestamp_ms - 3600000
end_ms = current_timestamp_ms
duration = 3600000 ms (1 hour)
```

### 24-Hour Window

```
start_ms = current_timestamp_ms - 86400000
end_ms = current_timestamp_ms
duration = 86400000 ms (24 hours)
```

### Units

| Metric | Unit | Type |
|--------|------|------|
| Timestamps | Unix milliseconds | BIGINT |
| Volumes | Raw token amounts | NUMERIC |
| Counts | Integer | INTEGER |
| Averages | Hourly average | NUMERIC |

### Window Boundaries

- Windows are **inclusive** of start, **exclusive** of end: `[start, end)`
- All comparisons use `>=` for start and `<` for end

---

## 5. Replayability Requirements

### Determinism Guarantees

1. **Same raw events → same candidate stream**
   - Discovery is a pure function of stored events
   - No randomness or time-dependent behavior

2. **No external API calls during replay**
   - All data comes from PostgreSQL
   - No RPC calls to Solana during replay

3. **Consistent ordering**
   - Events ordered by: `(slot, tx_signature, event_index)`
   - Deterministic tie-breaking

4. **Parameterized thresholds**
   - K_vol and K_swaps are parameters
   - Window sizes are explicit constants

### Replay Procedure

```
1. Load raw events from PostgreSQL:
   - swaps table
   - liquidity_events table (if needed)

2. Order events by:
   - slot ASC
   - tx_signature ASC
   - event_index ASC

3. Apply discovery rules in order:
   - NEW_TOKEN: first swap per mint
   - ACTIVE_TOKEN: spike detection with rolling windows

4. Generate candidate_id for each discovery

5. Output candidate stream must match original:
   - Same candidate_ids
   - Same order
   - Same captured fields
```

### Verification Query

```sql
-- Compare replay output with stored candidates
SELECT
    r.candidate_id AS replay_id,
    s.candidate_id AS stored_id,
    r.candidate_id = s.candidate_id AS match
FROM replay_candidates r
FULL OUTER JOIN token_candidates s
    ON r.candidate_id = s.candidate_id
WHERE r.candidate_id IS NULL OR s.candidate_id IS NULL;

-- Must return 0 rows for valid replay
```

---

## 6. Edge Cases

### NEW_TOKEN

| Case | Handling |
|------|----------|
| Multiple swaps in same tx | Use lowest event_index |
| Swap before pool creation | pool = NULL, still valid |
| Re-mint of existing token | Treated as new if mint address differs |

### ACTIVE_TOKEN

| Case | Handling |
|------|----------|
| Token with <1h history | **Skipped** — minimum 1 hour history required |
| Token with 1-24h history | Baseline normalized by actual history hours |
| Token with >=24h history | Baseline normalized by 24 (capped) |
| Spike exactly at threshold | Triggers (uses `>` not `>=`) |
| Multiple spikes same token | Only first spike generates candidate |
| Token already discovered (any source) | **Excluded** — each mint has exactly one candidate |

### Uniqueness Rule

**Each mint can have at most one candidate.** Once a mint is discovered (as NEW_TOKEN or ACTIVE_TOKEN), subsequent discovery attempts for the same mint are ignored. This ensures:
- Deterministic candidate stream
- No duplicate entries per mint
- Clear ownership of mint → candidate mapping

---

## References

- `docs/MVP_CRITERIA.md` — Discovery criteria (sections 1.1, 1.2)
- `docs/BRD.md` — Discovery requirements
- `docs/STRATEGY_CATALOG.md` — Entry event definitions
- `sql/postgres/005_discovery_views.sql` — SQL implementation
