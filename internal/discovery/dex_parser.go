package discovery

import (
	"encoding/base64"
	"encoding/binary"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mr-tron/base58"
)

// Known DEX program IDs.
const (
	// RaydiumAMMV4 is the Raydium AMM v4 program ID.
	RaydiumAMMV4 = "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8"
	// PumpFun is the pump.fun program ID.
	PumpFun = "6EF8rrecthR5Dkzon8Nwu78hRvfCKubJ14M5uBEwF6P"
)

// DEXParser parses swap events from multiple DEX programs.
type DEXParser struct {
	parsers map[string]Parser // programID -> parser
}

// NewDEXParser creates a new multi-DEX parser with default parsers registered.
func NewDEXParser() *DEXParser {
	p := &DEXParser{
		parsers: make(map[string]Parser),
	}

	// Register default parsers
	p.RegisterParser(RaydiumAMMV4, NewRaydiumParser())
	p.RegisterParser(PumpFun, NewPumpFunParser())

	return p
}

// RegisterParser registers a parser for a specific program ID.
func (p *DEXParser) RegisterParser(programID string, parser Parser) {
	p.parsers[programID] = parser
}

// ParseSwapEvents parses swap events from transaction logs.
// It tries all registered parsers and merges results.
// Note: For Raydium, this returns empty - use ParseSwapEventsV2 with account keys.
func (p *DEXParser) ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	var allEvents []*SwapEvent

	for _, parser := range p.parsers {
		events := parser.ParseSwapEvents(logs, txSig, slot, timestamp)
		allEvents = append(allEvents, events...)
	}

	// Sort by event_index for deterministic ordering
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].EventIndex < allEvents[j].EventIndex
	})

	return allEvents
}

// ParseSwapEventsV2 parses swap events using logs and account keys.
// This enables proper extraction of mint/pool for DEXes like Raydium.
func (p *DEXParser) ParseSwapEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	var allEvents []*SwapEvent

	for _, parser := range p.parsers {
		// Try V2 parser first
		if v2Parser, ok := parser.(ParserV2); ok {
			events := v2Parser.ParseSwapEventsV2(logs, accountKeys, txSig, slot, timestamp)
			allEvents = append(allEvents, events...)
		} else {
			// Fall back to V1
			events := parser.ParseSwapEvents(logs, txSig, slot, timestamp)
			allEvents = append(allEvents, events...)
		}
	}

	// Sort by event_index for deterministic ordering
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].EventIndex < allEvents[j].EventIndex
	})

	return allEvents
}

// ParseLiquidityEvents parses liquidity events from transaction logs.
func (p *DEXParser) ParseLiquidityEvents(logs []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent {
	var allEvents []*LiquidityEvent

	for _, parser := range p.parsers {
		if liqParser, ok := parser.(LiquidityParser); ok {
			events := liqParser.ParseLiquidityEvents(logs, txSig, slot, timestamp)
			allEvents = append(allEvents, events...)
		}
	}

	// Sort by event_index for deterministic ordering
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].EventIndex < allEvents[j].EventIndex
	})

	return allEvents
}

// LiquidityParser is an optional interface for parsers that can extract liquidity events.
type LiquidityParser interface {
	ParseLiquidityEvents(logs []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent
}

// LiquidityParserV2 extracts liquidity events with account keys.
type LiquidityParserV2 interface {
	LiquidityParser
	ParseLiquidityEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent
}

// ParseLiquidityEventsV2 parses liquidity events using logs and account keys.
func (p *DEXParser) ParseLiquidityEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent {
	var allEvents []*LiquidityEvent

	for _, parser := range p.parsers {
		// Try V2 parser first
		if v2Parser, ok := parser.(LiquidityParserV2); ok {
			events := v2Parser.ParseLiquidityEventsV2(logs, accountKeys, txSig, slot, timestamp)
			allEvents = append(allEvents, events...)
		} else if liqParser, ok := parser.(LiquidityParser); ok {
			// Fall back to V1
			events := liqParser.ParseLiquidityEvents(logs, txSig, slot, timestamp)
			allEvents = append(allEvents, events...)
		}
	}

	// Sort by event_index for deterministic ordering
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].EventIndex < allEvents[j].EventIndex
	})

	return allEvents
}

