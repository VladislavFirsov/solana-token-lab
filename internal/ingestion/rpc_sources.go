package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"filippo.io/edwards25519"
	"github.com/mr-tron/base58"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
)

// RPCSwapEventSource fetches swap events from Solana RPC.
type RPCSwapEventSource struct {
	rpc      *solana.HTTPClient
	parser   *discovery.DEXParser
	programs []string // DEX program IDs to monitor
}

// NewRPCSwapEventSource creates a new RPC-based swap event source.
func NewRPCSwapEventSource(rpc *solana.HTTPClient, programs []string) *RPCSwapEventSource {
	return &RPCSwapEventSource{
		rpc:      rpc,
		parser:   discovery.NewDEXParser(),
		programs: programs,
	}
}

// Fetch returns swap events for a time range [from, to) in milliseconds.
// It queries each program for signatures and fetches transactions.
func (s *RPCSwapEventSource) Fetch(ctx context.Context, from, to int64) ([]*domain.SwapEvent, error) {
	var allEvents []*domain.SwapEvent

	for _, program := range s.programs {
		events, err := s.fetchForProgram(ctx, program, from, to)
		if err != nil {
			return nil, fmt.Errorf("fetch for program %s: %w", program, err)
		}
		allEvents = append(allEvents, events...)
	}

	// Sort by (slot, tx_signature, event_index) for deterministic ordering
	SortSwapEvents(allEvents)

	return allEvents, nil
}

// fetchForProgram fetches swap events for a single program.
func (s *RPCSwapEventSource) fetchForProgram(ctx context.Context, program string, from, to int64) ([]*domain.SwapEvent, error) {
	// Convert milliseconds to seconds for RPC
	fromSec := from / 1000
	toSec := to / 1000

	var allEvents []*domain.SwapEvent
	var before string

	for {
		// Get signatures for the program
		opts := &solana.SignaturesOpts{
			Limit: 1000,
		}
		if before != "" {
			opts.Before = before
		}

		sigs, err := s.rpc.GetSignaturesForAddress(ctx, program, opts)
		if err != nil {
			return nil, fmt.Errorf("get signatures: %w", err)
		}

		if len(sigs) == 0 {
			break
		}

		// Process each signature
		for _, sig := range sigs {
			// Skip if outside time range
			if sig.BlockTime == nil {
				continue
			}
			blockTime := *sig.BlockTime
			if blockTime < fromSec || blockTime >= toSec {
				// If we've gone past the time range, stop
				if blockTime < fromSec {
					return allEvents, nil
				}
				continue
			}

			// Skip failed transactions
			if sig.Err != nil {
				continue
			}

			// Fetch full transaction
			tx, err := s.rpc.GetTransaction(ctx, sig.Signature)
			if err != nil {
				return nil, fmt.Errorf("get transaction %s: %w", sig.Signature, err)
			}

			if tx == nil || tx.Meta == nil {
				continue
			}

			// Parse swap events from logs using V2 parser with account keys
			timestamp := blockTime * 1000 // Convert to milliseconds

			// Get account keys from transaction message
			var accountKeys []string
			if tx.Message != nil {
				accountKeys = tx.Message.AccountKeys
			}

			// Use V2 parser to extract mint/pool from account keys
			swapEvents := s.parser.ParseSwapEventsV2(
				tx.Meta.LogMessages,
				accountKeys,
				tx.Signature,
				tx.Slot,
				timestamp,
			)

			inferredPool, inferredMint := "", ""
			if needsRaydiumInference(tx.Meta.LogMessages, swapEvents) {
				pool, mint, err := inferRaydiumPoolAndMint(ctx, s.rpc, accountKeys)
				if err == nil {
					inferredPool = pool
					inferredMint = mint
				}
			}

			// Convert to domain.SwapEvent
			for _, se := range swapEvents {
				if se.Mint == "" && inferredMint != "" {
					se.Mint = inferredMint
				}
				if se.Pool == nil && inferredPool != "" {
					pool := inferredPool
					se.Pool = &pool
				}
				if se.Mint == "" {
					continue // Skip events without mint
				}
				event := &domain.SwapEvent{
					Mint:        se.Mint,
					Pool:        se.Pool,
					TxSignature: se.TxSignature,
					EventIndex:  se.EventIndex,
					Slot:        se.Slot,
					Timestamp:   se.Timestamp,
					AmountOut:   se.AmountOut,
				}
				allEvents = append(allEvents, event)
			}
		}

		// Paginate
		before = sigs[len(sigs)-1].Signature

		// Check if we should continue
		lastSig := sigs[len(sigs)-1]
		if lastSig.BlockTime != nil && *lastSig.BlockTime < fromSec {
			break
		}
	}

	return allEvents, nil
}

