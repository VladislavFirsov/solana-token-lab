package postgres

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// testDB holds the test database container and pool.
type testDB struct {
	container testcontainers.Container
	pool      *Pool
}

// setupTestDB creates a PostgreSQL container for testing and applies migrations.
// Returns a cleanup function that must be called after tests complete.
func setupTestDB(t *testing.T) (*Pool, func()) {
	t.Helper()

	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "failed to get connection string")

	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err, "failed to create pool")

	// Run migrations
	runMigrations(t, ctx, pool)

	cleanup := func() {
		pool.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return pool, cleanup
}

// runMigrations applies all SQL migrations from sql/postgres/ directory.
func runMigrations(t *testing.T, ctx context.Context, pool *Pool) {
	t.Helper()

	// Find project root by looking for go.mod
	projectRoot := findProjectRoot(t)
	migrationsDir := filepath.Join(projectRoot, "sql", "postgres")

	// Read migration files
	entries, err := os.ReadDir(migrationsDir)
	require.NoError(t, err, "failed to read migrations directory")

	// Sort files by name (001_, 002_, etc.)
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// Execute each migration
	for _, file := range files {
		filePath := filepath.Join(migrationsDir, file)
		sql, err := os.ReadFile(filePath)
		require.NoError(t, err, "failed to read migration file: %s", file)

		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, "failed to execute migration: %s", file)

		t.Logf("Applied migration: %s", file)
	}
}

// findProjectRoot walks up from current directory to find go.mod.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	// Start from the current working directory
	dir, err := os.Getwd()
	require.NoError(t, err, "failed to get working directory")

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// ptr is a helper to create pointers to values.
func ptr[T any](v T) *T {
	return &v
}