// LiquidityEvent represents a liquidity add/remove event.
type LiquidityEvent struct {
	Pool         string
	Mint         string
	EventType    string // "add" or "remove"
	AmountToken  uint64
	AmountQuote  uint64
	TxSignature  string
	EventIndex   int
	Slot         int64
	Timestamp    int64
}

// RaydiumParser parses Raydium AMM v4 swap events.
type RaydiumParser struct {
	// ray_log pattern: base64 encoded data after "ray_log: "
	rayLogPattern *regexp.Regexp
	// Program invocation pattern to detect Raydium calls
	invokePattern *regexp.Regexp
}

// NewRaydiumParser creates a new Raydium parser.
func NewRaydiumParser() *RaydiumParser {
	return &RaydiumParser{
		rayLogPattern: regexp.MustCompile(`ray_log: ([A-Za-z0-9+/=]+)`),
		invokePattern: regexp.MustCompile(`Program ` + RaydiumAMMV4 + ` invoke`),
	}
}

// Raydium AMM v4 account indices for swap instructions.
// The account layout for swapBaseIn/swapBaseOut:
// 0: Token program
// 1: AMM ID (pool)
// 2: AMM authority
// 3: AMM open orders
// 4: AMM target orders  (or pool coin token account for some versions)
// 5: Pool coin token account
// 6: Pool PC token account
// 7: Serum program
// 8: Serum market
// 9: Serum bids
// 10: Serum asks
// 11: Serum event queue
// 12: Serum coin vault
// 13: Serum PC vault
// 14: Serum vault signer
// 15: User source token account
// 16: User destination token account
// 17: User owner
//
// For token mint extraction, we look at pool coin token account's mint.
// Since we can't derive mint from account address directly, we use heuristics:
// - In Raydium swaps involving SOL pairs, one token is WSOL (So11111111111111111111111111111111111111112)
// - The other token is what we're interested in
const (
	raydiumPoolIndex = 1 // AMM ID (pool address)
	// Token mints are typically at indices after the standard accounts
	// We'll extract from ray_log where possible, or use account heuristics
)

// WSOL is the Wrapped SOL mint address.
const WSOL = "So11111111111111111111111111111111111111112"

// ParseSwapEvents parses Raydium swap events from logs.
// This basic version cannot extract mint - use ParseSwapEventsV2 with account keys.
func (p *RaydiumParser) ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	// Without account keys, we can only detect swap occurred but not extract mint
	// Return empty - caller should use ParseSwapEventsV2
	return nil
}

// ParseSwapEventsV2 parses Raydium swap events using logs and account keys.
// This enables proper extraction of mint and pool addresses.
func (p *RaydiumParser) ParseSwapEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	var events []*SwapEvent
	var eventIdx int

	// Find ray_log entries indicating swaps
	for i, log := range logs {
		matches := p.rayLogPattern.FindStringSubmatch(log)
		if matches == nil {
			continue
		}

		// Decode base64 ray_log
		data, err := base64.StdEncoding.DecodeString(matches[1])
		if err != nil {
			continue
		}

		// Check if this is a swap log
		if !p.isSwapLog(data) {
			continue
		}

		// Extract pool and mint from account keys
		pool, mint := p.extractPoolAndMint(accountKeys, data)
		if mint == "" {
			continue // Skip if we couldn't determine the mint
		}

		// Extract amount_out from ray_log (raw value, no decimals normalization)
		// ray_log for swap: discriminator(1) + ammId(32) + inputMint(32) + outputMint(32) + amountIn(8) + amountOut(8)
		var amountOut float64
		if len(data) >= 113 { // 1 + 32 + 32 + 32 + 8 + 8
			amountOutRaw := readUint64LE(data, 105) // offset: 1 + 32 + 32 + 32 + 8 = 105
			// Store raw value - normalization happens later using token decimals from metadata
			amountOut = float64(amountOutRaw)
		}

		event := &SwapEvent{
			Mint:        mint,
			TxSignature: txSig,
			EventIndex:  eventIdx,
			Slot:        slot,
			Timestamp:   timestamp,
			AmountOut:   amountOut,
		}
		if pool != "" {
			event.Pool = &pool
		}

		events = append(events, event)
		eventIdx++

		_ = i // used for potential debugging
	}

	return events
}

