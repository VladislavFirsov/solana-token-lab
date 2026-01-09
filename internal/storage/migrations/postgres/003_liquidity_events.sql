-- Migration: 003_liquidity_events
-- Description: Liquidity add/remove events for token candidates
-- Append-only: No UPDATE/DELETE allowed

CREATE TABLE IF NOT EXISTS liquidity_events (
    id                  BIGSERIAL PRIMARY KEY,
    candidate_id        TEXT NOT NULL REFERENCES token_candidates(candidate_id),
    tx_signature        TEXT NOT NULL,
    event_index         INTEGER NOT NULL,           -- instruction/log index within tx
    slot                BIGINT NOT NULL,
    timestamp           BIGINT NOT NULL,            -- Unix timestamp (ms)
    event_type          TEXT NOT NULL,              -- 'add' | 'remove'
    amount_token        NUMERIC NOT NULL,
    amount_quote        NUMERIC NOT NULL,           -- SOL or USDC
    liquidity_after     NUMERIC NOT NULL,           -- pool liquidity after event
    created_at          BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM NOW()) * 1000),

    CONSTRAINT chk_event_type CHECK (event_type IN ('add', 'remove')),
    CONSTRAINT uq_liquidity_events_tx_event UNIQUE (tx_signature, candidate_id, event_index)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_liquidity_events_candidate_id ON liquidity_events(candidate_id);
CREATE INDEX IF NOT EXISTS idx_liquidity_events_slot ON liquidity_events(slot);
CREATE INDEX IF NOT EXISTS idx_liquidity_events_timestamp ON liquidity_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_liquidity_events_candidate_timestamp ON liquidity_events(candidate_id, timestamp);

-- Append-only enforcement: raise exception on UPDATE and DELETE
DROP TRIGGER IF EXISTS liquidity_events_no_update ON liquidity_events;
CREATE TRIGGER liquidity_events_no_update
    BEFORE UPDATE ON liquidity_events
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

DROP TRIGGER IF EXISTS liquidity_events_no_delete ON liquidity_events;
CREATE TRIGGER liquidity_events_no_delete
    BEFORE DELETE ON liquidity_events
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

COMMENT ON TABLE liquidity_events IS 'Liquidity add/remove events for token candidates. Append-only.';
COMMENT ON COLUMN liquidity_events.candidate_id IS 'Reference to token_candidates.candidate_id';
COMMENT ON COLUMN liquidity_events.tx_signature IS 'Solana transaction signature';
COMMENT ON COLUMN liquidity_events.event_index IS 'Index of event within transaction';
COMMENT ON COLUMN liquidity_events.slot IS 'Solana slot number';
COMMENT ON COLUMN liquidity_events.timestamp IS 'Unix timestamp in milliseconds';
COMMENT ON COLUMN liquidity_events.event_type IS 'Event type: add or remove';
COMMENT ON COLUMN liquidity_events.amount_token IS 'Amount of token added/removed';
COMMENT ON COLUMN liquidity_events.amount_quote IS 'Amount of quote currency (SOL/USDC) added/removed';
COMMENT ON COLUMN liquidity_events.liquidity_after IS 'Total pool liquidity after this event';
