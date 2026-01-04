package solana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Default configuration values.
const (
	DefaultTimeout     = 30 * time.Second
	DefaultMaxRetries  = 3
	DefaultRetryDelay  = 1 * time.Second
	DefaultMaxDelay    = 10 * time.Second
	DefaultBackoffMult = 2.0
)

// HTTPClient implements RPCClient using HTTP JSON-RPC 2.0.
type HTTPClient struct {
	endpoint    string
	client      *http.Client
	maxRetries  int
	retryDelay  time.Duration
	maxDelay    time.Duration
	backoffMult float64
	requestID   atomic.Uint64
}

// ClientOption configures HTTPClient.
type ClientOption func(*HTTPClient)

// WithTimeout sets HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		c.client.Timeout = d
	}
}

// WithMaxRetries sets maximum retry attempts.
func WithMaxRetries(n int) ClientOption {
	return func(c *HTTPClient) {
		c.maxRetries = n
	}
}

// WithRetryDelay sets initial retry delay.
func WithRetryDelay(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		c.retryDelay = d
	}
}

// WithMaxDelay sets maximum retry delay.
func WithMaxDelay(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		c.maxDelay = d
	}
}

// WithHTTPClient sets custom http.Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *HTTPClient) {
		c.client = client
	}
}

// NewHTTPClient creates a new Solana RPC HTTP client.
func NewHTTPClient(endpoint string, opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		endpoint:    endpoint,
		client:      &http.Client{Timeout: DefaultTimeout},
		maxRetries:  DefaultMaxRetries,
		retryDelay:  DefaultRetryDelay,
		maxDelay:    DefaultMaxDelay,
		backoffMult: DefaultBackoffMult,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// rpcRequest represents a JSON-RPC 2.0 request.
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      uint64        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

// rpcResponse represents a JSON-RPC 2.0 response.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint64          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// rpcError represents a JSON-RPC 2.0 error.
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// call performs a JSON-RPC call with retries and exponential backoff.
func (c *HTTPClient) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	reqID := c.requestID.Add(1)
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	delay := c.retryDelay
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			// Exponential backoff
			delay = time.Duration(float64(delay) * c.backoffMult)
			if delay > c.maxDelay {
				delay = c.maxDelay
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		// Handle rate limiting
		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limited (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
			continue
		}

		var rpcResp rpcResponse
		if err := json.Unmarshal(respBody, &rpcResp); err != nil {
			lastErr = fmt.Errorf("unmarshal response: %w", err)
			continue
		}

		if rpcResp.Error != nil {
			// RPC errors are not retried
			return rpcResp.Error
		}

		if result != nil && rpcResp.Result != nil {
			if err := json.Unmarshal(rpcResp.Result, result); err != nil {
				return fmt.Errorf("unmarshal result: %w", err)
			}
		}

		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// GetTransaction retrieves a transaction by signature.
func (c *HTTPClient) GetTransaction(ctx context.Context, signature string) (*Transaction, error) {
	params := []interface{}{
		signature,
		map[string]interface{}{
			"encoding":                       "json",
			"maxSupportedTransactionVersion": 0,
		},
	}

	var result getTransactionResult
	if err := c.call(ctx, "getTransaction", params, &result); err != nil {
		return nil, err
	}

	if result.Slot == 0 && result.BlockTime == nil {
		// Transaction not found
		return nil, nil
	}

	tx := &Transaction{
		Slot:      result.Slot,
		Signature: signature,
	}

	if result.BlockTime != nil {
		tx.BlockTime = *result.BlockTime
	}

	if result.Meta != nil {
		tx.Meta = &TransactionMeta{
			Err:         result.Meta.Err,
			LogMessages: result.Meta.LogMessages,
		}
	}

	if result.Transaction != nil && result.Transaction.Message != nil {
		tx.Message = &TransactionMessage{
			AccountKeys: result.Transaction.Message.AccountKeys,
		}
	}

	return tx, nil
}

// getTransactionResult is the raw RPC response for getTransaction.
type getTransactionResult struct {
	Slot        int64                     `json:"slot"`
	BlockTime   *int64                    `json:"blockTime"`
	Meta        *getTransactionMeta       `json:"meta"`
	Transaction *getTransactionTx         `json:"transaction"`
}

type getTransactionMeta struct {
	Err         interface{} `json:"err"`
	LogMessages []string    `json:"logMessages"`
}

type getTransactionTx struct {
	Message *getTransactionMessage `json:"message"`
}

type getTransactionMessage struct {
	AccountKeys []string `json:"accountKeys"`
}

// GetBlock retrieves a block by slot number.
func (c *HTTPClient) GetBlock(ctx context.Context, slot int64) (*Block, error) {
	params := []interface{}{
		slot,
		map[string]interface{}{
			"encoding":                       "json",
			"transactionDetails":             "full",
			"maxSupportedTransactionVersion": 0,
		},
	}

	var result getBlockResult
	if err := c.call(ctx, "getBlock", params, &result); err != nil {
		return nil, err
	}

	block := &Block{
		Slot:      slot,
		BlockTime: result.BlockTime,
	}

	for _, txWrapper := range result.Transactions {
		tx := Transaction{
			Slot: slot,
		}
		if result.BlockTime != nil {
			tx.BlockTime = *result.BlockTime
		}

		// Extract signature from transaction
		if len(txWrapper.Transaction.Signatures) > 0 {
			tx.Signature = txWrapper.Transaction.Signatures[0]
		}

		if txWrapper.Meta != nil {
			tx.Meta = &TransactionMeta{
				Err:         txWrapper.Meta.Err,
				LogMessages: txWrapper.Meta.LogMessages,
			}
		}

		if txWrapper.Transaction.Message != nil {
			tx.Message = &TransactionMessage{
				AccountKeys: txWrapper.Transaction.Message.AccountKeys,
			}
		}

		block.Transactions = append(block.Transactions, tx)
	}

	return block, nil
}

// getBlockResult is the raw RPC response for getBlock.
type getBlockResult struct {
	BlockTime    *int64               `json:"blockTime"`
	Transactions []getBlockTxWrapper  `json:"transactions"`
}

type getBlockTxWrapper struct {
	Transaction getBlockTx         `json:"transaction"`
	Meta        *getTransactionMeta `json:"meta"`
}

type getBlockTx struct {
	Signatures []string              `json:"signatures"`
	Message    *getTransactionMessage `json:"message"`
}

// GetSignaturesForAddress retrieves signatures for an address with pagination.
func (c *HTTPClient) GetSignaturesForAddress(ctx context.Context, address string, opts *SignaturesOpts) ([]SignatureInfo, error) {
	config := make(map[string]interface{})
	if opts != nil {
		if opts.Before != "" {
			config["before"] = opts.Before
		}
		if opts.Until != "" {
			config["until"] = opts.Until
		}
		if opts.Limit > 0 {
			config["limit"] = opts.Limit
		}
	}

	params := []interface{}{address}
	if len(config) > 0 {
		params = append(params, config)
	}

	var result []getSignaturesResult
	if err := c.call(ctx, "getSignaturesForAddress", params, &result); err != nil {
		return nil, err
	}

	sigs := make([]SignatureInfo, len(result))
	for i, r := range result {
		sigs[i] = SignatureInfo{
			Signature: r.Signature,
			Slot:      r.Slot,
			BlockTime: r.BlockTime,
			Err:       r.Err,
		}
	}

	return sigs, nil
}

// getSignaturesResult is the raw RPC response item for getSignaturesForAddress.
type getSignaturesResult struct {
	Signature string      `json:"signature"`
	Slot      int64       `json:"slot"`
	BlockTime *int64      `json:"blockTime"`
	Err       interface{} `json:"err"`
}

// GetAccountInfo retrieves account info by public key.
// Returns nil if account not found.
func (c *HTTPClient) GetAccountInfo(ctx context.Context, pubkey string) (*AccountInfo, error) {
	params := []interface{}{
		pubkey,
		map[string]interface{}{
			"encoding": "base64",
		},
	}

	var result getAccountInfoResult
	if err := c.call(ctx, "getAccountInfo", params, &result); err != nil {
		return nil, err
	}

	if result.Value == nil {
		return nil, nil
	}

	info := &AccountInfo{
		Lamports:   result.Value.Lamports,
		Owner:      result.Value.Owner,
		Executable: result.Value.Executable,
		RentEpoch:  result.Value.RentEpoch,
	}

	if len(result.Value.Data) >= 1 {
		info.Data = result.Value.Data[0]
	}

	return info, nil
}

// AccountInfo represents Solana account information.
type AccountInfo struct {
	Lamports   uint64 `json:"lamports"`
	Owner      string `json:"owner"`
	Data       string `json:"data"` // base64 encoded
	Executable bool   `json:"executable"`
	RentEpoch  uint64 `json:"rentEpoch"`
}

type getAccountInfoResult struct {
	Value *getAccountInfoValue `json:"value"`
}

type getAccountInfoValue struct {
	Lamports   uint64   `json:"lamports"`
	Owner      string   `json:"owner"`
	Data       []string `json:"data"` // [base64_data, encoding]
	Executable bool     `json:"executable"`
	RentEpoch  uint64   `json:"rentEpoch"`
}

// GetSlot retrieves the current slot.
func (c *HTTPClient) GetSlot(ctx context.Context) (int64, error) {
	var result int64
	if err := c.call(ctx, "getSlot", nil, &result); err != nil {
		return 0, err
	}
	return result, nil
}

// GetBlockTime retrieves the estimated production time of a block.
func (c *HTTPClient) GetBlockTime(ctx context.Context, slot int64) (*int64, error) {
	params := []interface{}{slot}
	var result *int64
	if err := c.call(ctx, "getBlockTime", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}