// isSwapLog checks if ray_log data represents a swap instruction.
func (p *RaydiumParser) isSwapLog(data []byte) bool {
	if len(data) < 1 {
		return false
	}
	// Raydium discriminators: 0x09 = SwapBaseIn, 0x0b = SwapBaseOut (newer versions)
	// Also 0x0d, 0x0e for some instruction variants
	disc := data[0]
	return disc == 0x09 || disc == 0x0b || disc == 0x0d || disc == 0x0e
}

// extractPoolAndMint extracts pool address and non-WSOL mint from account keys.
func (p *RaydiumParser) extractPoolAndMint(accountKeys []string, rayLogData []byte) (pool, mint string) {
	if len(accountKeys) < 18 {
		return "", ""
	}

	// Pool is at index 1
	pool = accountKeys[raydiumPoolIndex]

	// Try to extract mints from ray_log data if available
	// ray_log for swap contains: discriminator(1) + ammId(32) + inputMint(32) + outputMint(32) + ...
	if len(rayLogData) >= 97 { // 1 + 32 + 32 + 32
		// inputMint at offset 33 (1 + 32)
		// outputMint at offset 65 (1 + 32 + 32)
		inputMint := base58Encode(rayLogData[33:65])
		outputMint := base58Encode(rayLogData[65:97])

		// Return the non-WSOL mint
		if inputMint != WSOL && inputMint != "" {
			return pool, inputMint
		}
		if outputMint != WSOL && outputMint != "" {
			return pool, outputMint
		}
	}

	// Fallback: scan account keys for potential mints
	// Skip known program IDs and look for token mints
	for _, key := range accountKeys {
		if key == WSOL || key == RaydiumAMMV4 {
			continue
		}
		// Simple heuristic: first non-system account that looks like a mint
		if len(key) >= 32 && len(key) <= 44 {
			// This is a rough heuristic - in production we'd verify it's actually a mint
			mint = key
			break
		}
	}

	return pool, mint
}

