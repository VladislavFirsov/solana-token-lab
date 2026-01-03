-- Migration: 001_token_candidates
-- Description: Token candidates table (NEW_TOKEN and ACTIVE_TOKEN discovery)
-- Append-only: No UPDATE/DELETE allowed

CREATE TABLE IF NOT EXISTS token_candidates (
    candidate_id        TEXT PRIMARY KEY,           -- deterministic hash-based ID
    source              TEXT NOT NULL,              -- 'NEW_TOKEN' | 'ACTIVE_TOKEN'
    mint                TEXT NOT NULL,              -- token mint address
    pool                TEXT,                       -- pool address (nullable for pre-pool)
    tx_signature        TEXT NOT NULL,              -- discovery transaction signature
    slot                BIGINT NOT NULL,            -- Solana slot number
    discovered_at       BIGINT NOT NULL,            -- Unix timestamp (ms)
    created_at          BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM NOW()) * 1000),

    CONSTRAINT chk_source CHECK (source IN ('NEW_TOKEN', 'ACTIVE_TOKEN'))
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_token_candidates_source ON token_candidates(source);
CREATE INDEX IF NOT EXISTS idx_token_candidates_slot ON token_candidates(slot);
CREATE INDEX IF NOT EXISTS idx_token_candidates_discovered_at ON token_candidates(discovered_at);
CREATE INDEX IF NOT EXISTS idx_token_candidates_mint ON token_candidates(mint);

-- Append-only enforcement: raise exception on UPDATE and DELETE
CREATE OR REPLACE FUNCTION raise_append_only_violation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Table % is append-only. UPDATE and DELETE are prohibited.', TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS token_candidates_no_update ON token_candidates;
CREATE TRIGGER token_candidates_no_update
    BEFORE UPDATE ON token_candidates
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

DROP TRIGGER IF EXISTS token_candidates_no_delete ON token_candidates;
CREATE TRIGGER token_candidates_no_delete
    BEFORE DELETE ON token_candidates
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

COMMENT ON TABLE token_candidates IS 'Discovered token candidates for analysis. Append-only.';
COMMENT ON COLUMN token_candidates.candidate_id IS 'Deterministic hash-based unique identifier';
COMMENT ON COLUMN token_candidates.source IS 'Discovery source: NEW_TOKEN or ACTIVE_TOKEN';
COMMENT ON COLUMN token_candidates.mint IS 'Solana token mint address';
COMMENT ON COLUMN token_candidates.pool IS 'Pool address (may be NULL if discovered before pool creation)';
COMMENT ON COLUMN token_candidates.tx_signature IS 'Transaction signature of discovery event';
COMMENT ON COLUMN token_candidates.slot IS 'Solana slot number of discovery';
COMMENT ON COLUMN token_candidates.discovered_at IS 'Unix timestamp in milliseconds';
