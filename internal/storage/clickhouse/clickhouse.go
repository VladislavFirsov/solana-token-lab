package clickhouse

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Conn wraps clickhouse driver.Conn for dependency injection.
type Conn struct {
	driver.Conn
}

// NewConn creates a new ClickHouse connection.
func NewConn(ctx context.Context, dsn string) (*Conn, error) {
	opts, err := parseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}

	// Verify connection
	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	return &Conn{Conn: conn}, nil
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.Conn.Close()
}

// parseDSN parses ClickHouse DSN string into Options.
// Supports format: clickhouse://user:password@host:port/database
func parseDSN(dsn string) (*clickhouse.Options, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn url: %w", err)
	}

	opts := &clickhouse.Options{
		Protocol: clickhouse.Native,
	}

	// Host and port
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "9000" // default ClickHouse native port
	}
	opts.Addr = []string{fmt.Sprintf("%s:%s", host, port)}

	// Auth
	if u.User != nil {
		opts.Auth.Username = u.User.Username()
		if password, ok := u.User.Password(); ok {
			opts.Auth.Password = password
		}
	}

	// Database
	if len(u.Path) > 1 {
		opts.Auth.Database = strings.TrimPrefix(u.Path, "/")
	}

	return opts, nil
}

// isDuplicateKeyError checks if error indicates a duplicate key.
// Note: ClickHouse MergeTree doesn't enforce uniqueness, but we can detect
// during explicit duplicate checks.
func isDuplicateKeyError(_ error) bool {
	// ClickHouse MergeTree doesn't enforce uniqueness at insert time.
	// We handle duplicates via ReplacingMergeTree or explicit checks before insert.
	// This function is kept for API compatibility.
	return false
}
