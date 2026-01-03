package solana

import "context"

// WSClient defines Solana WebSocket subscription interface.
type WSClient interface {
	// SubscribeLogs subscribes to program logs matching the filter.
	SubscribeLogs(ctx context.Context, filter LogsFilter) (<-chan LogNotification, error)

	// Close closes the WebSocket connection.
	Close() error
}

// LogsFilter defines subscription filter for logs.
type LogsFilter struct {
	// Mentions filters logs that mention any of these program IDs.
	Mentions []string
}

// LogNotification represents a logs subscription message.
type LogNotification struct {
	Signature string
	Slot      int64
	Logs      []string
	Err       interface{}
}
