package domain

// TokenCandidate represents a discovered token candidate for analysis.
// Corresponds to token_candidates table in PostgreSQL.
type TokenCandidate struct {
	CandidateID  string  // PRIMARY KEY, deterministic hash
	Source       Source  // NEW_TOKEN | ACTIVE_TOKEN
	Mint         string  // token mint address
	Pool         *string // pool address (nullable)
	TxSignature  string  // discovery transaction signature
	Slot         int64   // Solana slot number
	DiscoveredAt int64   // Unix timestamp in milliseconds
	CreatedAt    int64   // record creation timestamp (ms)
}
