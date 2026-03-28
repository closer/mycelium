# Mycelium — Claude Code Inter-Session Communication MCP Server

## Overview

Mycelium is an MCP server that enables multiple Claude Code sessions on the same machine to discover each other and exchange messages. Named after fungal mycelium networks, it uses biological metaphors throughout its API.

**Key differentiators from existing solutions:**
- Broadcast & topic-based pub/sub messaging (sporulation)
- Structured task delegation with status tracking
- Automatic session summary from git context (no external API)
- Biological naming convention throughout

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
   │   localhost:7890 (default)      │
   │                                 │
   │  ┌───────────┐ ┌────────────┐  │
   │  │ Peer Store│ │ Message Q  │  │
   │  │ (in-mem)  │ │ (in-mem)   │  │
   │  └───────────┘ └────────────┘  │
   └─────────────────────────────────┘
```

- **MCP Server (stdio)**: One per Claude Code session. Spawned by Claude Code, communicates with broker via HTTP.
- **Broker Daemon**: Single process per machine. Listens on localhost only. Manages peer registration, discovery, message routing, and task tracking.
- **Auto-start**: MCP server starts the broker automatically if not running.

## Technology

- **Language**: Go
- **Storage**: In-memory (no persistence needed — all data is volatile)
- **CLI framework**: `github.com/spf13/cobra`
- **MCP SDK**: `github.com/mark3labs/mcp-go`
- **HTTP**: Standard library `net/http`
- **Single binary**: `mycelium` with subcommands

## Broker API

All endpoints are `POST`, JSON request/response, localhost only.

### Peer Lifecycle

| Endpoint | Request | Response | Description |
|---|---|---|---|
| `/germinate` | `{pid, cwd, repo?, summary?}` | `{peer_id}` | Register a peer. Returns UUID. |
| `/pulse` | `{peer_id}` | `{ok}` | Heartbeat (15s interval). |
| `/wither` | `{peer_id}` | `{ok}` | Explicit unregister. |
| `/identify` | `{peer_id, summary}` | `{ok}` | Update session summary. |
| `/discover` | `{peer_id, scope}` | `{peers: [...]}` | List peers. Scope: `machine` / `repo`. Excludes self. |

### Messaging

| Endpoint | Request | Response | Description |
|---|---|---|---|
| `/signal` | `{from, to, body}` | `{message_id}` | Send direct message. |
| `/sporulate` | `{from, body, topic?, scope}` | `{message_id}` | Broadcast to peers within scope. If `topic` is set, only topic subscribers within scope receive it. |
| `/sense` | `{peer_id}` | `{messages: [...]}` | Poll unread messages. Messages are deleted after retrieval. |
| `/attach` | `{peer_id, topic}` | `{ok}` | Subscribe to topic. |
| `/detach` | `{peer_id, topic}` | `{ok}` | Unsubscribe from topic. |

### Task Delegation

| Endpoint | Request | Response | Description |
|---|---|---|---|
| `/nurture` | `{from, to, description, context?}` | `{task_id}` | Delegate a task. |
| `/fruit` | `{task_id, status, result}` | `{ok}` | Report task result. Status: `completed` / `failed`. Notifies delegator via message. |
| `/survey` | `{peer_id}` | `{delegated: [...], received: [...]}` | List tasks (both delegated and received). |

### Peer Cleanup

- Peers with no heartbeat for 30 seconds are automatically removed.
- Every 30 seconds, broker checks PID liveness via `os.FindProcess` + signal 0.
- Dead peers' pending messages and tasks are cleaned up.

## MCP Tools

Tools exposed to Claude Code sessions:

### `discover`
- **Args**: `scope` (enum: `machine` | `repo`, default: `machine`)
- **Returns**: List of peers (`peer_id`, `cwd`, `repo`, `summary`, `topics`)
- **Description**: Explore the mycelium network to find other active sessions.

### `signal`
- **Args**: `to` (peer_id), `body` (string)
- **Returns**: `message_id`
- **Description**: Send a chemical signal to a specific peer.

### `sporulate`
- **Args**: `body` (string), `topic?` (string), `scope` (enum: `machine` | `repo`, default: `machine`)
- **Returns**: `message_id`
- **Description**: Release spores — broadcast a message to all peers or topic subscribers.

### `sense`
- **Args**: none
- **Returns**: List of messages (`from`, `from_summary`, `body`, `sent_at`, `topic?`)
- **Description**: Sense incoming signals and spores.

### `identify`
- **Args**: `summary` (string)
- **Returns**: `ok`
- **Description**: Identify this colony — set a summary visible to other peers.

### `attach`
- **Args**: `topic` (string)
- **Returns**: `ok`
- **Description**: Attach to a nutrient source — subscribe to a topic.

### `detach`
- **Args**: `topic` (string)
- **Returns**: `ok`
- **Description**: Detach from a nutrient source — unsubscribe from a topic.

### `nurture`
- **Args**: `to` (peer_id), `description` (string), `context?` (string)
- **Returns**: `task_id`
- **Description**: Send nutrients to grow a task — delegate work to another peer.

### `fruit`
- **Args**: `task_id` (string), `status` (enum: `completed` | `failed`), `result` (string)
- **Returns**: `ok`
- **Description**: Bear fruit — report the result of a delegated task.

### `survey`
- **Args**: none
- **Returns**: `{delegated: [...], received: [...]}`
- **Description**: Survey the mycelial bed — list all delegated and received tasks with their status.

## MCP Server Lifecycle

1. **Startup**: Check broker liveness → auto-start if needed → `/germinate` → auto-generate summary from git context
2. **Running**: `/pulse` every 15 seconds
3. **Shutdown**: `/wither` via `defer` + signal handling (`SIGINT`, `SIGTERM`)

### Auto Summary Generation

On startup, the MCP server generates a summary from:
- Directory name from `cwd`
- `git branch --show-current`
- `git log --oneline -3`

Format: `<dir> (<branch>) — latest: "<commit1>", "<commit2>", "<commit3>"`

Example: `mycelium (main) — latest: "fix: API validation", "feat: add user endpoint", "refactor: extract middleware"`

Claude can override this anytime via `identify`.

## Project Structure

```
mycelium/
├── cmd/
│   └── mycelium/
│       └── main.go            # Entry point (cobra root command)
├── internal/
│   ├── broker/
│   │   ├── broker.go          # HTTP server, routing, auto-cleanup
│   │   └── store.go           # In-memory Peer Store, Message Queue, Task Store
│   ├── mcp/
│   │   └── server.go          # MCP server (stdio), tool definitions
│   ├── client/
│   │   └── client.go          # Broker HTTP client
│   └── types/
│       └── types.go           # Peer, Message, Task shared types
├── go.mod
├── go.sum
└── CLAUDE.md
```

## CLI Subcommands

| Command | Description |
|---|---|
| `mycelium broker` | Start broker daemon (foreground) |
| `mycelium serve` | Start MCP server (stdio, called by Claude Code) |
| `mycelium status` | Show broker status and peer list |
| `mycelium send <peer_id> <msg>` | Send message from CLI (debug) |

## Claude Code Registration

```bash
go build -o mycelium ./cmd/mycelium
claude mcp add --scope user --transport stdio mycelium -- /path/to/mycelium serve
```

## Message Delivery

- **Polling-based**: Claude calls `sense` to check for messages.
- **No real-time push** in v1. Architecture supports adding channel push later when the Claude Code feature stabilizes.
- Task result notifications are delivered as messages (appear in `sense` output).

## Configuration

Environment variables:
- `MYCELIUM_PORT`: Broker port (default: `7890`)
- `MYCELIUM_HOST`: Broker host (default: `localhost`)

## Future Considerations

- Channel push support when Claude Code stabilizes the feature
- Shared key-value context store if a compelling use case emerges
