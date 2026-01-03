package stub

import (
	"context"
	"errors"

	"solana-token-lab/internal/solana"
)

// ErrNotFound is returned when a transaction or block is not found.
var ErrNotFound = errors.New("not found")

// RPCClient implements solana.RPCClient for testing.
type RPCClient struct {
	Transactions map[string]*solana.Transaction
	Blocks       map[int64]*solana.Block
	Signatures   map[string][]solana.SignatureInfo
}

// NewRPCClient creates a new stub RPC client.
func NewRPCClient() *RPCClient {
	return &RPCClient{
		Transactions: make(map[string]*solana.Transaction),
		Blocks:       make(map[int64]*solana.Block),
		Signatures:   make(map[string][]solana.SignatureInfo),
	}
}

// GetTransaction retrieves a transaction by signature from the stub store.
func (c *RPCClient) GetTransaction(_ context.Context, signature string) (*solana.Transaction, error) {
	tx, ok := c.Transactions[signature]
	if !ok {
		return nil, ErrNotFound
	}
	return tx, nil
}

// GetBlock retrieves a block by slot from the stub store.
func (c *RPCClient) GetBlock(_ context.Context, slot int64) (*solana.Block, error) {
	block, ok := c.Blocks[slot]
	if !ok {
		return nil, ErrNotFound
	}
	return block, nil
}

// GetSignaturesForAddress retrieves signatures for an address from the stub store.
func (c *RPCClient) GetSignaturesForAddress(_ context.Context, address string, opts *solana.SignaturesOpts) ([]solana.SignatureInfo, error) {
	sigs, ok := c.Signatures[address]
	if !ok {
		return nil, nil
	}

	// Apply limit if specified
	if opts != nil && opts.Limit > 0 && opts.Limit < len(sigs) {
		return sigs[:opts.Limit], nil
	}

	return sigs, nil
}

// AddTransaction adds a transaction to the stub store.
func (c *RPCClient) AddTransaction(tx *solana.Transaction) {
	c.Transactions[tx.Signature] = tx
}

// AddBlock adds a block to the stub store.
func (c *RPCClient) AddBlock(block *solana.Block) {
	c.Blocks[block.Slot] = block
}

// AddSignatures adds signatures for an address to the stub store.
func (c *RPCClient) AddSignatures(address string, sigs []solana.SignatureInfo) {
	c.Signatures[address] = sigs
}
