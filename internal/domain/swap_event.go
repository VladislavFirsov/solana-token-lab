package domain

// SwapEvent represents a raw swap event for discovery (not tied to candidate_id).
// Used for ACTIVE_TOKEN detection before a token becomes a candidate.
type SwapEvent struct {
	Mint        string  // token mint address
	Pool        *string // pool address (nullable)
	TxSignature string  // transaction signature
	EventIndex  int     // index within transaction
	Slot        int64   // Solana slot number
	Timestamp   int64   // Unix timestamp in milliseconds
	AmountOut   float64 // output amount for volume calculations
}
