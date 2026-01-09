-- Migration: 009_liquidity_nullable_candidate
-- Description: Allow liquidity events without candidate_id for deferred association
-- This enables storing events before candidate discovery and later association via mint/pool

-- Drop the foreign key constraint first
ALTER TABLE liquidity_events DROP CONSTRAINT IF EXISTS liquidity_events_candidate_id_fkey;

-- Make candidate_id nullable
ALTER TABLE liquidity_events ALTER COLUMN candidate_id DROP NOT NULL;

-- Drop old unique constraint that requires candidate_id
ALTER TABLE liquidity_events DROP CONSTRAINT IF EXISTS uq_liquidity_events_tx_event;

-- Create new unique constraint using tx_signature + event_index + COALESCE(mint, pool)
-- This ensures deduplication even without candidate_id
CREATE UNIQUE INDEX IF NOT EXISTS uq_liquidity_events_tx_mint_event
    ON liquidity_events(tx_signature, event_index, COALESCE(mint, pool, ''));

-- Add index for deferred association queries (find events by mint without candidate_id)
CREATE INDEX IF NOT EXISTS idx_liquidity_events_mint_no_candidate
    ON liquidity_events(mint) WHERE candidate_id IS NULL;

COMMENT ON COLUMN liquidity_events.candidate_id IS 'Reference to token_candidates.candidate_id (nullable for deferred association)';
