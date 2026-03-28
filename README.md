# Mycelium

Inter-session communication MCP server for Claude Code.

Multiple Claude Code sessions on the same machine can discover each other, exchange messages, broadcast via topics, and delegate tasks — like a fungal mycelium network connecting organisms underground.

## Features

- **Peer Discovery** — Find other active Claude Code sessions (machine-wide or same git repo)
- **Direct Messaging** — Send signals between specific sessions
- **Topic Broadcast** — Publish/subscribe messaging via named topics
- **Task Delegation** — Structured request/response workflow between sessions
- **Auto Summary** — Sessions auto-identify from git branch and recent commits
- **Channel Push** (experimental) — Real-time message delivery without polling

## Architecture

```
┌─────────────────┐     ┌─────────────────┐
│ Claude Code (A)  │     │ Claude Code (B)  │
│                  │     │                  │
│  ┌────────────┐  │     │  ┌────────────┐  │
│  │ MCP Server │  │     │  │ MCP Server │  │
│  │ (stdio)    │  │     │  │ (stdio)    │  │
│  └─────┬──────┘  │     │  └─────┬──────┘  │
└────────┼─────────┘     └────────┼─────────┘
         │ HTTP                   │ HTTP
         ▼                       ▼
   ┌─────────────────────────────────┐
   │         Broker Daemon           │
   │      localhost:7890             │
   │   (in-memory, auto-started)    │
   └─────────────────────────────────┘
```

Single Go binary. The broker starts automatically when the first session connects.

## Installation

```bash
go install github.com/closer/mycelium/cmd/mycelium@latest
```

Or build from source:

```bash
git clone https://github.com/closer/mycelium.git
cd mycelium
go build -o mycelium ./cmd/mycelium
```

## Setup

Register with Claude Code:

```bash
claude mcp add --scope user --transport stdio mycelium -- /path/to/mycelium serve
```

For channel push (experimental, real-time message delivery):

```bash
claude --dangerously-load-development-channels server:mycelium
```

## MCP Tools

All tool names follow a biological metaphor inspired by mycelium networks.

| Tool | Description |
|------|-------------|
| `discover` | Explore the network to find other active sessions |
| `signal` | Send a direct message to a specific peer |
| `sporulate` | Broadcast a message to all peers or topic subscribers |
| `sense` | Check for incoming messages |
| `identify` | Set a summary of your current work |
| `attach` | Subscribe to a topic |
| `detach` | Unsubscribe from a topic |
| `nurture` | Delegate a task to another peer |
| `fruit` | Report the result of a delegated task |
| `survey` | List all delegated and received tasks |

## Usage Examples

### Session Discovery

```
You: discover で他のセッションを探して
Claude: [calls discover] → 2 peers found:
  - backend (feat/api) — latest: "feat: add /users endpoint"
  - frontend (main) — latest: "fix: button style"
```

### Cross-Session Messaging

```
You: backend セッションに「API スキーマ更新したよ」と伝えて
Claude: [calls signal] → Message sent
```

With channel push enabled, the message arrives instantly in the other session without any action needed.

### Task Delegation

```
You: frontend セッションに CORS ミドルウェアの追加を依頼して
Claude: [calls nurture] → Task delegated (id: abc-123)

# Later, in the frontend session:
Claude: [calls fruit] → Task completed

# Back in this session (auto-delivered via channel push):
<channel source="mycelium">
[From: frontend] Task "Add CORS middleware" completed: Done with tests
</channel>
```

### Topic Broadcast

```
You: deploy トピックに「v2.0 をステージングにデプロイ中」とブロードキャストして
Claude: [calls sporulate] → Spores released

# All sessions subscribed to "deploy" receive the message
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `mycelium broker` | Start the broker daemon (usually auto-started) |
| `mycelium serve` | Start the MCP server (called by Claude Code) |
| `mycelium status` | Show broker status and active peer list |
| `mycelium send <id> <msg>` | Send a message from the command line (debug) |

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MYCELIUM_PORT` | `7890` | Broker port |
| `MYCELIUM_HOST` | `localhost` | Broker host |

## Project Structure

```
mycelium/
├── cmd/mycelium/       CLI entry point (cobra)
├── internal/
│   ├── types/          Shared types (Peer, Message, Task)
│   ├── broker/         HTTP broker daemon + in-memory store
│   ├── client/         Broker HTTP client
│   └── mcp/            MCP server (stdio) + tool definitions
├── integration_test.go End-to-end test
└── CLAUDE.md           AI assistant context
```

## License

MIT
