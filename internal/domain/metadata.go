package domain

// TokenMetadata represents token metadata from on-chain.
// Corresponds to token_metadata table in PostgreSQL.
type TokenMetadata struct {
	CandidateID string   // PK + FK to token_candidates
	Mint        string   // token mint address
	Name        *string  // token name (nullable)
	Symbol      *string  // token symbol (nullable)
	Decimals    int      // token decimals
	Supply      *float64 // total supply (nullable)
	FetchedAt   int64    // when metadata was fetched (ms)
	CreatedAt   int64    // record creation timestamp (ms)
}
