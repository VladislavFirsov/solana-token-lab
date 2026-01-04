package solana

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPClient_GetTransaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if req.Method != "getTransaction" {
			t.Errorf("expected method getTransaction, got %s", req.Method)
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"slot":      int64(123456),
				"blockTime": int64(1700000000),
				"meta": map[string]interface{}{
					"err":         nil,
					"logMessages": []string{"Program log: Hello", "Program log: World"},
				},
				"transaction": map[string]interface{}{
					"message": map[string]interface{}{
						"accountKeys": []string{"addr1", "addr2"},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	tx, err := client.GetTransaction(ctx, "testsig123")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}

	if tx == nil {
		t.Fatal("expected transaction, got nil")
	}

	if tx.Slot != 123456 {
		t.Errorf("expected slot 123456, got %d", tx.Slot)
	}

	if tx.BlockTime != 1700000000 {
		t.Errorf("expected blockTime 1700000000, got %d", tx.BlockTime)
	}

	if tx.Meta == nil {
		t.Fatal("expected meta, got nil")
	}

	if len(tx.Meta.LogMessages) != 2 {
		t.Errorf("expected 2 log messages, got %d", len(tx.Meta.LogMessages))
	}

	if tx.Message == nil {
		t.Fatal("expected message, got nil")
	}

	if len(tx.Message.AccountKeys) != 2 {
		t.Errorf("expected 2 account keys, got %d", len(tx.Message.AccountKeys))
	}
}

func TestHTTPClient_GetTransaction_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  nil,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	tx, err := client.GetTransaction(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetTransaction: %v", err)
	}

	if tx != nil {
		t.Errorf("expected nil for not found, got %+v", tx)
	}
}

func TestHTTPClient_GetSignaturesForAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "getSignaturesForAddress" {
			t.Errorf("expected method getSignaturesForAddress, got %s", req.Method)
		}

		blockTime := int64(1700000000)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": []map[string]interface{}{
				{"signature": "sig1", "slot": int64(100), "blockTime": blockTime, "err": nil},
				{"signature": "sig2", "slot": int64(101), "blockTime": blockTime, "err": nil},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	sigs, err := client.GetSignaturesForAddress(ctx, "testaddr", &SignaturesOpts{Limit: 10})
	if err != nil {
		t.Fatalf("GetSignaturesForAddress: %v", err)
	}

	if len(sigs) != 2 {
		t.Fatalf("expected 2 signatures, got %d", len(sigs))
	}

	if sigs[0].Signature != "sig1" {
		t.Errorf("expected sig1, got %s", sigs[0].Signature)
	}

	if sigs[1].Slot != 101 {
		t.Errorf("expected slot 101, got %d", sigs[1].Slot)
	}
}

func TestHTTPClient_GetBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "getBlock" {
			t.Errorf("expected method getBlock, got %s", req.Method)
		}

		blockTime := int64(1700000000)
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"blockTime": blockTime,
				"transactions": []map[string]interface{}{
					{
						"transaction": map[string]interface{}{
							"signatures": []string{"sig1"},
							"message": map[string]interface{}{
								"accountKeys": []string{"addr1"},
							},
						},
						"meta": map[string]interface{}{
							"err":         nil,
							"logMessages": []string{"log1"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	block, err := client.GetBlock(ctx, 12345)
	if err != nil {
		t.Fatalf("GetBlock: %v", err)
	}

	if block.Slot != 12345 {
		t.Errorf("expected slot 12345, got %d", block.Slot)
	}

	if block.BlockTime == nil || *block.BlockTime != 1700000000 {
		t.Errorf("expected blockTime 1700000000")
	}

	if len(block.Transactions) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(block.Transactions))
	}

	if block.Transactions[0].Signature != "sig1" {
		t.Errorf("expected sig1, got %s", block.Transactions[0].Signature)
	}
}

func TestHTTPClient_Retry(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  int64(999),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL,
		WithMaxRetries(3),
		WithRetryDelay(10*time.Millisecond),
	)
	ctx := context.Background()

	slot, err := client.GetSlot(ctx)
	if err != nil {
		t.Fatalf("GetSlot: %v", err)
	}

	if slot != 999 {
		t.Errorf("expected slot 999, got %d", slot)
	}

	if attempts.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestHTTPClient_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]interface{}{
				"code":    -32600,
				"message": "Invalid Request",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	_, err := client.GetSlot(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	rpcErr, ok := err.(*rpcError)
	if !ok {
		t.Fatalf("expected rpcError, got %T", err)
	}

	if rpcErr.Code != -32600 {
		t.Errorf("expected code -32600, got %d", rpcErr.Code)
	}
}

func TestHTTPClient_GetAccountInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Method != "getAccountInfo" {
			t.Errorf("expected method getAccountInfo, got %s", req.Method)
		}

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"value": map[string]interface{}{
					"lamports":   uint64(1000000),
					"owner":      "11111111111111111111111111111111",
					"data":       []string{"SGVsbG8gV29ybGQ=", "base64"},
					"executable": false,
					"rentEpoch":  uint64(100),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	info, err := client.GetAccountInfo(ctx, "testpubkey")
	if err != nil {
		t.Fatalf("GetAccountInfo: %v", err)
	}

	if info == nil {
		t.Fatal("expected account info, got nil")
	}

	if info.Lamports != 1000000 {
		t.Errorf("expected lamports 1000000, got %d", info.Lamports)
	}

	if info.Owner != "11111111111111111111111111111111" {
		t.Errorf("unexpected owner: %s", info.Owner)
	}

	if info.Data != "SGVsbG8gV29ybGQ=" {
		t.Errorf("unexpected data: %s", info.Data)
	}
}

func TestHTTPClient_GetAccountInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]interface{}{
				"value": nil,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx := context.Background()

	info, err := client.GetAccountInfo(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetAccountInfo: %v", err)
	}

	if info != nil {
		t.Errorf("expected nil for not found, got %+v", info)
	}
}

func TestHTTPClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient(server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetSlot(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
