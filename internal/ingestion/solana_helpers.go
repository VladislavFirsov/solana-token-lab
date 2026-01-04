package ingestion

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/mr-tron/base58"

	"solana-token-lab/internal/solana"
)

const wsolMint = "So11111111111111111111111111111111111111112"

// resolveBlockTimestamp returns a deterministic timestamp in ms for a slot/blockTime.
func resolveBlockTimestamp(ctx context.Context, rpc *solana.HTTPClient, slot int64, blockTime int64) (int64, error) {
	if blockTime > 0 {
		return blockTime * 1000, nil
	}
	if rpc == nil || slot <= 0 {
		return 0, fmt.Errorf("missing block time and slot")
	}
	bt, err := rpc.GetBlockTime(ctx, slot)
	if err != nil {
		return 0, err
	}
	if bt == nil {
		return 0, fmt.Errorf("block time not available for slot %d", slot)
	}
	return *bt * 1000, nil
}

// fetchTokenAccountMint retrieves the mint address for a token account.
func fetchTokenAccountMint(ctx context.Context, rpc *solana.HTTPClient, tokenAccount string) (string, error) {
	if rpc == nil || tokenAccount == "" {
		return "", nil
	}
	info, err := rpc.GetAccountInfo(ctx, tokenAccount)
	if err != nil {
		return "", err
	}
	if info == nil || info.Data == "" {
		return "", nil
	}
	return parseTokenAccountMint(info.Data)
}

// parseTokenAccountMint parses SPL token account data and returns the mint address.
// Token account layout: mint(32) | owner(32) | amount(8) | ...
func parseTokenAccountMint(data string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("decode token account data: %w", err)
	}
	if len(decoded) < 32 {
		return "", fmt.Errorf("token account data too short: %d", len(decoded))
	}
	return base58.Encode(decoded[:32]), nil
}

// inferRaydiumPoolAndMint extracts pool and token mint from Raydium swap accounts.
func inferRaydiumPoolAndMint(ctx context.Context, rpc *solana.HTTPClient, accountKeys []string) (string, string, error) {
	if len(accountKeys) < 7 {
		return "", "", nil
	}

	pool := accountKeys[1]
	tokenAccounts := []string{accountKeys[5], accountKeys[6]}

	for _, acct := range tokenAccounts {
		mint, err := fetchTokenAccountMint(ctx, rpc, acct)
		if err != nil {
			continue
		}
		if mint != "" && mint != wsolMint {
			return pool, mint, nil
		}
	}

	// If only WSOL found, return it to avoid empty mint.
	for _, acct := range tokenAccounts {
		mint, err := fetchTokenAccountMint(ctx, rpc, acct)
		if err == nil && mint != "" {
			return pool, mint, nil
		}
	}

	return pool, "", nil
}
