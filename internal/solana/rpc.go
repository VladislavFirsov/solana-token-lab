package solana

import "context"

// RPCClient defines Solana RPC HTTP interface.
type RPCClient interface {
	// GetTransaction retrieves a transaction by signature.
	GetTransaction(ctx context.Context, signature string) (*Transaction, error)

	// GetBlock retrieves a block by slot number.
	GetBlock(ctx context.Context, slot int64) (*Block, error)

	// GetSignaturesForAddress retrieves signatures for an address with pagination.
	GetSignaturesForAddress(ctx context.Context, address string, opts *SignaturesOpts) ([]SignatureInfo, error)
}

// Transaction represents a Solana transaction.
type Transaction struct {
	Slot      int64
	Signature string
	BlockTime int64 // Unix timestamp (seconds)
	Meta      *TransactionMeta
	Message   *TransactionMessage
}

// TransactionMeta contains transaction metadata.
type TransactionMeta struct {
	Err         interface{}
	LogMessages []string
}

// TransactionMessage contains parsed transaction message.
type TransactionMessage struct {
	AccountKeys []string
}