// RPCLiquidityEventSource fetches liquidity events from Solana RPC.
type RPCLiquidityEventSource struct {
	rpc      *solana.HTTPClient
	parser   *discovery.DEXParser
	programs []string
	candidates storage.CandidateStore
}

// NewRPCLiquidityEventSource creates a new RPC-based liquidity event source.
func NewRPCLiquidityEventSource(rpc *solana.HTTPClient, programs []string, stores ...storage.CandidateStore) *RPCLiquidityEventSource {
	var candidateStore storage.CandidateStore
	if len(stores) > 0 {
		candidateStore = stores[0]
	}

	return &RPCLiquidityEventSource{
		rpc:        rpc,
		parser:     discovery.NewDEXParser(),
		programs:   programs,
		candidates: candidateStore,
	}
}

// Fetch returns liquidity events for a candidate within time range.
func (s *RPCLiquidityEventSource) Fetch(ctx context.Context, candidateID string, from, to int64) ([]*domain.LiquidityEvent, error) {
	var allEvents []*domain.LiquidityEvent

	for _, program := range s.programs {
		events, err := s.fetchForProgram(ctx, program, candidateID, from, to)
		if err != nil {
			return nil, fmt.Errorf("fetch for program %s: %w", program, err)
		}
		allEvents = append(allEvents, events...)
	}

	// Sort for deterministic ordering
	SortLiquidityEvents(allEvents)

	return allEvents, nil
}

// fetchForProgram fetches liquidity events for a single program.
func (s *RPCLiquidityEventSource) fetchForProgram(ctx context.Context, program, candidateID string, from, to int64) ([]*domain.LiquidityEvent, error) {
	fromSec := from / 1000
	toSec := to / 1000

	// If candidateID is provided, resolve its mint for filtering
	// If candidate store is nil and candidateID is provided, we cannot verify mint - skip to avoid misassignment
	var filterMint string
	if candidateID != "" {
		if s.candidates == nil {
			// Cannot resolve mint without candidate store - skip filtering by candidateID
			// to avoid attributing all events to wrong candidate
			return nil, nil
		}
		candidate, err := s.candidates.GetByID(ctx, candidateID)
		if err == nil && candidate != nil {
			filterMint = candidate.Mint
		} else {
			// Candidate not found - cannot filter correctly
			return nil, nil
		}
	}

	var allEvents []*domain.LiquidityEvent
	var before string

	for {
		opts := &solana.SignaturesOpts{
			Limit: 1000,
		}
		if before != "" {
			opts.Before = before
		}

		sigs, err := s.rpc.GetSignaturesForAddress(ctx, program, opts)
		if err != nil {
			return nil, fmt.Errorf("get signatures: %w", err)
		}

		if len(sigs) == 0 {
			break
		}

		for _, sig := range sigs {
			if sig.BlockTime == nil {
				continue
			}
			blockTime := *sig.BlockTime
			if blockTime < fromSec || blockTime >= toSec {
				if blockTime < fromSec {
					return allEvents, nil
				}
				continue
			}

			if sig.Err != nil {
				continue
			}

			tx, err := s.rpc.GetTransaction(ctx, sig.Signature)
			if err != nil {
				return nil, fmt.Errorf("get transaction %s: %w", sig.Signature, err)
			}

			if tx == nil || tx.Meta == nil {
				continue
			}

			timestamp := blockTime * 1000

			// Get account keys for V2 parser
			var accountKeys []string
			if tx.Message != nil {
				accountKeys = tx.Message.AccountKeys
			}

			// Use V2 parser to extract Pool/Mint from account keys
			liqEvents := s.parser.ParseLiquidityEventsV2(
				tx.Meta.LogMessages,
				accountKeys,
				tx.Signature,
				tx.Slot,
				timestamp,
			)

			inferredPool, inferredMint := "", ""
			if needsRaydiumInference(tx.Meta.LogMessages, nil) {
				pool, mint, err := inferRaydiumPoolAndMint(ctx, s.rpc, accountKeys)
				if err == nil {
					inferredPool = pool
					inferredMint = mint
				}
			}

			for _, le := range liqEvents {
				if le.Mint == "" && inferredMint != "" {
					le.Mint = inferredMint
				}
				if le.Pool == "" && inferredPool != "" {
					le.Pool = inferredPool
				}

				// If we have a filter mint, skip events that don't match
				if filterMint != "" && le.Mint != filterMint {
					continue
				}

				// Try to resolve candidate ID, but allow events without it for deferred association
				resolvedCandidateID := candidateID
				if resolvedCandidateID == "" && s.candidates != nil && le.Mint != "" {
					id, err := resolveCandidateIDByMint(ctx, s.candidates, le.Mint)
					if err == nil {
						resolvedCandidateID = id
					}
				}
				// Skip events without mint/pool - we need at least one for deferred association
				if le.Mint == "" && le.Pool == "" {
					continue
				}
				event := &domain.LiquidityEvent{
					CandidateID: resolvedCandidateID, // May be empty for deferred association
					Pool:        le.Pool,
					Mint:        le.Mint,
					EventType:   le.EventType,
					AmountToken: float64(le.AmountToken),
					AmountQuote: float64(le.AmountQuote),
					TxSignature: le.TxSignature,
					EventIndex:  le.EventIndex,
					Slot:        le.Slot,
					Timestamp:   le.Timestamp,
				}
				allEvents = append(allEvents, event)
			}
		}

		before = sigs[len(sigs)-1].Signature
		lastSig := sigs[len(sigs)-1]
		if lastSig.BlockTime != nil && *lastSig.BlockTime < fromSec {
			break
		}
	}

	return allEvents, nil
}

