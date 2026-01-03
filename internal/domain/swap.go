package domain

// Swap represents a swap transaction for a token candidate.
// Corresponds to swaps table in PostgreSQL.
type Swap struct {
	ID          int64   // BIGSERIAL primary key
	CandidateID string  // FK to token_candidates
	TxSignature string  // Solana transaction signature
	EventIndex  int     // index of swap within transaction
	Slot        int64   // Solana slot number
	Timestamp   int64   // Unix timestamp in milliseconds
	Side        string  // "buy" | "sell"
	AmountIn    float64 // input token amount
	AmountOut   float64 // output token amount
	Price       float64 // execution price
	CreatedAt   int64   // record creation timestamp (ms)
}

// Swap side constants
const (
	SwapSideBuy  = "buy"
	SwapSideSell = "sell"
)
