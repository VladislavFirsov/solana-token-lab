package domain

// LiquidityEvent represents a liquidity add/remove event.
// Corresponds to liquidity_events table in PostgreSQL.
type LiquidityEvent struct {
	ID             int64   // BIGSERIAL primary key
	CandidateID    string  // FK to token_candidates
	Pool           string  // Pool address (AMM ID)
	Mint           string  // Token mint address
	TxSignature    string  // Solana transaction signature
	EventIndex     int     // index of event within transaction
	Slot           int64   // Solana slot number
	Timestamp      int64   // Unix timestamp in milliseconds
	EventType      string  // "add" | "remove"
	AmountToken    float64 // token amount added/removed
	AmountQuote    float64 // quote currency (SOL/USDC) amount
	LiquidityAfter float64 // total pool liquidity after event
	CreatedAt      int64   // record creation timestamp (ms)
}

// Liquidity event type constants
const (
	LiquidityEventAdd    = "add"
	LiquidityEventRemove = "remove"
)
