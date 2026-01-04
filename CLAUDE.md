# CLAUDE.md - Project Guidelines for Claude Code

## Project Overview
Go service for Solana token analysis.

## Permissions

### Allowed
- All bash commands within this project directory
- All git commands (init, add, commit, push, pull, branch, etc.)
- File operations (create, edit, delete) within /Users/vladislav/Work/solana-token-lab/
- Running Go commands (go build, go run, go test, go mod, etc.)
- Package management (go get, go mod tidy)

### Denied
- Network commands (curl, wget, nc, ssh to external hosts)
- Modifying files outside this repository
- System-level changes

## Tech Stack
- Language: Go
- Target: Solana blockchain token analysis

## Project Structure (Go conventions)
```
/cmd           - Application entrypoints
/internal      - Private application code
/pkg           - Public libraries
/api           - API definitions (OpenAPI, protobuf)
/configs       - Configuration files
/scripts       - Build/deploy scripts
```

## Commands
- Build: `go build ./...`
- Test: `go test ./...`
- Run: `go run cmd/server/main.go`