// RPCMetadataSource fetches token metadata from Solana RPC.
type RPCMetadataSource struct {
	rpc *solana.HTTPClient
}

// NewRPCMetadataSource creates a new RPC-based metadata source.
func NewRPCMetadataSource(rpc *solana.HTTPClient) *RPCMetadataSource {
	return &RPCMetadataSource{rpc: rpc}
}

// Metaplex Token Metadata program ID
const metaplexProgramID = "metaqbxxUerdq28cj1RbAWkYQm3ybzjb6a8bt518x1s"

// Fetch returns token metadata for a given mint address.
// Fetches from both SPL Token Mint and Metaplex Metadata accounts.
func (s *RPCMetadataSource) Fetch(ctx context.Context, mint string) (*domain.TokenMetadata, error) {
	now := time.Now().UnixMilli()

	// Start with basic metadata
	meta := &domain.TokenMetadata{
		Mint:      mint,
		FetchedAt: now,
	}

	// 1. Fetch mint account for decimals and supply
	mintInfo, err := s.rpc.GetAccountInfo(ctx, mint)
	if err != nil {
		return nil, fmt.Errorf("get mint account info: %w", err)
	}

	if mintInfo == nil {
		return nil, nil // Mint not found
	}

	// Parse SPL Token Mint data
	if err := s.parseMintData(mintInfo.Data, meta); err != nil {
		// Log but continue - we can still try to get Metaplex metadata
		_ = err
	}

	// 2. Derive Metaplex metadata PDA and fetch name/symbol
	metadataPDA := s.deriveMetadataPDA(mint)
	if metadataPDA != "" {
		metaInfo, err := s.rpc.GetAccountInfo(ctx, metadataPDA)
		if err == nil && metaInfo != nil {
			s.parseMetaplexData(metaInfo.Data, meta)
		}
	}

	return meta, nil
}

// parseMintData parses SPL Token Mint account data.
// SPL Token Mint layout (82 bytes):
// - mintAuthority: Option<Pubkey> (36 bytes: 4 + 32)
// - supply: u64 (8 bytes)
// - decimals: u8 (1 byte)
// - isInitialized: bool (1 byte)
// - freezeAuthority: Option<Pubkey> (36 bytes: 4 + 32)
func (s *RPCMetadataSource) parseMintData(data string, meta *domain.TokenMetadata) error {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("decode mint data: %w", err)
	}

	// Minimum size for mint account
	if len(decoded) < 82 {
		return fmt.Errorf("mint data too short: %d", len(decoded))
	}

	// supply at offset 36 (after mintAuthority option)
	supply := binary.LittleEndian.Uint64(decoded[36:44])

	// decimals at offset 44
	decimals := int(decoded[44])

	meta.Decimals = decimals

	// Calculate supply as float64 adjusted for decimals
	supplyFloat := float64(supply) / math.Pow(10, float64(decimals))
	meta.Supply = &supplyFloat

	return nil
}

