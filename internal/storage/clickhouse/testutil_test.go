package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupTestDB creates a ClickHouse container and returns a connection.
// Returns a cleanup function that must be called when done.
func setupTestDB(t *testing.T) (*Conn, func()) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start ClickHouse container
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:24.1-alpine",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Application: Ready for connections").
				WithStartupTimeout(60 * time.Second),
			wait.ForListeningPort("9000/tcp"),
		),
		Env: map[string]string{
			"CLICKHOUSE_DB":       "test",
			"CLICKHOUSE_USER":     "default",
			"CLICKHOUSE_PASSWORD": "",
		},
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	// Get native port (9000)
	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	dsn := fmt.Sprintf("clickhouse://%s:%s/test", host, port.Port())

	// Connect to ClickHouse
	conn, err := NewConn(ctx, dsn)
	require.NoError(t, err)

	// Run migrations
	runMigrations(t, conn)

	cleanup := func() {
		conn.Close()
		_ = container.Terminate(ctx)
	}

	return conn, cleanup
}

// runMigrations applies all SQL migrations from sql/clickhouse/
func runMigrations(t *testing.T, conn *Conn) {
	t.Helper()
	ctx := context.Background()

	// Find project root by looking for sql/clickhouse directory
	migrations := []string{
		"001_timeseries.sql",
		"002_derived_features.sql",
		"003_feature_views.sql",
		"004_strategy_aggregates.sql",
	}

	// Try to find the sql directory
	basePath := findSQLDir()

	for _, m := range migrations {
		path := basePath + "/" + m
		content, err := os.ReadFile(path)
		if err != nil {
			t.Logf("Could not read migration %s: %v, trying inline migrations", m, err)
			// Fall back to inline migrations
			runInlineMigrations(t, conn)
			return
		}

		err = conn.Exec(ctx, string(content))
		require.NoError(t, err, "failed to apply migration %s", m)
	}
}

// findSQLDir attempts to locate the sql/clickhouse directory
func findSQLDir() string {
	paths := []string{
		"../../../sql/clickhouse",
		"../../sql/clickhouse",
		"sql/clickhouse",
		"./sql/clickhouse",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Default path
	return "../../../sql/clickhouse"
}

// runInlineMigrations applies migrations directly without reading files
func runInlineMigrations(t *testing.T, conn *Conn) {
	t.Helper()
	ctx := context.Background()

	// 001_timeseries.sql
	err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS price_timeseries (
			candidate_id        String,
			timestamp_ms        UInt64,
			slot                UInt64,
			price               Float64,
			volume              Float64,
			swap_count          UInt32
		) ENGINE = MergeTree()
		ORDER BY (candidate_id, timestamp_ms)
		SETTINGS index_granularity = 8192
	`)
	require.NoError(t, err)

	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS liquidity_timeseries (
			candidate_id        String,
			timestamp_ms        UInt64,
			slot                UInt64,
			liquidity           Float64,
			liquidity_token     Float64,
			liquidity_quote     Float64
		) ENGINE = MergeTree()
		ORDER BY (candidate_id, timestamp_ms)
		SETTINGS index_granularity = 8192
	`)
	require.NoError(t, err)

	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS volume_timeseries (
			candidate_id        String,
			timestamp_ms        UInt64,
			interval_seconds    UInt32,
			volume              Float64,
			swap_count          UInt32,
			buy_volume          Float64,
			sell_volume         Float64
		) ENGINE = MergeTree()
		ORDER BY (candidate_id, interval_seconds, timestamp_ms)
		SETTINGS index_granularity = 8192
	`)
	require.NoError(t, err)

	// 002_derived_features.sql
	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS derived_features (
			candidate_id                String,
			timestamp_ms                UInt64,
			price_delta                 Nullable(Float64),
			price_velocity              Nullable(Float64),
			price_acceleration          Nullable(Float64),
			liquidity_delta             Nullable(Float64),
			liquidity_velocity          Nullable(Float64),
			token_lifetime_ms           UInt64,
			last_swap_interval_ms       Nullable(UInt64),
			last_liq_event_interval_ms  Nullable(UInt64)
		) ENGINE = MergeTree()
		ORDER BY (candidate_id, timestamp_ms)
		SETTINGS index_granularity = 8192
	`)
	require.NoError(t, err)

	// 004_strategy_aggregates.sql
	err = conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS strategy_aggregates (
			strategy_id String,
			scenario_id String,
			entry_event_type String,
			total_trades UInt32,
			total_tokens UInt32,
			wins UInt32,
			losses UInt32,
			win_rate Float64,
			token_win_rate Float64,
			outcome_mean Float64,
			outcome_median Float64,
			outcome_p10 Float64,
			outcome_p25 Float64,
			outcome_p75 Float64,
			outcome_p90 Float64,
			outcome_min Float64,
			outcome_max Float64,
			outcome_stddev Float64,
			max_drawdown Float64,
			max_consecutive_losses UInt32,
			outcome_realistic Nullable(Float64),
			outcome_pessimistic Nullable(Float64),
			outcome_degraded Nullable(Float64),
			created_at DateTime DEFAULT now()
		)
		ENGINE = ReplacingMergeTree(created_at)
		ORDER BY (strategy_id, scenario_id, entry_event_type)
		SETTINGS index_granularity = 8192
	`)
	require.NoError(t, err)
}

// ptr is a helper to create pointers for test values
func ptr[T any](v T) *T {
	return &v
}
