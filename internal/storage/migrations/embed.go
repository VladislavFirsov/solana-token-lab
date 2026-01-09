package migrations

import "embed"

// PostgresFS embeds all PostgreSQL migration files.
//
//go:embed postgres/*.sql
var PostgresFS embed.FS

// ClickhouseFS embeds all ClickHouse migration files.
//
//go:embed clickhouse/*.sql
var ClickhouseFS embed.FS
