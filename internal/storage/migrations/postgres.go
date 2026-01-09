package migrations

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"solana-token-lab/internal/storage/postgres"
)

// RunPostgresMigrations applies all embedded SQL files in lexical order.
// Migrations are expected to be idempotent.
func RunPostgresMigrations(ctx context.Context, pool *postgres.Pool) error {
	entries, err := fs.ReadDir(PostgresFS, "postgres")
	if err != nil {
		return fmt.Errorf("read embedded postgres migrations: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		data, err := fs.ReadFile(PostgresFS, "postgres/"+file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			continue
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", file, err)
		}
	}

	return nil
}
