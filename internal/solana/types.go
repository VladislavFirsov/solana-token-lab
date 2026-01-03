package solana

// SignatureInfo from getSignaturesForAddress.
type SignatureInfo struct {
	Signature string
	Slot      int64
	BlockTime *int64
	Err       interface{}
}

// SignaturesOpts defines optional pagination parameters for getSignaturesForAddress.
type SignaturesOpts struct {
	Before string // Start searching backwards from this signature
	Until  string // Search until this signature
	Limit  int    // Maximum number of signatures to return
}

// Block represents a Solana block.
type Block struct {
	Slot         int64
	BlockTime    *int64
	Transactions []Transaction
}
