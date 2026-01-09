package ingestion

import (
	"context"
	"log"
	"strings"
	"time"

	"solana-token-lab/internal/discovery"
	"solana-token-lab/internal/domain"
	"solana-token-lab/internal/solana"
	"solana-token-lab/internal/storage"
)

const (
	maxRetries     = 3
	baseRetryDelay = 500 * time.Millisecond
)

// retryGetTransaction fetches a transaction with exponential backoff retry.
func retryGetTransaction(ctx context.Context, rpc *solana.HTTPClient, signature string) (*solana.Transaction, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		tx, err := rpc.GetTransaction(ctx, signature)
		if err == nil {
			return tx, nil
		}
		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Exponential backoff: 500ms, 1s, 2s
		delay := baseRetryDelay * time.Duration(1<<attempt)
		log.Printf("[ws] Retry %d/%d for GetTransaction %s after %v: %v", attempt+1, maxRetries, signature, delay, err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

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
	// Subscribe to logs for each program separately (some providers only support 1 address per subscription)
	var logsChannels []<-chan solana.LogNotification
	for _, program := range s.programs {
		logsCh, err := s.ws.SubscribeLogs(ctx, solana.LogsFilter{
			Mentions: []string{program},
		})
		if err != nil {
			return nil, err
		}
		logsChannels = append(logsChannels, logsCh)
		log.Printf("[ws-swap] Subscribed to program: %s", program)
	}

	eventsCh := make(chan *domain.SwapEvent, 100)

	// Merge all log channels and process
	go func() {
		defer close(eventsCh)

		// Merge channels
		merged := make(chan solana.LogNotification, 1000)
		for _, ch := range logsChannels {
			go func(logsCh <-chan solana.LogNotification) {
				for notif := range logsCh {
					select {
					case merged <- notif:
					case <-ctx.Done():
						return
					}
				}
			}(ch)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case notif, ok := <-merged:
				if !ok {
					log.Println("[ws-swap] merged channel closed")
					return
				}
				log.Printf("[ws-swap] Received notif: sig=%s err=%v", notif.Signature, notif.Err)
				s.processSwapNotification(ctx, eventsCh, notif)
			}
		}
	}()

	return eventsCh, nil
}

// processSwapNotification processes a single log notification for swaps.
func (s *WSSwapEventSource) processSwapNotification(ctx context.Context, eventsCh chan<- *domain.SwapEvent, notif solana.LogNotification) {
	// Skip failed transactions
	if notif.Err != nil {
		return
	}

	log.Printf("[ws-swap] Processing tx: %s (slot=%d, logs=%d)", notif.Signature, notif.Slot, len(notif.Logs))

	// Fetch full transaction for account keys and blockTime with retry
	tx, err := retryGetTransaction(ctx, s.rpc, notif.Signature)
	if err != nil || tx == nil {
		// Fallback: use deterministic timestamp and V2 parser
		isRaydium := containsRaydiumProgram(notif.Logs)
		if isRaydium {
			log.Printf("WARN: RPC fetch failed for Raydium tx %s after %d retries, swap will be dropped: %v", notif.Signature, maxRetries, err)
		}
		timestamp, _ := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, 0)
		swapEvents := s.parser.ParseSwapEventsV2(
			notif.Logs,
			nil, // No account keys - Raydium swaps will be skipped
			notif.Signature,
			notif.Slot,
			timestamp,
		)
		s.sendSwapEvents(ctx, eventsCh, swapEvents)
		return
	}

	timestamp, _ := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, tx.BlockTime)

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

	// Infer Raydium pool/mint if needed
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

	if len(swapEvents) > 0 {
		log.Printf("[ws-swap] Parsed %d swaps from tx %s", len(swapEvents), notif.Signature)
	}
	s.sendSwapEvents(ctx, eventsCh, swapEvents)
}

// sendSwapEvents sends parsed swap events to the channel.
func (s *WSSwapEventSource) sendSwapEvents(ctx context.Context, eventsCh chan<- *domain.SwapEvent, swapEvents []*discovery.SwapEvent) {
	for _, se := range swapEvents {
		if se.Mint == "" {
			log.Printf("[ws-swap] SKIP: empty mint for tx %s (event_index=%d)", se.TxSignature, se.EventIndex)
			continue
		}
		log.Printf("[ws-swap] SEND: mint=%s tx=%s", se.Mint, se.TxSignature)

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
	// Subscribe to logs for each program separately (some providers only support 1 address per subscription)
	var logsChannels []<-chan solana.LogNotification
	for _, program := range s.programs {
		logsCh, err := s.ws.SubscribeLogs(ctx, solana.LogsFilter{
			Mentions: []string{program},
		})
		if err != nil {
			return nil, err
		}
		logsChannels = append(logsChannels, logsCh)
		log.Printf("[ws-liquidity] Subscribed to program: %s", program)
	}

	eventsCh := make(chan *domain.LiquidityEvent, 100)

	// Merge all log channels and process
	go func() {
		defer close(eventsCh)

		// Merge channels
		merged := make(chan solana.LogNotification, 1000)
		for _, ch := range logsChannels {
			go func(logsCh <-chan solana.LogNotification) {
				for notif := range logsCh {
					select {
					case merged <- notif:
					case <-ctx.Done():
						return
					}
				}
			}(ch)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case notif, ok := <-merged:
				if !ok {
					return
				}
				s.processLiquidityNotification(ctx, eventsCh, notif)
			}
		}
	}()

	return eventsCh, nil
}

// processLiquidityNotification processes a single log notification for liquidity events.
func (s *WSLiquidityEventSource) processLiquidityNotification(ctx context.Context, eventsCh chan<- *domain.LiquidityEvent, notif solana.LogNotification) {
	if notif.Err != nil {
		return
	}

	// Fetch full transaction for account keys and blockTime with retry
	tx, err := retryGetTransaction(ctx, s.rpc, notif.Signature)
	if err != nil || tx == nil {
		log.Printf("WARN: RPC fetch failed for liquidity tx %s after %d retries: %v", notif.Signature, maxRetries, err)
		timestamp, _ := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, 0)
		liqEvents := s.parser.ParseLiquidityEventsV2(
			notif.Logs,
			nil,
			notif.Signature,
			notif.Slot,
			timestamp,
		)
		s.sendLiquidityEvents(ctx, eventsCh, liqEvents)
		return
	}

	timestamp, _ := resolveBlockTimestamp(ctx, s.rpc, notif.Slot, tx.BlockTime)

	var accountKeys []string
	if tx.Message != nil {
		accountKeys = tx.Message.AccountKeys
	}

	liqEvents := s.parser.ParseLiquidityEventsV2(
		notif.Logs,
		accountKeys,
		notif.Signature,
		notif.Slot,
		timestamp,
	)

	// Infer Raydium pool/mint if needed
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
