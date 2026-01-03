# PostgreSQL Schema — Phase 1

## Overview

PostgreSQL serves as the **source of truth** for all raw on-chain data. All tables are **append-only** to ensure data integrity and enable deterministic replay.

---

## Tables

### token_candidates

Primary table for discovered token candidates.

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| candidate_id | TEXT | NO | PRIMARY KEY. Deterministic hash-based unique identifier |
| source | TEXT | NO | Discovery source: `NEW_TOKEN` or `ACTIVE_TOKEN` |
| mint | TEXT | NO | Solana token mint address |
| pool | TEXT | YES | Pool address (NULL if discovered before pool creation) |
| tx_signature | TEXT | NO | Transaction signature of discovery event |
| slot | BIGINT | NO | Solana slot number of discovery |
| discovered_at | BIGINT | NO | Unix timestamp in milliseconds |
| created_at | BIGINT | NO | Record creation timestamp (ms) |

**Constraints:**
- PRIMARY KEY on `candidate_id`
- CHECK constraint: `source IN ('NEW_TOKEN', 'ACTIVE_TOKEN')`

**Indexes:**
- `idx_token_candidates_source` — filter by discovery source
- `idx_token_candidates_slot` — query by slot range
- `idx_token_candidates_discovered_at` — query by time range
- `idx_token_candidates_mint` — lookup by mint address

---

### swaps

Swap transactions for token candidates.

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| id | BIGSERIAL | NO | PRIMARY KEY. Auto-increment |
| candidate_id | TEXT | NO | FK to token_candidates |
| tx_signature | TEXT | NO | Solana transaction signature |
| event_index | INTEGER | NO | Index of swap within transaction |
| slot | BIGINT | NO | Solana slot number |
| timestamp | BIGINT | NO | Unix timestamp in milliseconds |
| side | TEXT | NO | Trade direction: `buy` or `sell` |
| amount_in | NUMERIC | NO | Input token amount |
| amount_out | NUMERIC | NO | Output token amount |
| price | NUMERIC | NO | Execution price |
| created_at | BIGINT | NO | Record creation timestamp (ms) |

**Constraints:**
- PRIMARY KEY on `id`
- FOREIGN KEY on `candidate_id`
- UNIQUE on `(tx_signature, candidate_id, event_index)`
- CHECK constraint: `side IN ('buy', 'sell')`

**Indexes:**
- `idx_swaps_candidate_id` — filter by token
- `idx_swaps_slot` — query by slot range
- `idx_swaps_timestamp` — query by time range
- `idx_swaps_candidate_timestamp` — token + time queries

---

### liquidity_events

Liquidity add/remove events.

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| id | BIGSERIAL | NO | PRIMARY KEY. Auto-increment |
| candidate_id | TEXT | NO | FK to token_candidates |
| tx_signature | TEXT | NO | Solana transaction signature |
| event_index | INTEGER | NO | Index of event within transaction |
| slot | BIGINT | NO | Solana slot number |
| timestamp | BIGINT | NO | Unix timestamp in milliseconds |
| event_type | TEXT | NO | Event type: `add` or `remove` |
| amount_token | NUMERIC | NO | Token amount added/removed |
| amount_quote | NUMERIC | NO | Quote currency (SOL/USDC) amount |
| liquidity_after | NUMERIC | NO | Total pool liquidity after event |
| created_at | BIGINT | NO | Record creation timestamp (ms) |

**Constraints:**
- PRIMARY KEY on `id`
- FOREIGN KEY on `candidate_id`
- UNIQUE on `(tx_signature, candidate_id, event_index)`
- CHECK constraint: `event_type IN ('add', 'remove')`

**Indexes:**
- `idx_liquidity_events_candidate_id` — filter by token
- `idx_liquidity_events_slot` — query by slot range
- `idx_liquidity_events_timestamp` — query by time range
- `idx_liquidity_events_candidate_timestamp` — token + time queries

---

### token_metadata

Token metadata from on-chain.

| Column | Type | Nullable | Description |
|--------|------|----------|-------------|
| candidate_id | TEXT | NO | PRIMARY KEY + FK to token_candidates |
| mint | TEXT | NO | Token mint address |
| name | TEXT | YES | Token name |
| symbol | TEXT | YES | Token symbol |
| decimals | INTEGER | NO | Token decimals |
| supply | NUMERIC | YES | Total supply |
| fetched_at | BIGINT | NO | When metadata was fetched (ms) |
| created_at | BIGINT | NO | Record creation timestamp (ms) |

**Constraints:**
- PRIMARY KEY on `candidate_id`
- FOREIGN KEY on `candidate_id`

**Indexes:**
- `idx_token_metadata_mint` — lookup by mint
- `idx_token_metadata_symbol` — search by symbol

---

## Append-Only Policy

All tables enforce append-only semantics:

1. **Application level:** No UPDATE or DELETE statements in code
2. **Database level:** PostgreSQL triggers raise exceptions on UPDATE/DELETE:
   ```sql
   -- Trigger function that raises exception
   CREATE OR REPLACE FUNCTION raise_append_only_violation()
   RETURNS TRIGGER AS $$
   BEGIN
       RAISE EXCEPTION 'Table % is append-only. UPDATE and DELETE are prohibited.', TG_TABLE_NAME;
   END;
   $$ LANGUAGE plpgsql;

   -- Apply to each table
   CREATE TRIGGER table_append_only_update
       BEFORE UPDATE ON table
       FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

   CREATE TRIGGER table_append_only_delete
       BEFORE DELETE ON table
       FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();
   ```

This ensures:
- Historical data integrity
- Deterministic replay capability
- Audit trail preservation
- **Errors are raised** (not silently ignored) to catch bugs early

---

## Replay Procedure

To replay analysis for a specific token:

```sql
-- 1. Get all swaps for a token ordered by time
SELECT * FROM swaps
WHERE candidate_id = '<candidate_id>'
ORDER BY timestamp;

-- 2. Get all liquidity events
SELECT * FROM liquidity_events
WHERE candidate_id = '<candidate_id>'
ORDER BY timestamp;

-- 3. Reconstruct time series from events
-- (Application logic combines swaps + liquidity events)
```

Replay guarantees:
- Same input data → same output (deterministic)
- No data loss (append-only)
- Full audit trail via tx_signature

---

## Migration Files

| Order | File | Description |
|-------|------|-------------|
| 1 | `001_token_candidates.sql` | Token candidates table |
| 2 | `002_swaps.sql` | Swap transactions |
| 3 | `003_liquidity_events.sql` | Liquidity events |
| 4 | `004_token_metadata.sql` | Token metadata |

Run migrations in order:
```bash
psql -d solana_token_lab -f sql/postgres/001_token_candidates.sql
psql -d solana_token_lab -f sql/postgres/002_swaps.sql
psql -d solana_token_lab -f sql/postgres/003_liquidity_events.sql
psql -d solana_token_lab -f sql/postgres/004_token_metadata.sql
```

---

## References

- `docs/BRD.md` — Data collection requirements
- `docs/MVP_CRITERIA.md` — Storage & replay criteria
- `docs/SCHEMA_CLICKHOUSE.md` — Aggregate schema