// ParseLiquidityEvents parses Raydium liquidity events from logs.
func (p *RaydiumParser) ParseLiquidityEvents(logs []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent {
	return p.ParseLiquidityEventsV2(logs, nil, txSig, slot, timestamp)
}

// ParseLiquidityEventsV2 parses Raydium liquidity events with account keys.
func (p *RaydiumParser) ParseLiquidityEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent {
	var events []*LiquidityEvent
	var eventIdx int

	for i, log := range logs {
		matches := p.rayLogPattern.FindStringSubmatch(log)
		if matches == nil {
			continue
		}

		data, err := base64.StdEncoding.DecodeString(matches[1])
		if err != nil {
			continue
		}

		// Check if this is a liquidity log (add/remove)
		eventType := p.getLiquidityEventType(data)
		if eventType == "" {
			continue
		}

		// Extract pool and mint
		var pool, mint string
		if len(accountKeys) > raydiumPoolIndex {
			pool = accountKeys[raydiumPoolIndex]
		}
		if len(accountKeys) > 5 {
			// Try to find non-WSOL mint in accounts
			for _, key := range accountKeys {
				if key != WSOL && key != RaydiumAMMV4 && len(key) >= 32 && len(key) <= 44 {
					mint = key
					break
				}
			}
		}

		// Extract amounts from ray_log if available
		var amountToken, amountQuote uint64
		if len(data) >= 17 { // 1 + 8 + 8 minimum
			amountToken = readUint64LE(data, 1)
			amountQuote = readUint64LE(data, 9)
		}

		event := &LiquidityEvent{
			Pool:        pool,
			Mint:        mint,
			EventType:   eventType,
			AmountToken: amountToken,
			AmountQuote: amountQuote,
			TxSignature: txSig,
			EventIndex:  eventIdx,
			Slot:        slot,
			Timestamp:   timestamp,
		}

		events = append(events, event)
		eventIdx++
		_ = i
	}

	return events
}

// getLiquidityEventType determines if ray_log represents a liquidity event.
func (p *RaydiumParser) getLiquidityEventType(data []byte) string {
	if len(data) < 1 {
		return ""
	}
	// Raydium discriminators for liquidity:
	// 0x03 = Deposit (add liquidity)
	// 0x04 = Withdraw (remove liquidity)
	switch data[0] {
	case 0x03:
		return "add"
	case 0x04:
		return "remove"
	default:
		return ""
	}
}

// PumpFunParser parses pump.fun swap events.
type PumpFunParser struct {
	// Buy/Sell instruction patterns
	buyPattern  *regexp.Regexp
	sellPattern *regexp.Regexp
	// Token mint pattern
	mintPattern *regexp.Regexp
}

// NewPumpFunParser creates a new pump.fun parser.
func NewPumpFunParser() *PumpFunParser {
	return &PumpFunParser{
		buyPattern:  regexp.MustCompile(`Program log: Instruction: Buy`),
		sellPattern: regexp.MustCompile(`Program log: Instruction: Sell`),
		mintPattern: regexp.MustCompile(`mint=([A-Za-z0-9]+)`),
	}
}

// ParseSwapEvents parses pump.fun swap events from logs.
func (p *PumpFunParser) ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	var events []*SwapEvent
	var currentMint string
	var pendingAmount float64
	inPumpFun := false

	// Pattern to extract amount from pump.fun logs
	amountPattern := regexp.MustCompile(`(?:amount|token_amount)[=:]?\s*(\d+)`)
	solAmountPattern := regexp.MustCompile(`(?:sol_amount)[=:]?\s*(\d+)`)

	for i, log := range logs {
		// Detect pump.fun program invocation
		if strings.Contains(log, "Program "+PumpFun+" invoke") {
			inPumpFun = true
			pendingAmount = 0
			continue
		}

		// Detect program exit
		if strings.Contains(log, "Program "+PumpFun+" success") ||
			strings.Contains(log, "Program "+PumpFun+" failed") {
			inPumpFun = false
			currentMint = ""
			pendingAmount = 0
			continue
		}

		if !inPumpFun {
			continue
		}

		// Look for mint in logs
		if mintMatch := p.mintPattern.FindStringSubmatch(log); mintMatch != nil {
			currentMint = mintMatch[1]
		}

		// Try to extract token amount from logs (raw value, no decimals normalization)
		if amountMatch := amountPattern.FindStringSubmatch(log); amountMatch != nil {
			if parsed, err := strconv.ParseUint(amountMatch[1], 10, 64); err == nil {
				// Store raw value - normalization happens later using token decimals from metadata
				pendingAmount = float64(parsed)
			}
		}

		// For buy, SOL amount is input, token is output
		// For sell, token is input, SOL is output
		if solMatch := solAmountPattern.FindStringSubmatch(log); solMatch != nil {
			// We have SOL amount, but we want token amount for volume
			// This is already captured above
		}

		// Check for buy/sell instructions
		isBuy := p.buyPattern.MatchString(log)
		isSell := p.sellPattern.MatchString(log)

		if isBuy || isSell {
			event := &SwapEvent{
				Mint:        currentMint,
				TxSignature: txSig,
				EventIndex:  i,
				Slot:        slot,
				Timestamp:   timestamp,
				AmountOut:   pendingAmount,
			}

			events = append(events, event)
		}
	}

	return events
}

