# Mycelium

Inter-session communication MCP server for Claude Code.

## Build & Run

go build -o mycelium ./cmd/mycelium

## Test

go test ./...

## Architecture

- `cmd/mycelium/` — CLI entry point (cobra)
- `internal/types/` — Shared types (Peer, Message, Task)
- `internal/broker/` — HTTP broker daemon (in-memory store)
- `internal/client/` — Broker HTTP client
- `internal/mcp/` — MCP server (stdio), tool definitions
