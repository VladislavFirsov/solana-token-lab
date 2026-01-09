-- Discovery progress tables for resumption after restarts.
-- Prevents duplicate candidates and enables 7-day continuous detection.

-- Single-row table for last processed position
CREATE TABLE IF NOT EXISTS discovery_progress (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    slot BIGINT NOT NULL,
    signature VARCHAR(128) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Set of processed mint addresses
CREATE TABLE IF NOT EXISTS discovery_seen_mints (
    mint VARCHAR(64) PRIMARY KEY,
    seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for efficient lookup
CREATE INDEX IF NOT EXISTS idx_discovery_seen_mints_seen_at
    ON discovery_seen_mints(seen_at);
