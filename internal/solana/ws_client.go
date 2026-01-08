package solana

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WSClientConfig configures WebSocket client behavior.
type WSClientConfig struct {
	// ReconnectDelay is initial delay before reconnect attempt.
	ReconnectDelay time.Duration
	// MaxReconnectDelay is maximum delay between reconnect attempts.
	MaxReconnectDelay time.Duration
	// PingInterval is interval for sending ping frames.
	PingInterval time.Duration
	// ReadTimeout is timeout for reading messages.
	ReadTimeout time.Duration
	// WriteTimeout is timeout for writing messages.
	WriteTimeout time.Duration
}

// DefaultWSConfig returns default WebSocket configuration.
func DefaultWSConfig() WSClientConfig {
	return WSClientConfig{
		ReconnectDelay:    1 * time.Second,
		MaxReconnectDelay: 30 * time.Second,
		PingInterval:      30 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
}

// WSClientImpl implements WSClient using gorilla/websocket.
type WSClientImpl struct {
	endpoint string
	config   WSClientConfig

	conn      *websocket.Conn
	connMu    sync.Mutex
	closed    atomic.Bool
	requestID atomic.Uint64

	// subscriptions maps subscription ID to channel
	subs   map[int64]chan LogNotification
	subsMu sync.RWMutex

	// activeFilters stores filters for resubscription after reconnect
	activeFilters   map[int64]LogsFilter
	activeFiltersMu sync.RWMutex

	// pendingSubs maps request ID to channel waiting for subscription ID
	pendingSubs   map[uint64]chan int64
	pendingSubsMu sync.Mutex

	// done signals shutdown
	done chan struct{}
	wg   sync.WaitGroup

	// reconnecting indicates reconnection in progress
	reconnecting atomic.Bool
}

// NewWSClient creates a new WebSocket client and connects to the endpoint.
func NewWSClient(ctx context.Context, endpoint string, config *WSClientConfig) (*WSClientImpl, error) {
	cfg := DefaultWSConfig()
	if config != nil {
		cfg = *config
	}

	c := &WSClientImpl{
		endpoint:      endpoint,
		config:        cfg,
		subs:          make(map[int64]chan LogNotification),
		activeFilters: make(map[int64]LogsFilter),
		pendingSubs:   make(map[uint64]chan int64),
		done:          make(chan struct{}),
	}

	if err := c.connect(ctx); err != nil {
		return nil, err
	}

	// Start reader goroutine
	c.wg.Add(1)
	go c.readLoop()

	// Start ping goroutine
	c.wg.Add(1)
	go c.pingLoop()

	return c, nil
}

// connect establishes WebSocket connection.
func (c *WSClientImpl) connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.endpoint, nil)
	if err != nil {
		return fmt.Errorf("websocket dial: %w", err)
	}

	c.conn = conn
	return nil
}

// SubscribeLogs subscribes to program logs matching the filter.
func (c *WSClientImpl) SubscribeLogs(ctx context.Context, filter LogsFilter) (<-chan LogNotification, error) {
	if c.closed.Load() {
		return nil, fmt.Errorf("client closed")
	}

	reqID := c.requestID.Add(1)

	// Build subscription request
	mentionsFilter := make(map[string]interface{})
	if len(filter.Mentions) > 0 {
		mentionsFilter["mentions"] = filter.Mentions
	} else {
		mentionsFilter["all"] = nil
	}

	req := wsRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "logsSubscribe",
		Params: []interface{}{
			mentionsFilter,
			map[string]string{"commitment": "confirmed"},
		},
	}

	// Create channel to receive subscription ID
	confirmCh := make(chan int64, 1)
	c.pendingSubsMu.Lock()
	c.pendingSubs[reqID] = confirmCh
	c.pendingSubsMu.Unlock()

	// Send subscribe request
	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return nil, fmt.Errorf("not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	err := c.conn.WriteJSON(req)
	c.connMu.Unlock()

	if err != nil {
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return nil, fmt.Errorf("write subscribe: %w", err)
	}

	// Wait for subscription confirmation (30s timeout for slow providers)
	var subID int64
	select {
	case subID = <-confirmCh:
	case <-time.After(30 * time.Second):
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return nil, fmt.Errorf("subscription timeout after 30s")
	case <-c.done:
		return nil, fmt.Errorf("client closed")
	case <-ctx.Done():
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return nil, ctx.Err()
	}

	// Create notification channel with large buffer for backpressure
	// Blocking send ensures no event loss; buffer absorbs burst
	ch := make(chan LogNotification, 10000)
	c.subsMu.Lock()
	c.subs[subID] = ch
	c.subsMu.Unlock()

	// Store filter for resubscription after reconnect
	c.activeFiltersMu.Lock()
	c.activeFilters[subID] = filter
	c.activeFiltersMu.Unlock()

	return ch, nil
}

