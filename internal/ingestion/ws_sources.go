package ingestion

import (
	"context"
	"log"
	"strings"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
)

// WSSwapEventSource provides real-time swap events via WebSocket subscription.
type WSSwapEventSource struct {
	ws       *solana.WSClientImpl
	rpc      *solana.HTTPClient // For fetching full transaction data
	parser   *discovery.DEXParser
	programs []string
}

// NewWSSwapEventSource creates a new WebSocket-based swap event source.
func NewWSSwapEventSource(ws *solana.WSClientImpl, rpc *solana.HTTPClient, programs []string) *WSSwapEventSource {
	return &WSSwapEventSource{
		ws:       ws,
		rpc:      rpc,
		parser:   discovery.NewDEXParser(),
		programs: programs,
	}
}

// Subscribe returns a channel of swap events from live WebSocket subscription.
// The channel is closed when the context is cancelled or an error occurs.
func (s *WSSwapEventSource) Subscribe(ctx context.Context) (<-chan *domain.SwapEvent, error) {
	// Subscribe to logs mentioning our DEX programs
	logsCh, err := s.ws.SubscribeLogs(ctx, solana.LogsFilter{
		Mentions: s.programs,
	})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan *domain.SwapEvent, 100)

	go func() {
		defer close(eventsCh)

		for {
			select {
			case <-ctx.Done():
				return
			case notif, ok := <-logsCh:
				if !ok {
					return
				}

				// Skip failed transactions
				if notif.Err != nil {
					continue
				}

				// Fetch full transaction for account keys and blockTime
				tx, err := s.rpc.GetTransaction(ctx, notif.Signature)
				if err != nil || tx == nil {
					// Fallback: use deterministic timestamp and V2 parser
					// NOTE: Raydium swaps require account keys and will be dropped here
					// Only pump.fun swaps (which use log parsing) will work in fallback
					isRaydium := containsRaydiumProgram(notif.Logs)
					if isRaydium {
						log.Printf("WARN: RPC fetch failed for Raydium tx %s, swap will be dropped: %v", notif.Signature, err)
					} else {
						log.Printf("WARN: RPC fetch failed for %s, using fallback: %v", notif.Signature, err)
					}
					timestamp, err := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, 0)
					if err != nil {
						timestamp = 0
					}
					// Only parse pump.fun swaps in fallback (they use log-based parsing)
					swapEvents := s.parser.ParseSwapEventsV2(
						notif.Logs,
						nil, // No account keys - Raydium swaps will be skipped
						notif.Signature,
						notif.Slot,
						timestamp,
					)
					s.sendSwapEvents(ctx, eventsCh, swapEvents)
					continue
				}

				timestamp, err := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, tx.BlockTime)
				if err != nil {
					timestamp = 0
				}

				// Get account keys from transaction message
				var accountKeys []string
				if tx.Message != nil {
					accountKeys = tx.Message.AccountKeys
				}

				// Use V2 parser to extract mint/pool from account keys
				swapEvents := s.parser.ParseSwapEventsV2(
					notif.Logs,
					accountKeys,
					notif.Signature,
					notif.Slot,
					timestamp,
				)
				inferredPool, inferredMint := "", ""
				if needsRaydiumInference(notif.Logs, swapEvents) {
					pool, mint, err := inferRaydiumPoolAndMint(ctx, s.rpc, accountKeys)
					if err == nil {
						inferredPool = pool
						inferredMint = mint
					}
				}
				for _, se := range swapEvents {
					if se.Mint == "" && inferredMint != "" {
						se.Mint = inferredMint
					}
					if se.Pool == nil && inferredPool != "" {
						pool := inferredPool
						se.Pool = &pool
					}
				}

				s.sendSwapEvents(ctx, eventsCh, swapEvents)
			}
		}
	}()

	return eventsCh, nil
}

