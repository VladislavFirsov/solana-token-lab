-- Migration: 008_liquidity_pool_mint
-- Description: Add pool and mint columns to liquidity_events for direct queries
-- Required for filtering by pool/mint without candidate lookup

ALTER TABLE liquidity_events ADD COLUMN IF NOT EXISTS pool TEXT;
ALTER TABLE liquidity_events ADD COLUMN IF NOT EXISTS mint TEXT;

-- Indexes for querying by pool/mint
CREATE INDEX IF NOT EXISTS idx_liquidity_events_pool ON liquidity_events(pool);
CREATE INDEX IF NOT EXISTS idx_liquidity_events_mint ON liquidity_events(mint);

COMMENT ON COLUMN liquidity_events.pool IS 'DEX pool address';
COMMENT ON COLUMN liquidity_events.mint IS 'Token mint address';
