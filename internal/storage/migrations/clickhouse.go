package migrations

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strings"

	chstore "solana-token-lab/internal/storage/clickhouse"
)

// RunClickhouseMigrations ensures the database exists and applies all embedded SQL files.
// Returns a ClickHouse connection to the target database for reuse.
func RunClickhouseMigrations(ctx context.Context, dsn string) (*chstore.Conn, error) {
	dbName, err := databaseFromDSN(dsn)
	if err != nil {
		return nil, err
	}

	adminConn, err := chstore.NewConnWithDatabase(ctx, dsn, "")
	if err != nil {
		return nil, fmt.Errorf("connect clickhouse admin: %w", err)
	}
	if err := adminConn.Exec(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)); err != nil {
		adminConn.Close()
		return nil, fmt.Errorf("create database %s: %w", dbName, err)
	}
	if err := adminConn.Close(); err != nil {
		return nil, fmt.Errorf("close admin connection: %w", err)
	}

	conn, err := chstore.NewConnWithDatabase(ctx, dsn, dbName)
	if err != nil {
		return nil, fmt.Errorf("connect clickhouse db: %w", err)
	}

	entries, err := fs.ReadDir(ClickhouseFS, "clickhouse")
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read embedded clickhouse migrations: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		data, err := fs.ReadFile(ClickhouseFS, "clickhouse/"+file)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("read migration %s: %w", file, err)
		}

		// Validate SQL doesn't contain semicolons in strings (would break splitter)
		if err := validateNoSemicolonInStrings(string(data)); err != nil {
			conn.Close()
			return nil, fmt.Errorf("validate migration %s: %w", file, err)
		}

		// Execute each statement individually (split by semicolon)
		// ClickHouse driver doesn't support multiquery in Exec
		stmts := splitStatements(string(data))
		for _, stmt := range stmts {
			if err := conn.Exec(ctx, stmt); err != nil {
				conn.Close()
				return nil, fmt.Errorf("apply migration %s: %w", file, err)
			}
		}
	}

	return conn, nil
}

// splitStatements splits SQL content into individual statements by semicolon.
//
// IMPORTANT CONSTRAINT: This splitter is intentionally simple and does NOT handle:
//   - Semicolons inside string literals (e.g., 'foo;bar')
//   - Semicolons inside inline comments (e.g., /* foo; bar */)
//   - Dollar-quoted strings
//
// All ClickHouse migrations MUST follow these rules:
//  1. No semicolons inside string literals
//  2. Use -- style comments only (not /* */ with semicolons)
//  3. Each statement ends with a semicolon on its own line or at end of statement
//
// This constraint is validated at migration time - see validateNoSemicolonInStrings.
func splitStatements(input string) []string {
	var filtered []string
	for _, line := range strings.Split(input, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		filtered = append(filtered, line)
	}
	joined := strings.Join(filtered, "\n")

	var stmts []string
	for _, part := range strings.Split(joined, ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}

// validateNoSemicolonInStrings checks that SQL doesn't contain semicolons inside
// single-quoted strings, which would break our simple statement splitter.
// Returns an error if a dangerous pattern is detected.
func validateNoSemicolonInStrings(sql string) error {
	inString := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if ch == '\'' {
			// Handle escaped quotes ''
			if i+1 < len(sql) && sql[i+1] == '\'' {
				i++ // skip next quote
				continue
			}
			inString = !inString
		} else if ch == ';' && inString {
			return fmt.Errorf("semicolon found inside string literal - this breaks the migration splitter")
		}
	}
	return nil
}

func databaseFromDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse clickhouse dsn: %w", err)
	}
	db := strings.TrimPrefix(u.Path, "/")
	if db == "" {
		return "", fmt.Errorf("clickhouse dsn missing database")
	}
	return db, nil
}
