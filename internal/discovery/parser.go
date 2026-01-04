package discovery

import (
	"regexp"
	"sort"
	"strconv"
)

// Parser extracts swap events from transaction logs.
type Parser interface {
	// ParseSwapEvents extracts swap events from transaction logs.
	// Returns events sorted by event_index for deterministic ordering.
	ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent
}

// ParserV2 extracts swap events from transaction logs with account keys.
// This enables extraction of mint/pool from account keys for DEXes like Raydium.
type ParserV2 interface {
	Parser
	// ParseSwapEventsV2 extracts swap events using logs and account keys.
	ParseSwapEventsV2(logs []string, accountKeys []string, txSig string, slot int64, timestamp int64) []*SwapEvent
}

// MinimalParser implements Parser with basic pattern matching.
// This is a minimal implementation for testing; real DEX parsers would be more complex.
type MinimalParser struct {
	// swapPattern matches log lines indicating a swap.
	// Format: "Program log: Swap mint=<MINT> pool=<POOL>" or "Program log: Swap mint=<MINT>"
	swapPattern *regexp.Regexp
}

// NewParser creates a new MinimalParser.
func NewParser() *MinimalParser {
	return &MinimalParser{
		// Matches: "Program log: Swap mint=<base58> pool=<base58>" or "Program log: Swap mint=<base58>"
		swapPattern: regexp.MustCompile(`Program log: Swap mint=([A-Za-z0-9]+)(?: pool=([A-Za-z0-9]+))?`),
	}
}

// ParseSwapEvents extracts swap events from logs.
// Returns events sorted by event_index (order of appearance in logs).
func (p *MinimalParser) ParseSwapEvents(logs []string, txSig string, slot int64, timestamp int64) []*SwapEvent {
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

		// Pool is optional (group 2)
		if len(matches) > 2 && matches[2] != "" {
			pool := matches[2]
			event.Pool = &pool
		}

		events = append(events, event)
	}

	// Sort by event_index for deterministic ordering
	sort.Slice(events, func(i, j int) bool {
		return events[i].EventIndex < events[j].EventIndex
	})

	return events
}

// SortSwapEvents sorts events by (slot, tx_signature, event_index) for deterministic ordering.
func SortSwapEvents(events []*SwapEvent) {
	sort.Slice(events, func(i, j int) bool {
		if events[i].Slot != events[j].Slot {
			return events[i].Slot < events[j].Slot
		}
		if events[i].TxSignature != events[j].TxSignature {
			return events[i].TxSignature < events[j].TxSignature
		}
		return events[i].EventIndex < events[j].EventIndex
	})
}

// ParseEventIndex parses event index from string (for testing).
func ParseEventIndex(s string) (int, error) {
	return strconv.Atoi(s)
}
