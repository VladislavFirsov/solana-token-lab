-- Migration: 002_swaps
-- Description: Swap transactions for token candidates
-- Append-only: No UPDATE/DELETE allowed

CREATE TABLE IF NOT EXISTS swaps (
    id                  BIGSERIAL PRIMARY KEY,
    candidate_id        TEXT NOT NULL REFERENCES token_candidates(candidate_id),
    tx_signature        TEXT NOT NULL,
    event_index         INTEGER NOT NULL,           -- instruction/log index within tx
    slot                BIGINT NOT NULL,
    timestamp           BIGINT NOT NULL,            -- Unix timestamp (ms)
    side                TEXT NOT NULL,              -- 'buy' | 'sell'
    amount_in           NUMERIC NOT NULL,
    amount_out          NUMERIC NOT NULL,
    price               NUMERIC NOT NULL,
    created_at          BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM NOW()) * 1000),

    CONSTRAINT chk_side CHECK (side IN ('buy', 'sell')),
    CONSTRAINT uq_swaps_tx_event UNIQUE (tx_signature, candidate_id, event_index)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_swaps_candidate_id ON swaps(candidate_id);
CREATE INDEX IF NOT EXISTS idx_swaps_slot ON swaps(slot);
CREATE INDEX IF NOT EXISTS idx_swaps_timestamp ON swaps(timestamp);
CREATE INDEX IF NOT EXISTS idx_swaps_candidate_timestamp ON swaps(candidate_id, timestamp);

-- Append-only enforcement: raise exception on UPDATE and DELETE
DROP TRIGGER IF EXISTS swaps_no_update ON swaps;
CREATE TRIGGER swaps_no_update
    BEFORE UPDATE ON swaps
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

DROP TRIGGER IF EXISTS swaps_no_delete ON swaps;
CREATE TRIGGER swaps_no_delete
    BEFORE DELETE ON swaps
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

COMMENT ON TABLE swaps IS 'Swap transactions for token candidates. Append-only.';
COMMENT ON COLUMN swaps.candidate_id IS 'Reference to token_candidates.candidate_id';
COMMENT ON COLUMN swaps.tx_signature IS 'Solana transaction signature';
COMMENT ON COLUMN swaps.event_index IS 'Index of swap event within transaction (for multiple swaps in one tx)';
COMMENT ON COLUMN swaps.slot IS 'Solana slot number';
COMMENT ON COLUMN swaps.timestamp IS 'Unix timestamp in milliseconds';
COMMENT ON COLUMN swaps.side IS 'Trade direction: buy or sell';
COMMENT ON COLUMN swaps.amount_in IS 'Amount of input token';
COMMENT ON COLUMN swaps.amount_out IS 'Amount of output token';
COMMENT ON COLUMN swaps.price IS 'Execution price';
