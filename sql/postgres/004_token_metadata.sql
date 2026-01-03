-- Migration: 004_token_metadata
-- Description: Token metadata fetched from chain
-- Append-only: No UPDATE/DELETE allowed

CREATE TABLE IF NOT EXISTS token_metadata (
    candidate_id        TEXT PRIMARY KEY REFERENCES token_candidates(candidate_id),
    mint                TEXT NOT NULL,
    name                TEXT,
    symbol              TEXT,
    decimals            INTEGER NOT NULL,
    supply              NUMERIC,
    fetched_at          BIGINT NOT NULL,            -- Unix timestamp (ms)
    created_at          BIGINT NOT NULL DEFAULT (EXTRACT(EPOCH FROM NOW()) * 1000)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_token_metadata_mint ON token_metadata(mint);
CREATE INDEX IF NOT EXISTS idx_token_metadata_symbol ON token_metadata(symbol);

-- Append-only enforcement: raise exception on UPDATE and DELETE
DROP TRIGGER IF EXISTS token_metadata_no_update ON token_metadata;
CREATE TRIGGER token_metadata_no_update
    BEFORE UPDATE ON token_metadata
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

DROP TRIGGER IF EXISTS token_metadata_no_delete ON token_metadata;
CREATE TRIGGER token_metadata_no_delete
    BEFORE DELETE ON token_metadata
    FOR EACH ROW EXECUTE FUNCTION raise_append_only_violation();

COMMENT ON TABLE token_metadata IS 'Token metadata from on-chain. Append-only.';
COMMENT ON COLUMN token_metadata.candidate_id IS 'Reference to token_candidates.candidate_id';
COMMENT ON COLUMN token_metadata.mint IS 'Token mint address';
COMMENT ON COLUMN token_metadata.name IS 'Token name (may be NULL)';
COMMENT ON COLUMN token_metadata.symbol IS 'Token symbol (may be NULL)';
COMMENT ON COLUMN token_metadata.decimals IS 'Token decimals';
COMMENT ON COLUMN token_metadata.supply IS 'Total supply (may be NULL if not available)';
COMMENT ON COLUMN token_metadata.fetched_at IS 'Unix timestamp when metadata was fetched';
