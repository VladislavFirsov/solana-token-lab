-- Migration: 005_discovery_views
-- Description: Helper views for NEW_TOKEN and ACTIVE_TOKEN discovery
-- Requires: pgcrypto extension for SHA256

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ============================================================================
-- NEW_TOKEN Discovery View
-- ============================================================================
-- Identifies first swap for each mint (NEW_TOKEN candidates)
-- Uses token_metadata to get mint address

CREATE OR REPLACE VIEW v_new_token_candidates AS
WITH first_swaps AS (
    SELECT DISTINCT ON (tm.mint)
        tm.mint,
        s.tx_signature,
        s.event_index,
        s.slot,
        s.timestamp
    FROM swaps s
    JOIN token_metadata tm ON s.candidate_id = tm.candidate_id
    ORDER BY tm.mint, s.slot, s.tx_signature, s.event_index
)
SELECT
    encode(digest(
        (mint || '|' || '' || '|' || 'NEW_TOKEN' || '|' ||
         tx_signature || '|' || event_index::text || '|' || slot::text
        )::bytea,
        'sha256'
    ), 'hex') AS candidate_id,
    'NEW_TOKEN'::text AS source,
    mint,
    NULL::text AS pool,
    tx_signature,
    event_index,
    slot,
    timestamp
FROM first_swaps;

COMMENT ON VIEW v_new_token_candidates IS 'NEW_TOKEN candidates: first swap per mint using token_metadata';

-- ============================================================================
-- ACTIVE_TOKEN Discovery View
-- ============================================================================
-- Identifies tokens with volume/swap spikes (ACTIVE_TOKEN candidates)
-- Uses thresholds: K_vol = 3.0, K_swaps = 5.0
-- Excludes tokens already discovered (any source)

CREATE OR REPLACE VIEW v_active_token_spike_detection AS
WITH
-- Current timestamp for window calculations (use max timestamp from swaps)
current_time AS (
    SELECT MAX(timestamp) AS now_ms FROM swaps
),

-- Get mint for each candidate (from token_metadata only, no token_candidates dependency)
candidate_mints AS (
    SELECT candidate_id, mint FROM token_metadata
),

-- Hourly aggregates per mint (last 1 hour)
hourly_stats AS (
    SELECT
        cm.mint,
        SUM(s.amount_out) AS volume_1h,
        COUNT(*) AS swaps_1h,
        -- Get the swap that would trigger (last swap in window)
        (ARRAY_AGG(s.tx_signature ORDER BY s.timestamp DESC, s.event_index DESC))[1] AS last_tx_signature,
        (ARRAY_AGG(s.event_index ORDER BY s.timestamp DESC, s.event_index DESC))[1] AS last_event_index,
        (ARRAY_AGG(s.slot ORDER BY s.timestamp DESC, s.event_index DESC))[1] AS last_slot,
        MAX(s.timestamp) AS last_timestamp
    FROM swaps s
    JOIN candidate_mints cm ON s.candidate_id = cm.candidate_id
    CROSS JOIN current_time ct
    WHERE s.timestamp >= ct.now_ms - 3600000  -- 1 hour in ms
      AND s.timestamp < ct.now_ms
    GROUP BY cm.mint
),

-- Daily aggregates per mint (last 24 hours)
daily_stats AS (
    SELECT
        cm.mint,
        SUM(s.amount_out) / 24.0 AS volume_24h_avg,
        COUNT(*)::numeric / 24.0 AS swaps_24h_avg
    FROM swaps s
    JOIN candidate_mints cm ON s.candidate_id = cm.candidate_id
    CROSS JOIN current_time ct
    WHERE s.timestamp >= ct.now_ms - 86400000  -- 24 hours in ms
      AND s.timestamp < ct.now_ms
    GROUP BY cm.mint
),

-- Spike detection
spikes AS (
    SELECT
        h.mint,
        h.volume_1h,
        h.swaps_1h,
        d.volume_24h_avg,
        d.swaps_24h_avg,
        h.last_tx_signature AS tx_signature,
        h.last_event_index AS event_index,
        h.last_slot AS slot,
        h.last_timestamp AS timestamp,
        -- Spike flags
        (h.volume_1h > 3.0 * d.volume_24h_avg) AS volume_spike,
        (h.swaps_1h > 5.0 * d.swaps_24h_avg) AS swaps_spike
    FROM hourly_stats h
    JOIN daily_stats d ON h.mint = d.mint
    WHERE h.volume_1h > 3.0 * d.volume_24h_avg
       OR h.swaps_1h > 5.0 * d.swaps_24h_avg
)