// Close closes the WebSocket connection.
func (c *WSClientImpl) Close() error {
	if c.closed.Swap(true) {
		return nil // Already closed
	}

	close(c.done)

	c.connMu.Lock()
	if c.conn != nil {
		c.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		c.conn.Close()
	}
	c.connMu.Unlock()

	// Close all subscription channels
	c.subsMu.Lock()
	for id, ch := range c.subs {
		close(ch)
		delete(c.subs, id)
	}
	c.subsMu.Unlock()

	// Close pending subscription channels
	c.pendingSubsMu.Lock()
	for id, ch := range c.pendingSubs {
		close(ch)
		delete(c.pendingSubs, id)
	}
	c.pendingSubsMu.Unlock()

	c.wg.Wait()
	return nil
}

// readLoop reads messages from WebSocket and dispatches to subscribers.
func (c *WSClientImpl) readLoop() {
	defer c.wg.Done()

	reconnectDelay := c.config.ReconnectDelay

	for !c.closed.Load() {
		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		if conn == nil {
			select {
			case <-c.done:
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))

		_, message, err := conn.ReadMessage()
		if err != nil {
			if c.closed.Load() {
				return
			}

			// Connection error - attempt reconnect with exponential backoff
			if !c.reconnecting.Swap(true) {
				go c.reconnect(reconnectDelay)
			}

			// Increase delay for next reconnect (exponential backoff)
			reconnectDelay = reconnectDelay * 2
			if reconnectDelay > c.config.MaxReconnectDelay {
				reconnectDelay = c.config.MaxReconnectDelay
			}

			select {
			case <-c.done:
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		// Reset delay on successful read
		reconnectDelay = c.config.ReconnectDelay

		c.handleMessage(message)
	}
}

// reconnect attempts to reconnect and resubscribe.
func (c *WSClientImpl) reconnect(delay time.Duration) {
	defer c.reconnecting.Store(false)

	if c.closed.Load() {
		return
	}

	// Wait before reconnecting
	select {
	case <-c.done:
		return
	case <-time.After(delay):
	}

	// Close existing connection
	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	// Attempt reconnect
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.connect(ctx); err != nil {
		// Reconnect failed, will retry on next read error
		return
	}

	// Resubscribe to all active subscriptions
	c.resubscribeAll()
}

// resubscribeAll resubscribes to all active filters after reconnect.
func (c *WSClientImpl) resubscribeAll() {
	c.activeFiltersMu.RLock()
	filters := make(map[int64]LogsFilter)
	for id, f := range c.activeFilters {
		filters[id] = f
	}
	c.activeFiltersMu.RUnlock()

	c.subsMu.RLock()
	channels := make(map[int64]chan LogNotification)
	for id, ch := range c.subs {
		channels[id] = ch
	}
	c.subsMu.RUnlock()

	for oldSubID, filter := range filters {
		ch := channels[oldSubID]
		if ch == nil {
			continue
		}

		// Resubscribe
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		newSubID, err := c.subscribeLogsInternal(ctx, filter)
		cancel()

		if err != nil {
			// Failed to resubscribe, keep old mapping
			continue
		}

		// Update mappings with new subscription ID
		c.subsMu.Lock()
		delete(c.subs, oldSubID)
		c.subs[newSubID] = ch
		c.subsMu.Unlock()

		c.activeFiltersMu.Lock()
		delete(c.activeFilters, oldSubID)
		c.activeFilters[newSubID] = filter
		c.activeFiltersMu.Unlock()
	}
}

// subscribeLogsInternal subscribes without storing channel/filter.
func (c *WSClientImpl) subscribeLogsInternal(ctx context.Context, filter LogsFilter) (int64, error) {
	if c.closed.Load() {
		return 0, fmt.Errorf("client closed")
	}

	reqID := c.requestID.Add(1)

	mentionsFilter := make(map[string]interface{})
	if len(filter.Mentions) > 0 {
		mentionsFilter["mentions"] = filter.Mentions
	} else {
		mentionsFilter["all"] = nil
	}

	req := wsRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  "logsSubscribe",
		Params: []interface{}{
			mentionsFilter,
			map[string]string{"commitment": "confirmed"},
		},
	}

	confirmCh := make(chan int64, 1)
	c.pendingSubsMu.Lock()
	c.pendingSubs[reqID] = confirmCh
	c.pendingSubsMu.Unlock()

	c.connMu.Lock()
	if c.conn == nil {
		c.connMu.Unlock()
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return 0, fmt.Errorf("not connected")
	}

	c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	err := c.conn.WriteJSON(req)
	c.connMu.Unlock()

	if err != nil {
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return 0, fmt.Errorf("write subscribe: %w", err)
	}

	select {
	case subID := <-confirmCh:
		return subID, nil
	case <-time.After(30 * time.Second):
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return 0, fmt.Errorf("subscription timeout after 30s")
	case <-c.done:
		return 0, fmt.Errorf("client closed")
	case <-ctx.Done():
		c.pendingSubsMu.Lock()
		delete(c.pendingSubs, reqID)
		c.pendingSubsMu.Unlock()
		return 0, ctx.Err()
	}
}

// handleMessage processes incoming WebSocket message.
func (c *WSClientImpl) handleMessage(message []byte) {
	// Try to parse as subscription response first
	var resp wsSubscribeResponse
	if err := json.Unmarshal(message, &resp); err == nil && resp.Result > 0 {
		c.handleSubscribeResponse(&resp)
		return
	}

	// Try to parse as notification
	var notif wsNotification
	if err := json.Unmarshal(message, &notif); err == nil && notif.Method == "logsNotification" {
		c.handleLogsNotification(&notif)
		return
	}

	// Check for error response
	var errResp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      uint64 `json:"id"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(message, &errResp); err == nil && errResp.Error != nil {
		// Log error but don't crash - subscription will timeout
		fmt.Printf("[ws] Error response: code=%d msg=%s\n", errResp.Error.Code, errResp.Error.Message)
	}
}

// handleSubscribeResponse handles subscription confirmation.
func (c *WSClientImpl) handleSubscribeResponse(resp *wsSubscribeResponse) {
	c.pendingSubsMu.Lock()
	ch, ok := c.pendingSubs[resp.ID]
	if ok {
		delete(c.pendingSubs, resp.ID)
	}
	c.pendingSubsMu.Unlock()

	if ok {
		select {
		case ch <- resp.Result:
		default:
		}
	}
}

// handleLogsNotification dispatches log notification to subscriber.
func (c *WSClientImpl) handleLogsNotification(notif *wsNotification) {
	if notif.Params == nil {
		return
	}

	subID := notif.Params.Subscription
	value := notif.Params.Result.Value

	logNotif := LogNotification{
		Signature: value.Signature,
		Logs:      value.Logs,
		Err:       value.Err,
	}

	// Get slot from context if available
	if notif.Params.Result.Context != nil {
		logNotif.Slot = notif.Params.Result.Context.Slot
	}

	c.subsMu.RLock()
	ch, ok := c.subs[subID]
	c.subsMu.RUnlock()

	if ok {
		// Block until we can send - never drop events
		select {
		case ch <- logNotif:
		case <-c.done:
			return
		}
	}
}

// pingLoop sends periodic ping frames to keep connection alive.
func (c *WSClientImpl) pingLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			c.connMu.Lock()
			if c.conn != nil {
				c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					// Connection might be dead, reader will handle reconnect
				}
			}
			c.connMu.Unlock()
		}
	}
}

// WebSocket message types

type wsRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      uint64        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params,omitempty"`
}

type wsSubscribeResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Result  int64  `json:"result"` // subscription ID
}

type wsNotification struct {
	JSONRPC string                `json:"jsonrpc"`
	Method  string                `json:"method"`
	Params  *wsNotificationParams `json:"params"`
}

type wsNotificationParams struct {
	Subscription int64                `json:"subscription"`
	Result       wsNotificationResult `json:"result"`
}

type wsNotificationResult struct {
	Context *wsContext  `json:"context"`
	Value   wsLogsValue `json:"value"`
}

type wsContext struct {
	Slot int64 `json:"slot"`
}

type wsLogsValue struct {
	Signature string      `json:"signature"`
	Logs      []string    `json:"logs"`
	Err       interface{} `json:"err"`
}
