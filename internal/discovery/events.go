package discovery

// SwapEvent represents a parsed swap from transaction logs.
type SwapEvent struct {
	Mint        string  // Token mint address
	Pool        *string // Pool address (nullable)
	TxSignature string  // Transaction signature
	EventIndex  int     // Index of swap within transaction
	Slot        int64   // Solana slot number
	Timestamp   int64   // Unix timestamp in milliseconds
}