SELECT
    encode(digest(
        (mint || '|' || '' || '|' || 'ACTIVE_TOKEN' || '|' ||
         tx_signature || '|' || event_index::text || '|' || slot::text
        )::bytea,
        'sha256'
    ), 'hex') AS candidate_id,
    'ACTIVE_TOKEN'::text AS source,
    mint,
    NULL::text AS pool,
    tx_signature,
    event_index,
    slot,
    timestamp,
    volume_1h,
    swaps_1h,
    volume_24h_avg,
    swaps_24h_avg,
    volume_spike,
    swaps_spike
FROM spikes;

COMMENT ON VIEW v_active_token_spike_detection IS 'ACTIVE_TOKEN spike detection with K_vol=3.0, K_swaps=5.0';

-- ============================================================================
-- ACTIVE_TOKEN Candidates View (filtered, no duplicates)
-- ============================================================================
-- Returns ACTIVE_TOKEN candidates where mint is not already discovered

CREATE OR REPLACE VIEW v_active_token_candidates AS
SELECT
    s.candidate_id,
    s.source,
    s.mint,
    s.pool,
    s.tx_signature,
    s.event_index,
    s.slot,
    s.timestamp
FROM v_active_token_spike_detection s
WHERE NOT EXISTS (
    -- Exclude if this mint already has any candidate (NEW_TOKEN or ACTIVE_TOKEN)
    SELECT 1 FROM token_candidates tc
    JOIN token_metadata tm ON tc.candidate_id = tm.candidate_id
    WHERE tm.mint = s.mint
);

COMMENT ON VIEW v_active_token_candidates IS 'ACTIVE_TOKEN candidates for mints not yet discovered';

-- ============================================================================
-- Combined Discovery View
-- ============================================================================
-- Union of NEW_TOKEN and ACTIVE_TOKEN candidates for convenience

CREATE OR REPLACE VIEW v_all_discovery_candidates AS
SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, timestamp
FROM v_new_token_candidates
UNION ALL
SELECT candidate_id, source, mint, pool, tx_signature, event_index, slot, timestamp
FROM v_active_token_candidates;

COMMENT ON VIEW v_all_discovery_candidates IS 'Combined NEW_TOKEN and ACTIVE_TOKEN discovery candidates';

-- ============================================================================
-- Validation View
-- ============================================================================
-- Verifies candidate_id formula consistency
-- Requires first swap for each candidate to get event_index

CREATE OR REPLACE VIEW v_discovery_validation AS
WITH first_swap_per_candidate AS (
    SELECT DISTINCT ON (candidate_id)
        candidate_id,
        tx_signature,
        event_index,
        slot
    FROM swaps
    ORDER BY candidate_id, slot, tx_signature, event_index
)
SELECT
    tc.candidate_id AS stored_id,
    tc.source,
    tm.mint,
    tc.pool,
    fs.tx_signature,
    fs.event_index,
    tc.slot,
    encode(digest(
        (tm.mint || '|' || COALESCE(tc.pool, '') || '|' || tc.source || '|' ||
         fs.tx_signature || '|' || fs.event_index::text || '|' || tc.slot::text
        )::bytea,
        'sha256'
    ), 'hex') AS computed_id,
    tc.candidate_id = encode(digest(
        (tm.mint || '|' || COALESCE(tc.pool, '') || '|' || tc.source || '|' ||
         fs.tx_signature || '|' || fs.event_index::text || '|' || tc.slot::text
        )::bytea,
        'sha256'
    ), 'hex') AS id_matches
FROM token_candidates tc
JOIN token_metadata tm ON tc.candidate_id = tm.candidate_id
LEFT JOIN first_swap_per_candidate fs ON tc.candidate_id = fs.candidate_id;

COMMENT ON VIEW v_discovery_validation IS 'Validates candidate_id formula using actual event_index from swaps';