// deriveMetadataPDA derives the Metaplex metadata PDA for a given mint.
// Seeds: ["metadata", metaplex_program_id, mint]
func (s *RPCMetadataSource) deriveMetadataPDA(mint string) string {
	// Decode mint and program ID from base58 to bytes
	mintBytes, err := base58.Decode(mint)
	if err != nil {
		return ""
	}
	programBytes, err := base58.Decode(metaplexProgramID)
	if err != nil {
		return ""
	}

	if len(mintBytes) != 32 || len(programBytes) != 32 {
		return ""
	}

	// PDA derivation: sha256("metadata" || programID || mint || bump)
	// We need to find the valid bump (starting from 255, decreasing)
	// For simplicity, we'll try the common case first
	seeds := [][]byte{
		[]byte("metadata"),
		programBytes,
		mintBytes,
	}

	pda := derivePDA(seeds, programBytes)
	return pda
}

// parseMetaplexData parses Metaplex Token Metadata account data.
// Metaplex Metadata layout:
// - key: u8 (1 byte, should be 4 for MetadataV1)
// - updateAuthority: Pubkey (32 bytes)
// - mint: Pubkey (32 bytes)
// - name: String (4 + length bytes, max 32 chars)
// - symbol: String (4 + length bytes, max 10 chars)
// - uri: String (4 + length bytes, max 200 chars)
// ...and more fields
func (s *RPCMetadataSource) parseMetaplexData(data string, meta *domain.TokenMetadata) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return
	}

	// Minimum size check
	if len(decoded) < 100 {
		return
	}

	// Check metadata key
	if decoded[0] != 4 { // MetadataV1 key
		return
	}

	// Skip: key(1) + updateAuthority(32) + mint(32) = 65 bytes
	offset := 65

	// Parse name (borsh string: 4-byte length + data)
	if offset+4 > len(decoded) {
		return
	}
	nameLen := binary.LittleEndian.Uint32(decoded[offset:])
	offset += 4

	if nameLen > 100 || offset+int(nameLen) > len(decoded) {
		return
	}
	name := strings.TrimRight(string(decoded[offset:offset+int(nameLen)]), "\x00")
	offset += int(nameLen)
	if name != "" {
		meta.Name = &name
	}

	// Parse symbol
	if offset+4 > len(decoded) {
		return
	}
	symbolLen := binary.LittleEndian.Uint32(decoded[offset:])
	offset += 4

	if symbolLen > 20 || offset+int(symbolLen) > len(decoded) {
		return
	}
	symbol := strings.TrimRight(string(decoded[offset:offset+int(symbolLen)]), "\x00")
	if symbol != "" {
		meta.Symbol = &symbol
	}
}

// derivePDA derives a Program Derived Address using the Solana algorithm.
func derivePDA(seeds [][]byte, programID []byte) string {
	// PDA derivation algorithm:
	// 1. Concatenate all seeds with bump
	// 2. Append program ID and "ProgramDerivedAddress" marker
	// 3. SHA256 hash
	// 4. Find bump seed that results in off-curve point

	for bump := byte(255); bump > 0; bump-- {
		data := make([]byte, 0)
		for _, seed := range seeds {
			data = append(data, seed...)
		}
		data = append(data, bump)
		data = append(data, programID...)
		data = append(data, []byte("ProgramDerivedAddress")...)

		hash := sha256.Sum256(data)

		// Check if point is off the ed25519 curve
		if !isOnCurve(hash[:]) {
			return base58.Encode(hash[:])
		}
	}

	return ""
}

func isOnCurve(point []byte) bool {
	if len(point) != 32 {
		return false
	}
	_, err := new(edwards25519.Point).SetBytes(point)
	return err == nil
}

func needsRaydiumInference(logs []string, events []*discovery.SwapEvent) bool {
	isRaydium := false
	for _, log := range logs {
		if strings.Contains(log, discovery.RaydiumAMMV4) {
			isRaydium = true
			break
		}
	}
	if !isRaydium {
		return false
	}
	if events == nil {
		return true
	}
	for _, event := range events {
		if event.Mint == "" {
			return true
		}
	}
	return false
}
