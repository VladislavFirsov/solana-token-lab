-- Migration: 006_swap_events
-- Description: Discovery swap events table for ACTIVE_TOKEN detection
-- Append-only: No UPDATE/DELETE allowed

CREATE TABLE IF NOT EXISTS swap_events (
    mint                TEXT NOT NULL,              -- token mint address
    pool                TEXT,                       -- pool address (nullable)
    tx_signature        TEXT NOT NULL,              -- transaction signature
    event_index         INTEGER NOT NULL,           -- index within transaction
    slot                BIGINT NOT NULL,            -- Solana slot number
    timestamp           BIGINT NOT NULL,            -- Unix timestamp (ms)
    amount_out          NUMERIC NOT NULL,           -- output amount for volume

    PRIMARY KEY (mint, tx_signature, event_index)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_swap_events_timestamp ON swap_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_swap_events_mint_timestamp ON swap_events(mint, timestamp);
CREATE INDEX IF NOT EXISTS idx_swap_events_slot ON swap_events(slot);

-- Append-only enforcement: raise exception on UPDATE and DELETE
DROP TRIGGER IF EXISTS swap_events_no_update ON swap_events;
CREATE TRIGGER swap_events_no_update
    BEFORE UPDATE ON swap_events
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

DROP TRIGGER IF EXISTS swap_events_no_delete ON swap_events;
CREATE TRIGGER swap_events_no_delete
    BEFORE DELETE ON swap_events
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

COMMENT ON TABLE swap_events IS 'Discovery swap events for ACTIVE_TOKEN detection. Append-only.';
COMMENT ON COLUMN swap_events.mint IS 'Token mint address';
COMMENT ON COLUMN swap_events.pool IS 'Pool address (may be NULL)';
COMMENT ON COLUMN swap_events.tx_signature IS 'Transaction signature';
COMMENT ON COLUMN swap_events.event_index IS 'Index of swap within transaction';
COMMENT ON COLUMN swap_events.slot IS 'Solana slot number';
COMMENT ON COLUMN swap_events.timestamp IS 'Unix timestamp in milliseconds';
COMMENT ON COLUMN swap_events.amount_out IS 'Output amount for volume calculations';