// ParseLiquidityEvents parses pump.fun liquidity events from logs.
// pump.fun uses a bonding curve model - "Create" initializes liquidity,
// and liquidity is migrated when the token "graduates" to Raydium.
func (p *PumpFunParser) ParseLiquidityEvents(logs []string, txSig string, slot int64, timestamp int64) []*LiquidityEvent {
	var events []*LiquidityEvent
	var currentMint string
	inPumpFun := false
	eventIdx := 0

	// Patterns for liquidity events
	createPattern := regexp.MustCompile(`Program log: Instruction: Create`)
	migratePattern := regexp.MustCompile(`Program log: Instruction: Migrate`)

	for i, log := range logs {
		// Detect pump.fun program invocation
		if strings.Contains(log, "Program "+PumpFun+" invoke") {
			inPumpFun = true
			continue
		}

		// Detect program exit
		if strings.Contains(log, "Program "+PumpFun+" success") ||
			strings.Contains(log, "Program "+PumpFun+" failed") {
			inPumpFun = false
			currentMint = ""
			continue
		}

		if !inPumpFun {
			continue
		}

		// Look for mint in logs
		if mintMatch := p.mintPattern.FindStringSubmatch(log); mintMatch != nil {
			currentMint = mintMatch[1]
		}

		// Check for Create (initial liquidity add)
		if createPattern.MatchString(log) {
			event := &LiquidityEvent{
				Mint:        currentMint,
				EventType:   "add",
				TxSignature: txSig,
				EventIndex:  eventIdx,
				Slot:        slot,
				Timestamp:   timestamp,
			}
			events = append(events, event)
			eventIdx++
		}

		// Check for Migrate (liquidity removal from pump.fun)
		if migratePattern.MatchString(log) {
			event := &LiquidityEvent{
				Mint:        currentMint,
				EventType:   "remove",
				TxSignature: txSig,
				EventIndex:  eventIdx,
				Slot:        slot,
				Timestamp:   timestamp,
			}
			events = append(events, event)
			eventIdx++
		}

		_ = i
	}

	return events
}

// GenericSwapParser is a fallback parser that uses simple pattern matching.
// It's used when no specific DEX parser matches.
type GenericSwapParser struct {
	swapPattern *regexp.Regexp
}

// NewGenericSwapParser creates a generic swap parser.
func NewGenericSwapParser() *GenericSwapParser {
	return &GenericSwapParser{
		// Matches various swap log patterns
		swapPattern: regexp.MustCompile(`(?i)(?:swap|trade|exchange).*mint[=:]?\s*([A-Za-z0-9]{32,44})`),
	}
}

// ParseSwapEvents parses swap events using generic patterns.
func (p *GenericSwapParser) ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
	var events []*SwapEvent

	for i, log := range logs {
		matches := p.swapPattern.FindStringSubmatch(log)
		if matches == nil {
			continue
		}

		event := &SwapEvent{
			Mint:        matches[1],
			TxSignature: txSig,
			EventIndex:  i,
			Slot:        slot,
			Timestamp:   timestamp,
		}
		events = append(events, event)
	}

	return events
}

// Helper functions for parsing binary data

// readUint64LE reads a little-endian uint64 from data at offset.
func readUint64LE(data []byte, offset int) uint64 {
	if offset+8 > len(data) {
		return 0
	}
	return binary.LittleEndian.Uint64(data[offset:])
}

// base58Encode encodes bytes to base58 string (Bitcoin alphabet).
func base58Encode(data []byte) string {
	return base58.Encode(data)
}