// sendSwapEvents sends parsed swap events to the channel.
func (s *WSSwapEventSource) sendSwapEvents(ctx context.Context, eventsCh chan<- *domain.SwapEvent, swapEvents []*discovery.SwapEvent) {
	for _, se := range swapEvents {
		if se.Mint == "" {
			continue
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

		select {
		case eventsCh <- event:
		case <-ctx.Done():
			return
		}
	}
}

// containsRaydiumProgram checks if logs mention the Raydium AMM program.
func containsRaydiumProgram(logs []string) bool {
	for _, log := range logs {
		if strings.Contains(log, discovery.RaydiumAMMV4) {
			return true
		}
	}
	return false
}

// WSLiquidityEventSource provides real-time liquidity events via WebSocket.
type WSLiquidityEventSource struct {
	ws             *solana.WSClientImpl
	rpc            *solana.HTTPClient
	parser         *discovery.DEXParser
	programs       []string
	candidateStore storage.CandidateStore // For looking up CandidateID by mint
}

// NewWSLiquidityEventSource creates a new WebSocket-based liquidity event source.
func NewWSLiquidityEventSource(ws *solana.WSClientImpl, rpc *solana.HTTPClient, programs []string) *WSLiquidityEventSource {
	return &WSLiquidityEventSource{
		ws:       ws,
		rpc:      rpc,
		parser:   discovery.NewDEXParser(),
		programs: programs,
	}
}

// NewWSLiquidityEventSourceWithStore creates a liquidity event source with candidate store for ID lookup.
func NewWSLiquidityEventSourceWithStore(ws *solana.WSClientImpl, rpc *solana.HTTPClient, programs []string, candidateStore storage.CandidateStore) *WSLiquidityEventSource {
	return &WSLiquidityEventSource{
		ws:             ws,
		rpc:            rpc,
		parser:         discovery.NewDEXParser(),
		programs:       programs,
		candidateStore: candidateStore,
	}
}

// Subscribe returns a channel of liquidity events from live WebSocket subscription.
func (s *WSLiquidityEventSource) Subscribe(ctx context.Context) (<-chan *domain.LiquidityEvent, error) {
	logsCh, err := s.ws.SubscribeLogs(ctx, solana.LogsFilter{
		Mentions: s.programs,
	})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan *domain.LiquidityEvent, 100)

	go func() {
		defer close(eventsCh)

		for {
			select {
			case <-ctx.Done():
				return
			case notif, ok := <-logsCh:
				if !ok {
					return
				}

				if notif.Err != nil {
					continue
				}

				// Fetch full transaction for account keys and blockTime
				tx, err := s.rpc.GetTransaction(ctx, notif.Signature)
				if err != nil || tx == nil {
					// Fallback: use deterministic timestamp and V2 parser
					log.Printf("WARN: RPC fetch failed for %s, using fallback: %v", notif.Signature, err)
					timestamp, err := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, 0)
					if err != nil {
						timestamp = 0
					}
					liqEvents := s.parser.ParseLiquidityEventsV2(
						notif.Logs,
						nil, // No account keys - some events may lack pool/mint
						notif.Signature,
						notif.Slot,
						timestamp,
					)
					s.sendLiquidityEvents(ctx, eventsCh, liqEvents)
					continue
				}

				timestamp, err := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, tx.BlockTime)
				if err != nil {
					timestamp = 0
				}

				// Get account keys from transaction message
				var accountKeys []string
				if tx.Message != nil {
					accountKeys = tx.Message.AccountKeys
				}

				// Use V2 parser to extract pool/mint from account keys
				liqEvents := s.parser.ParseLiquidityEventsV2(
					notif.Logs,
					accountKeys,
					notif.Signature,
					notif.Slot,
					timestamp,
				)
				inferredPool, inferredMint := "", ""
				if needsRaydiumInference(notif.Logs, nil) {
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
				}

				s.sendLiquidityEvents(ctx, eventsCh, liqEvents)
			}
		}
	}()

	return eventsCh, nil
}

// sendLiquidityEvents sends parsed liquidity events to the channel.
func (s *WSLiquidityEventSource) sendLiquidityEvents(ctx context.Context, eventsCh chan<- *domain.LiquidityEvent, liqEvents []*discovery.LiquidityEvent) {
	for _, le := range liqEvents {
		// Look up CandidateID by mint if store is available
		var candidateID string
		if s.candidateStore != nil && le.Mint != "" {
			id, err := resolveCandidateIDByMint(ctx, s.candidateStore, le.Mint)
			if err == nil {
				candidateID = id
			}
		}

		event := &domain.LiquidityEvent{
			CandidateID: candidateID,
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

		select {
		case eventsCh <- event:
		case <-ctx.Done():
			return
		}
	}
}
