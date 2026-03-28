# Mycelium Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go-based MCP server enabling multiple Claude Code sessions on the same machine to discover each other, exchange messages, broadcast via topics, and delegate tasks.

**Architecture:** Single binary (`mycelium`) with cobra subcommands. A broker daemon (HTTP, in-memory store) handles peer registration, messaging, and task tracking. Each Claude Code session spawns a stdio MCP server that connects to the broker. The MCP server auto-starts the broker if needed.

**Tech Stack:** Go, `github.com/mark3labs/mcp-go`, `github.com/spf13/cobra`, `net/http`

**Spec:** `docs/superpowers/specs/2026-03-29-mycelium-design.md`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/mycelium/main.go`
- Create: `CLAUDE.md`

- [ ] **Step 1: Initialize Go module**

Run: `go mod init github.com/closer/mycelium`
Expected: `go.mod` created

- [ ] **Step 2: Install dependencies**

Run: `go get github.com/spf13/cobra github.com/mark3labs/mcp-go github.com/google/uuid`
Expected: `go.sum` created, dependencies in `go.mod`

- [ ] **Step 3: Create minimal main.go with cobra root command**

```go
// cmd/mycelium/main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mycelium",
	Short: "Inter-session communication for Claude Code",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Create CLAUDE.md**

```markdown
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
```

- [ ] **Step 5: Verify it builds**

Run: `go build ./cmd/mycelium`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/ CLAUDE.md
git commit -m "feat: project scaffolding with cobra root command"
```

---

### Task 2: Shared Types

**Files:**
- Create: `internal/types/types.go`
- Create: `internal/types/types_test.go`

- [ ] **Step 1: Write tests for type constructors**

```go
// internal/types/types_test.go
package types_test

import (
	"testing"

	"github.com/closer/mycelium/internal/types"
)

func TestNewPeer(t *testing.T) {
	p := types.NewPeer(1234, "/home/user/project", "git@github.com:user/project.git", "working on API")
	if p.ID == "" {
		t.Fatal("peer ID should not be empty")
	}
	if p.PID != 1234 {
		t.Fatalf("expected PID 1234, got %d", p.PID)
	}
	if p.CWD != "/home/user/project" {
		t.Fatalf("expected CWD /home/user/project, got %s", p.CWD)
	}
	if p.Repo != "git@github.com:user/project.git" {
		t.Fatalf("expected repo, got %s", p.Repo)
	}
	if p.Summary != "working on API" {
		t.Fatalf("expected summary, got %s", p.Summary)
	}
	if len(p.Topics) != 0 {
		t.Fatal("topics should be empty initially")
	}
}

func TestNewMessage(t *testing.T) {
	m := types.NewMessage("peer-a", "peer-b", "hello", "")
	if m.ID == "" {
		t.Fatal("message ID should not be empty")
	}
	if m.From != "peer-a" {
		t.Fatalf("expected from peer-a, got %s", m.From)
	}
	if m.To != "peer-b" {
		t.Fatalf("expected to peer-b, got %s", m.To)
	}
	if m.Body != "hello" {
		t.Fatalf("expected body hello, got %s", m.Body)
	}
	if m.SentAt.IsZero() {
		t.Fatal("sent_at should be set")
	}
}

func TestNewTask(t *testing.T) {
	task := types.NewTask("peer-a", "peer-b", "write tests", "for the API module")
	if task.ID == "" {
		t.Fatal("task ID should not be empty")
	}
	if task.Status != types.TaskStatusPending {
		t.Fatalf("expected status pending, got %s", task.Status)
	}
	if task.Description != "write tests" {
		t.Fatalf("expected description, got %s", task.Description)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/types/`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement types**

```go
// internal/types/types.go
package types

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

type Peer struct {
	ID        string    `json:"peer_id"`
	PID       int       `json:"pid"`
	CWD       string    `json:"cwd"`
	Repo      string    `json:"repo,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	Topics    []string  `json:"topics,omitempty"`
	LastPulse time.Time `json:"last_pulse"`
}

func NewPeer(pid int, cwd, repo, summary string) Peer {
	return Peer{
		ID:        uuid.New().String(),
		PID:       pid,
		CWD:       cwd,
		Repo:      repo,
		Summary:   summary,
		Topics:    []string{},
		LastPulse: time.Now(),
	}
}

type Message struct {
	ID     string    `json:"message_id"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Body   string    `json:"body"`
	Topic  string    `json:"topic,omitempty"`
	SentAt time.Time `json:"sent_at"`
}

func NewMessage(from, to, body, topic string) Message {
	return Message{
		ID:     uuid.New().String(),
		From:   from,
		To:     to,
		Body:   body,
		Topic:  topic,
		SentAt: time.Now(),
	}
}

type Task struct {
	ID          string     `json:"task_id"`
	From        string     `json:"from"`
	To          string     `json:"to"`
	Description string     `json:"description"`
	Context     string     `json:"context,omitempty"`
	Status      TaskStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewTask(from, to, description, context string) Task {
	return Task{
		ID:          uuid.New().String(),
		From:        from,
		To:          to,
		Description: description,
		Context:     context,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/types/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/types/
git commit -m "feat: add shared types for Peer, Message, Task"
```

---

### Task 3: In-Memory Store

**Files:**
- Create: `internal/broker/store.go`
- Create: `internal/broker/store_test.go`

- [ ] **Step 1: Write tests for peer operations**

```go
// internal/broker/store_test.go
package broker_test

import (
	"testing"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/types"
)

func TestStore_RegisterAndGetPeer(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1234, "/tmp/project", "repo-url", "summary")
	s.RegisterPeer(peer)

	got, ok := s.GetPeer(peer.ID)
	if !ok {
		t.Fatal("expected to find peer")
	}
	if got.PID != 1234 {
		t.Fatalf("expected PID 1234, got %d", got.PID)
	}
}

func TestStore_ListPeers_Machine(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "", "")
	p2 := types.NewPeer(2, "/b", "", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	peers := s.ListPeers(p1.ID, "machine")
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer (excluding self), got %d", len(peers))
	}
	if peers[0].ID != p2.ID {
		t.Fatalf("expected peer %s, got %s", p2.ID, peers[0].ID)
	}
}

func TestStore_ListPeers_Repo(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "same-repo", "")
	p2 := types.NewPeer(2, "/b", "same-repo", "")
	p3 := types.NewPeer(3, "/c", "other-repo", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	peers := s.ListPeers(p1.ID, "repo")
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer in same repo, got %d", len(peers))
	}
	if peers[0].ID != p2.ID {
		t.Fatalf("expected peer %s, got %s", p2.ID, peers[0].ID)
	}
}

func TestStore_UnregisterPeer(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1, "/a", "", "")
	s.RegisterPeer(peer)
	s.UnregisterPeer(peer.ID)

	_, ok := s.GetPeer(peer.ID)
	if ok {
		t.Fatal("peer should have been removed")
	}
}

func TestStore_UpdateSummary(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1, "/a", "", "old")
	s.RegisterPeer(peer)
	s.UpdateSummary(peer.ID, "new summary")

	got, _ := s.GetPeer(peer.ID)
	if got.Summary != "new summary" {
		t.Fatalf("expected new summary, got %s", got.Summary)
	}
}

func TestStore_Pulse(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1, "/a", "", "")
	s.RegisterPeer(peer)

	before, _ := s.GetPeer(peer.ID)
	s.Pulse(peer.ID)
	after, _ := s.GetPeer(peer.ID)

	if !after.LastPulse.After(before.LastPulse) || after.LastPulse.Equal(before.LastPulse) {
		// LastPulse might be equal if test is fast, so just check it's not before
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/broker/`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement Store (peer operations)**

```go
// internal/broker/store.go
package broker

import (
	"sync"
	"time"

	"github.com/closer/mycelium/internal/types"
)

type Store struct {
	mu       sync.RWMutex
	peers    map[string]types.Peer
	messages map[string][]types.Message // peer_id -> messages
	tasks    map[string]types.Task      // task_id -> task
}

func NewStore() *Store {
	return &Store{
		peers:    make(map[string]types.Peer),
		messages: make(map[string][]types.Message),
		tasks:    make(map[string]types.Task),
	}
}

func (s *Store) RegisterPeer(p types.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[p.ID] = p
}

func (s *Store) GetPeer(id string) (types.Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[id]
	return p, ok
}

func (s *Store) UnregisterPeer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, id)
	delete(s.messages, id)
	// Clean up tasks where this peer is involved
	for taskID, task := range s.tasks {
		if task.From == id || task.To == id {
			delete(s.tasks, taskID)
		}
	}
}

func (s *Store) ListPeers(selfID, scope string) []types.Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	self, ok := s.peers[selfID]
	if !ok {
		return nil
	}

	var result []types.Peer
	for _, p := range s.peers {
		if p.ID == selfID {
			continue
		}
		switch scope {
		case "repo":
			if p.Repo != "" && p.Repo == self.Repo {
				result = append(result, p)
			}
		default: // "machine"
			result = append(result, p)
		}
	}
	return result
}

func (s *Store) UpdateSummary(id, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.Summary = summary
		s.peers[id] = p
	}
}

func (s *Store) Pulse(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.LastPulse = time.Now()
		s.peers[id] = p
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/broker/`
Expected: PASS

- [ ] **Step 5: Add message tests**

```go
// Append to internal/broker/store_test.go

func TestStore_SignalAndSense(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "", "")
	p2 := types.NewPeer(2, "/b", "", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	msg := types.NewMessage(p1.ID, p2.ID, "hello", "")
	s.EnqueueMessage(msg)

	messages := s.DrainMessages(p2.ID)
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Body != "hello" {
		t.Fatalf("expected body hello, got %s", messages[0].Body)
	}

	// Messages should be drained
	messages = s.DrainMessages(p2.ID)
	if len(messages) != 0 {
		t.Fatalf("expected 0 messages after drain, got %d", len(messages))
	}
}

func TestStore_Sporulate_Machine(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "", "")
	p2 := types.NewPeer(2, "/b", "", "")
	p3 := types.NewPeer(3, "/c", "", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	s.Broadcast(p1.ID, "spore msg", "", "machine")

	m2 := s.DrainMessages(p2.ID)
	m3 := s.DrainMessages(p3.ID)
	m1 := s.DrainMessages(p1.ID)

	if len(m2) != 1 || len(m3) != 1 {
		t.Fatalf("expected 1 message each for p2 and p3, got %d and %d", len(m2), len(m3))
	}
	if len(m1) != 0 {
		t.Fatalf("sender should not receive own broadcast, got %d", len(m1))
	}
}

func TestStore_Sporulate_Topic(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "", "")
	p2 := types.NewPeer(2, "/b", "", "")
	p3 := types.NewPeer(3, "/c", "", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	s.Subscribe(p2.ID, "deploy")

	s.Broadcast(p1.ID, "deploying now", "deploy", "machine")

	m2 := s.DrainMessages(p2.ID)
	m3 := s.DrainMessages(p3.ID)

	if len(m2) != 1 {
		t.Fatalf("p2 (subscribed) should get message, got %d", len(m2))
	}
	if len(m3) != 0 {
		t.Fatalf("p3 (not subscribed) should not get message, got %d", len(m3))
	}
}

func TestStore_AttachDetach(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1, "/a", "", "")
	s.RegisterPeer(peer)

	s.Subscribe(peer.ID, "deploy")
	got, _ := s.GetPeer(peer.ID)
	if len(got.Topics) != 1 || got.Topics[0] != "deploy" {
		t.Fatalf("expected [deploy], got %v", got.Topics)
	}

	s.Unsubscribe(peer.ID, "deploy")
	got, _ = s.GetPeer(peer.ID)
	if len(got.Topics) != 0 {
		t.Fatalf("expected empty topics, got %v", got.Topics)
	}
}
```

- [ ] **Step 6: Implement message and topic operations**

Add to `internal/broker/store.go`:

```go
func (s *Store) EnqueueMessage(msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[msg.To] = append(s.messages[msg.To], msg)
}

func (s *Store) DrainMessages(peerID string) []types.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.messages[peerID]
	delete(s.messages, peerID)
	return msgs
}

func (s *Store) Broadcast(fromID, body, topic, scope string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	from, ok := s.peers[fromID]
	if !ok {
		return
	}

	for _, p := range s.peers {
		if p.ID == fromID {
			continue
		}

		// Scope filter
		if scope == "repo" && (p.Repo == "" || p.Repo != from.Repo) {
			continue
		}

		// Topic filter
		if topic != "" && !containsString(p.Topics, topic) {
			continue
		}

		msg := types.NewMessage(fromID, p.ID, body, topic)
		s.messages[p.ID] = append(s.messages[p.ID], msg)
	}
}

func (s *Store) Subscribe(peerID, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[peerID]; ok {
		if !containsString(p.Topics, topic) {
			p.Topics = append(p.Topics, topic)
			s.peers[peerID] = p
		}
	}
}

func (s *Store) Unsubscribe(peerID, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[peerID]; ok {
		p.Topics = removeString(p.Topics, topic)
		s.peers[peerID] = p
	}
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(ss []string, s string) []string {
	result := make([]string, 0, len(ss))
	for _, v := range ss {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/broker/`
Expected: PASS

- [ ] **Step 8: Add task tests**

```go
// Append to internal/broker/store_test.go

func TestStore_NurtureAndFruit(t *testing.T) {
	s := broker.NewStore()
	p1 := types.NewPeer(1, "/a", "", "worker A")
	p2 := types.NewPeer(2, "/b", "", "worker B")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	task := types.NewTask(p1.ID, p2.ID, "write tests", "for the API")
	s.CreateTask(task)

	// Survey from delegator
	delegated, received := s.ListTasks(p1.ID)
	if len(delegated) != 1 {
		t.Fatalf("expected 1 delegated task, got %d", len(delegated))
	}
	if len(received) != 0 {
		t.Fatalf("expected 0 received tasks for p1, got %d", len(received))
	}

	// Survey from assignee
	delegated2, received2 := s.ListTasks(p2.ID)
	if len(delegated2) != 0 {
		t.Fatalf("expected 0 delegated tasks for p2, got %d", len(delegated2))
	}
	if len(received2) != 1 {
		t.Fatalf("expected 1 received task, got %d", len(received2))
	}

	// Complete task
	s.CompleteTask(task.ID, types.TaskStatusCompleted, "done")

	got, ok := s.GetTask(task.ID)
	if !ok {
		t.Fatal("task should exist")
	}
	if got.Status != types.TaskStatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}
	if got.Result != "done" {
		t.Fatalf("expected result done, got %s", got.Result)
	}

	// Completion should notify delegator
	msgs := s.DrainMessages(p1.ID)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification message, got %d", len(msgs))
	}
}
```

- [ ] **Step 9: Implement task operations**

Add to `internal/broker/store.go`:

```go
func (s *Store) CreateTask(task types.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

func (s *Store) GetTask(id string) (types.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

func (s *Store) CompleteTask(taskID string, status types.TaskStatus, result string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return false
	}
	task.Status = status
	task.Result = result
	s.tasks[taskID] = task

	// Notify delegator via message
	body := fmt.Sprintf("Task \"%s\" %s: %s", task.Description, status, result)
	msg := types.NewMessage(task.To, task.From, body, "")
	s.messages[task.From] = append(s.messages[task.From], msg)
	return true
}

func (s *Store) ListTasks(peerID string) (delegated, received []types.Task) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.From == peerID {
			delegated = append(delegated, t)
		}
		if t.To == peerID {
			received = append(received, t)
		}
	}
	return
}
```

Add `"fmt"` to the import block in `store.go`.

- [ ] **Step 10: Run all tests**

Run: `go test ./internal/broker/`
Expected: PASS

- [ ] **Step 11: Add cleanup test and implement**

```go
// Append to internal/broker/store_test.go

func TestStore_CleanupStalePeers(t *testing.T) {
	s := broker.NewStore()
	peer := types.NewPeer(1, "/a", "", "")
	s.RegisterPeer(peer)

	// Manually set LastPulse to 60 seconds ago
	s.SetLastPulse(peer.ID, time.Now().Add(-60*time.Second))

	removed := s.CleanupStalePeers(30 * time.Second)
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	_, ok := s.GetPeer(peer.ID)
	if ok {
		t.Fatal("stale peer should have been removed")
	}
}
```

Add to `internal/broker/store.go`:

```go
func (s *Store) SetLastPulse(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.LastPulse = t
		s.peers[id] = p
	}
}

func (s *Store) CleanupStalePeers(timeout time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	now := time.Now()
	for id, p := range s.peers {
		if now.Sub(p.LastPulse) > timeout {
			delete(s.peers, id)
			delete(s.messages, id)
			for taskID, task := range s.tasks {
				if task.From == id || task.To == id {
					delete(s.tasks, taskID)
				}
			}
			count++
		}
	}
	return count
}
```

- [ ] **Step 12: Run all tests**

Run: `go test ./internal/broker/`
Expected: PASS

- [ ] **Step 13: Commit**

```bash
git add internal/broker/
git commit -m "feat: add in-memory store with peer, message, topic, and task operations"
```

---

### Task 4: Broker HTTP Server

**Files:**
- Create: `internal/broker/broker.go`
- Create: `internal/broker/broker_test.go`

- [ ] **Step 1: Write tests for broker HTTP endpoints**

```go
// internal/broker/broker_test.go
package broker_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/closer/mycelium/internal/broker"
)

func postJSON(t *testing.T, handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestBroker_GerminateAndDiscover(t *testing.T) {
	b := broker.New()
	handler := b.Handler()

	// Register peer 1
	w := postJSON(t, handler, "/germinate", map[string]any{
		"pid": 1, "cwd": "/a", "repo": "repo-1", "summary": "peer 1",
	})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var reg1 map[string]string
	decodeJSON(t, w, &reg1)
	if reg1["peer_id"] == "" {
		t.Fatal("expected peer_id")
	}

	// Register peer 2
	w = postJSON(t, handler, "/germinate", map[string]any{
		"pid": 2, "cwd": "/b", "repo": "repo-1", "summary": "peer 2",
	})
	var reg2 map[string]string
	decodeJSON(t, w, &reg2)

	// Discover from peer 1
	w = postJSON(t, handler, "/discover", map[string]any{
		"peer_id": reg1["peer_id"], "scope": "machine",
	})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var disc map[string][]map[string]any
	decodeJSON(t, w, &disc)
	if len(disc["peers"]) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(disc["peers"]))
	}
}

func TestBroker_SignalAndSense(t *testing.T) {
	b := broker.New()
	handler := b.Handler()

	// Register two peers
	w := postJSON(t, handler, "/germinate", map[string]any{"pid": 1, "cwd": "/a"})
	var reg1 map[string]string
	decodeJSON(t, w, &reg1)

	w = postJSON(t, handler, "/germinate", map[string]any{"pid": 2, "cwd": "/b"})
	var reg2 map[string]string
	decodeJSON(t, w, &reg2)

	// Signal
	w = postJSON(t, handler, "/signal", map[string]any{
		"from": reg1["peer_id"], "to": reg2["peer_id"], "body": "hello",
	})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Sense
	w = postJSON(t, handler, "/sense", map[string]any{"peer_id": reg2["peer_id"]})
	var msgs map[string][]map[string]any
	decodeJSON(t, w, &msgs)
	if len(msgs["messages"]) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs["messages"]))
	}
	if msgs["messages"][0]["body"] != "hello" {
		t.Fatalf("expected body hello, got %v", msgs["messages"][0]["body"])
	}
}

func TestBroker_WitherCleanup(t *testing.T) {
	b := broker.New()
	handler := b.Handler()

	w := postJSON(t, handler, "/germinate", map[string]any{"pid": 1, "cwd": "/a"})
	var reg map[string]string
	decodeJSON(t, w, &reg)

	w = postJSON(t, handler, "/wither", map[string]any{"peer_id": reg["peer_id"]})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	w = postJSON(t, handler, "/discover", map[string]any{"peer_id": reg["peer_id"], "scope": "machine"})
	var disc map[string][]map[string]any
	decodeJSON(t, w, &disc)
	if len(disc["peers"]) != 0 {
		t.Fatalf("expected 0 peers after wither, got %d", len(disc["peers"]))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/broker/ -run TestBroker`
Expected: FAIL — `broker.New` undefined

- [ ] **Step 3: Implement broker HTTP server**

```go
// internal/broker/broker.go
package broker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/closer/mycelium/internal/types"
)

type Broker struct {
	store *Store
}

func New() *Broker {
	return &Broker{store: NewStore()}
}

func (b *Broker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/germinate", b.handleGerminate)
	mux.HandleFunc("/pulse", b.handlePulse)
	mux.HandleFunc("/wither", b.handleWither)
	mux.HandleFunc("/identify", b.handleIdentify)
	mux.HandleFunc("/discover", b.handleDiscover)
	mux.HandleFunc("/signal", b.handleSignal)
	mux.HandleFunc("/sporulate", b.handleSporulate)
	mux.HandleFunc("/sense", b.handleSense)
	mux.HandleFunc("/attach", b.handleAttach)
	mux.HandleFunc("/detach", b.handleDetach)
	mux.HandleFunc("/nurture", b.handleNurture)
	mux.HandleFunc("/fruit", b.handleFruit)
	mux.HandleFunc("/survey", b.handleSurvey)
	return mux
}

func (b *Broker) StartCleanup(interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			b.store.CleanupStalePeers(timeout)
		}
	}()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (b *Broker) handleGerminate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PID     int    `json:"pid"`
		CWD     string `json:"cwd"`
		Repo    string `json:"repo"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	peer := types.NewPeer(req.PID, req.CWD, req.Repo, req.Summary)
	b.store.RegisterPeer(peer)
	writeJSON(w, map[string]string{"peer_id": peer.ID})
}

func (b *Broker) handlePulse(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.Pulse(req.PeerID)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleWither(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.UnregisterPeer(req.PeerID)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleIdentify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID  string `json:"peer_id"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.UpdateSummary(req.PeerID, req.Summary)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleDiscover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
		Scope  string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	peers := b.store.ListPeers(req.PeerID, req.Scope)
	if peers == nil {
		peers = []types.Peer{}
	}
	writeJSON(w, map[string][]types.Peer{"peers": peers})
}

func (b *Broker) handleSignal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	msg := types.NewMessage(req.From, req.To, req.Body, "")
	b.store.EnqueueMessage(msg)
	writeJSON(w, map[string]string{"message_id": msg.ID})
}

func (b *Broker) handleSporulate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From  string `json:"from"`
		Body  string `json:"body"`
		Topic string `json:"topic"`
		Scope string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.Broadcast(req.From, req.Body, req.Topic, req.Scope)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleSense(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	msgs := b.store.DrainMessages(req.PeerID)
	if msgs == nil {
		msgs = []types.Message{}
	}

	// Enrich with sender summary
	type enriched struct {
		types.Message
		FromSummary string `json:"from_summary"`
	}
	result := make([]enriched, len(msgs))
	for i, m := range msgs {
		summary := ""
		if p, ok := b.store.GetPeer(m.From); ok {
			summary = p.Summary
		}
		result[i] = enriched{Message: m, FromSummary: summary}
	}
	writeJSON(w, map[string][]enriched{"messages": result})
}

func (b *Broker) handleAttach(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
		Topic  string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.Subscribe(req.PeerID, req.Topic)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleDetach(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
		Topic  string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	b.store.Unsubscribe(req.PeerID, req.Topic)
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleNurture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From        string `json:"from"`
		To          string `json:"to"`
		Description string `json:"description"`
		Context     string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	task := types.NewTask(req.From, req.To, req.Description, req.Context)
	b.store.CreateTask(task)
	writeJSON(w, map[string]string{"task_id": task.ID})
}

func (b *Broker) handleFruit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID string           `json:"task_id"`
		Status types.TaskStatus `json:"status"`
		Result string           `json:"result"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if !b.store.CompleteTask(req.TaskID, req.Status, req.Result) {
		writeError(w, 404, fmt.Sprintf("task %s not found", req.TaskID))
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleSurvey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	delegated, received := b.store.ListTasks(req.PeerID)
	if delegated == nil {
		delegated = []types.Task{}
	}
	if received == nil {
		received = []types.Task{}
	}
	writeJSON(w, map[string][]types.Task{
		"delegated": delegated,
		"received":  received,
	})
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/broker/ -run TestBroker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/broker/broker.go internal/broker/broker_test.go
git commit -m "feat: add broker HTTP server with all endpoints"
```

---

### Task 5: Broker HTTP Client

**Files:**
- Create: `internal/client/client.go`
- Create: `internal/client/client_test.go`

- [ ] **Step 1: Write tests using a real broker**

```go
// internal/client/client_test.go
package client_test

import (
	"net/http/httptest"
	"testing"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
)

func setupTestBroker(t *testing.T) (*client.Client, func()) {
	t.Helper()
	b := broker.New()
	srv := httptest.NewServer(b.Handler())
	c := client.New(srv.URL)
	return c, srv.Close
}

func TestClient_GerminateAndDiscover(t *testing.T) {
	c, cleanup := setupTestBroker(t)
	defer cleanup()

	id1, err := c.Germinate(1, "/a", "repo-1", "peer 1")
	if err != nil {
		t.Fatalf("germinate failed: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected peer_id")
	}

	id2, err := c.Germinate(2, "/b", "repo-1", "peer 2")
	if err != nil {
		t.Fatalf("germinate failed: %v", err)
	}

	peers, err := c.Discover(id1, "machine")
	if err != nil {
		t.Fatalf("discover failed: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].ID != id2 {
		t.Fatalf("expected peer %s, got %s", id2, peers[0].ID)
	}
}

func TestClient_SignalAndSense(t *testing.T) {
	c, cleanup := setupTestBroker(t)
	defer cleanup()

	id1, _ := c.Germinate(1, "/a", "", "")
	id2, _ := c.Germinate(2, "/b", "", "")

	_, err := c.Signal(id1, id2, "hello from client")
	if err != nil {
		t.Fatalf("signal failed: %v", err)
	}

	msgs, err := c.Sense(id2)
	if err != nil {
		t.Fatalf("sense failed: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "hello from client" {
		t.Fatalf("expected body, got %s", msgs[0].Body)
	}
}

func TestClient_NurtureAndFruit(t *testing.T) {
	c, cleanup := setupTestBroker(t)
	defer cleanup()

	id1, _ := c.Germinate(1, "/a", "", "")
	id2, _ := c.Germinate(2, "/b", "", "")

	taskID, err := c.Nurture(id1, id2, "write tests", "for API")
	if err != nil {
		t.Fatalf("nurture failed: %v", err)
	}
	if taskID == "" {
		t.Fatal("expected task_id")
	}

	err = c.Fruit(taskID, "completed", "all tests pass")
	if err != nil {
		t.Fatalf("fruit failed: %v", err)
	}

	tasks, err := c.Survey(id1)
	if err != nil {
		t.Fatalf("survey failed: %v", err)
	}
	if len(tasks.Delegated) != 1 {
		t.Fatalf("expected 1 delegated task, got %d", len(tasks.Delegated))
	}
	if tasks.Delegated[0].Status != "completed" {
		t.Fatalf("expected completed, got %s", tasks.Delegated[0].Status)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/client/`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement client**

```go
// internal/client/client.go
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/closer/mycelium/internal/types"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

func (c *Client) post(path string, body any, result any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("request to %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("%s returned %d: %s", path, resp.StatusCode, errResp["error"])
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response from %s: %w", path, err)
		}
	}
	return nil
}

func (c *Client) Germinate(pid int, cwd, repo, summary string) (string, error) {
	var resp struct {
		PeerID string `json:"peer_id"`
	}
	err := c.post("/germinate", map[string]any{
		"pid": pid, "cwd": cwd, "repo": repo, "summary": summary,
	}, &resp)
	return resp.PeerID, err
}

func (c *Client) Pulse(peerID string) error {
	return c.post("/pulse", map[string]string{"peer_id": peerID}, nil)
}

func (c *Client) Wither(peerID string) error {
	return c.post("/wither", map[string]string{"peer_id": peerID}, nil)
}

func (c *Client) Identify(peerID, summary string) error {
	return c.post("/identify", map[string]any{
		"peer_id": peerID, "summary": summary,
	}, nil)
}

func (c *Client) Discover(peerID, scope string) ([]types.Peer, error) {
	var resp struct {
		Peers []types.Peer `json:"peers"`
	}
	err := c.post("/discover", map[string]any{
		"peer_id": peerID, "scope": scope,
	}, &resp)
	return resp.Peers, err
}

func (c *Client) Signal(from, to, body string) (string, error) {
	var resp struct {
		MessageID string `json:"message_id"`
	}
	err := c.post("/signal", map[string]any{
		"from": from, "to": to, "body": body,
	}, &resp)
	return resp.MessageID, err
}

func (c *Client) Sporulate(from, body, topic, scope string) error {
	return c.post("/sporulate", map[string]any{
		"from": from, "body": body, "topic": topic, "scope": scope,
	}, nil)
}

type EnrichedMessage struct {
	types.Message
	FromSummary string `json:"from_summary"`
}

func (c *Client) Sense(peerID string) ([]EnrichedMessage, error) {
	var resp struct {
		Messages []EnrichedMessage `json:"messages"`
	}
	err := c.post("/sense", map[string]string{"peer_id": peerID}, &resp)
	return resp.Messages, err
}

func (c *Client) Attach(peerID, topic string) error {
	return c.post("/attach", map[string]any{
		"peer_id": peerID, "topic": topic,
	}, nil)
}

func (c *Client) Detach(peerID, topic string) error {
	return c.post("/detach", map[string]any{
		"peer_id": peerID, "topic": topic,
	}, nil)
}

func (c *Client) Nurture(from, to, description, context string) (string, error) {
	var resp struct {
		TaskID string `json:"task_id"`
	}
	err := c.post("/nurture", map[string]any{
		"from": from, "to": to, "description": description, "context": context,
	}, &resp)
	return resp.TaskID, err
}

func (c *Client) Fruit(taskID string, status types.TaskStatus, result string) error {
	return c.post("/fruit", map[string]any{
		"task_id": taskID, "status": status, "result": result,
	}, nil)
}

type SurveyResult struct {
	Delegated []types.Task `json:"delegated"`
	Received  []types.Task `json:"received"`
}

func (c *Client) Survey(peerID string) (SurveyResult, error) {
	var resp SurveyResult
	err := c.post("/survey", map[string]string{"peer_id": peerID}, &resp)
	return resp, err
}

func (c *Client) IsAlive() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/pulse")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/client/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat: add broker HTTP client"
```

---

### Task 6: MCP Server with Tool Definitions

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`

- [ ] **Step 1: Write tests for auto-summary generation**

```go
// internal/mcp/server_test.go
package mcp_test

import (
	"testing"

	mcpserver "github.com/closer/mycelium/internal/mcp"
)

func TestGenerateSummary(t *testing.T) {
	// Test with a known git directory
	summary := mcpserver.GenerateSummary("/tmp")
	// Should at least contain the directory name
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/`
Expected: FAIL

- [ ] **Step 3: Implement MCP server**

```go
// internal/mcp/server.go
package mcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/closer/mycelium/internal/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	client *client.Client
	peerID string
}

func GenerateSummary(cwd string) string {
	dir := filepath.Base(cwd)

	branch := ""
	if out, err := exec.Command("git", "-C", cwd, "branch", "--show-current").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}

	commits := ""
	if out, err := exec.Command("git", "-C", cwd, "log", "--oneline", "-3").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		quoted := make([]string, 0, len(lines))
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				quoted = append(quoted, fmt.Sprintf("%q", l))
			}
		}
		commits = strings.Join(quoted, ", ")
	}

	if branch != "" && commits != "" {
		return fmt.Sprintf("%s (%s) — latest: %s", dir, branch, commits)
	}
	if branch != "" {
		return fmt.Sprintf("%s (%s)", dir, branch)
	}
	return dir
}

func NewServer(brokerURL string) *Server {
	return &Server{
		client: client.New(brokerURL),
	}
}

func (s *Server) Run() error {
	cwd, _ := os.Getwd()
	repo := ""
	if out, err := exec.Command("git", "-C", cwd, "remote", "get-url", "origin").Output(); err == nil {
		repo = strings.TrimSpace(string(out))
	}
	summary := GenerateSummary(cwd)

	// Ensure broker is running
	if !s.client.IsAlive() {
		if err := s.startBroker(); err != nil {
			return fmt.Errorf("failed to start broker: %w", err)
		}
	}

	// Register with broker
	peerID, err := s.client.Germinate(os.Getpid(), cwd, repo, summary)
	if err != nil {
		return fmt.Errorf("failed to germinate: %w", err)
	}
	s.peerID = peerID

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		s.client.Wither(s.peerID)
		cancel()
	}()

	// Start heartbeat
	go s.heartbeat(ctx)

	// Create and run MCP server
	mcpServer := server.NewMCPServer(
		"mycelium",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	s.registerTools(mcpServer)

	return server.ServeStdio(mcpServer)
}

func (s *Server) startBroker() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "broker")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	// Wait for broker to be ready
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if s.client.IsAlive() {
			return nil
		}
	}
	return fmt.Errorf("broker did not start within 2 seconds")
}

func (s *Server) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.client.Pulse(s.peerID); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
		}
	}
}

func (s *Server) registerTools(mcpServer *server.MCPServer) {
	// discover
	mcpServer.AddTool(
		gomcp.NewTool("discover",
			gomcp.WithDescription("Explore the mycelium network to find other active Claude Code sessions."),
			gomcp.WithString("scope",
				gomcp.Description("Discovery scope: 'machine' (all sessions) or 'repo' (same git repository)"),
				gomcp.Enum("machine", "repo"),
				gomcp.DefaultString("machine"),
			),
		),
		s.handleDiscover,
	)

	// signal
	mcpServer.AddTool(
		gomcp.NewTool("signal",
			gomcp.WithDescription("Send a direct message to a specific peer."),
			gomcp.WithString("to", gomcp.Required(), gomcp.Description("Target peer ID")),
			gomcp.WithString("body", gomcp.Required(), gomcp.Description("Message content")),
		),
		s.handleSignal,
	)

	// sporulate
	mcpServer.AddTool(
		gomcp.NewTool("sporulate",
			gomcp.WithDescription("Broadcast a message to all peers or topic subscribers within scope."),
			gomcp.WithString("body", gomcp.Required(), gomcp.Description("Message content")),
			gomcp.WithString("topic", gomcp.Description("Topic to broadcast to (omit for all peers)")),
			gomcp.WithString("scope",
				gomcp.Description("Broadcast scope: 'machine' or 'repo'"),
				gomcp.Enum("machine", "repo"),
				gomcp.DefaultString("machine"),
			),
		),
		s.handleSporulate,
	)

	// sense
	mcpServer.AddTool(
		gomcp.NewTool("sense",
			gomcp.WithDescription("Check for incoming messages and spores from other peers."),
		),
		s.handleSense,
	)

	// identify
	mcpServer.AddTool(
		gomcp.NewTool("identify",
			gomcp.WithDescription("Set a summary of your current work, visible to other peers."),
			gomcp.WithString("summary", gomcp.Required(), gomcp.Description("Summary of current work")),
		),
		s.handleIdentify,
	)

	// attach
	mcpServer.AddTool(
		gomcp.NewTool("attach",
			gomcp.WithDescription("Subscribe to a topic to receive broadcast messages on that topic."),
			gomcp.WithString("topic", gomcp.Required(), gomcp.Description("Topic name to subscribe to")),
		),
		s.handleAttach,
	)

	// detach
	mcpServer.AddTool(
		gomcp.NewTool("detach",
			gomcp.WithDescription("Unsubscribe from a topic."),
			gomcp.WithString("topic", gomcp.Required(), gomcp.Description("Topic name to unsubscribe from")),
		),
		s.handleDetach,
	)

	// nurture
	mcpServer.AddTool(
		gomcp.NewTool("nurture",
			gomcp.WithDescription("Delegate a task to another peer."),
			gomcp.WithString("to", gomcp.Required(), gomcp.Description("Target peer ID")),
			gomcp.WithString("description", gomcp.Required(), gomcp.Description("Task description")),
			gomcp.WithString("context", gomcp.Description("Additional context for the task")),
		),
		s.handleNurture,
	)

	// fruit
	mcpServer.AddTool(
		gomcp.NewTool("fruit",
			gomcp.WithDescription("Report the result of a delegated task."),
			gomcp.WithString("task_id", gomcp.Required(), gomcp.Description("Task ID to report on")),
			gomcp.WithString("status", gomcp.Required(),
				gomcp.Description("Task outcome"),
				gomcp.Enum("completed", "failed"),
			),
			gomcp.WithString("result", gomcp.Required(), gomcp.Description("Result details")),
		),
		s.handleFruit,
	)

	// survey
	mcpServer.AddTool(
		gomcp.NewTool("survey",
			gomcp.WithDescription("List all delegated and received tasks with their status."),
		),
		s.handleSurvey,
	)
}

func (s *Server) handleDiscover(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	scope := req.GetString("scope", "machine")
	peers, err := s.client.Discover(s.peerID, scope)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("discover failed: %v", err)), nil
	}
	b, _ := jsonMarshal(peers)
	return gomcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleSignal(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	to, err := req.RequireString("to")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	body, err := req.RequireString("body")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	msgID, err := s.client.Signal(s.peerID, to, body)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("signal failed: %v", err)), nil
	}
	return gomcp.NewToolResultText(fmt.Sprintf("Message sent (id: %s)", msgID)), nil
}

func (s *Server) handleSporulate(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	body, err := req.RequireString("body")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	topic := req.GetString("topic", "")
	scope := req.GetString("scope", "machine")
	if err := s.client.Sporulate(s.peerID, body, topic, scope); err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("sporulate failed: %v", err)), nil
	}
	return gomcp.NewToolResultText("Spores released"), nil
}

func (s *Server) handleSense(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	msgs, err := s.client.Sense(s.peerID)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("sense failed: %v", err)), nil
	}
	if len(msgs) == 0 {
		return gomcp.NewToolResultText("No new signals"), nil
	}
	b, _ := jsonMarshal(msgs)
	return gomcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleIdentify(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	summary, err := req.RequireString("summary")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	if err := s.client.Identify(s.peerID, summary); err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("identify failed: %v", err)), nil
	}
	return gomcp.NewToolResultText("Colony identified"), nil
}

func (s *Server) handleAttach(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	topic, err := req.RequireString("topic")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	if err := s.client.Attach(s.peerID, topic); err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("attach failed: %v", err)), nil
	}
	return gomcp.NewToolResultText(fmt.Sprintf("Attached to topic: %s", topic)), nil
}

func (s *Server) handleDetach(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	topic, err := req.RequireString("topic")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	if err := s.client.Detach(s.peerID, topic); err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("detach failed: %v", err)), nil
	}
	return gomcp.NewToolResultText(fmt.Sprintf("Detached from topic: %s", topic)), nil
}

func (s *Server) handleNurture(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	to, err := req.RequireString("to")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	desc, err := req.RequireString("description")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	taskCtx := req.GetString("context", "")
	taskID, err := s.client.Nurture(s.peerID, to, desc, taskCtx)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("nurture failed: %v", err)), nil
	}
	return gomcp.NewToolResultText(fmt.Sprintf("Task delegated (id: %s)", taskID)), nil
}

func (s *Server) handleFruit(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	taskID, err := req.RequireString("task_id")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	status, err := req.RequireString("status")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	result, err := req.RequireString("result")
	if err != nil {
		return gomcp.NewToolResultError(err.Error()), nil
	}
	if err := s.client.Fruit(taskID, types.TaskStatus(status), result); err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("fruit failed: %v", err)), nil
	}
	return gomcp.NewToolResultText("Fruit borne"), nil
}

func (s *Server) handleSurvey(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
	result, err := s.client.Survey(s.peerID)
	if err != nil {
		return gomcp.NewToolResultError(fmt.Sprintf("survey failed: %v", err)), nil
	}
	b, _ := jsonMarshal(result)
	return gomcp.NewToolResultText(string(b)), nil
}
```

Also add a helper at the bottom of the file:

```go
func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
```

The full import block for `internal/mcp/server.go` should be:

```go
import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/closer/mycelium/internal/client"
	"github.com/closer/mycelium/internal/types"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mcp/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/
git commit -m "feat: add MCP server with all tool definitions"
```

---

### Task 7: CLI Subcommands

**Files:**
- Modify: `cmd/mycelium/main.go`

- [ ] **Step 1: Add broker subcommand**

Add to `cmd/mycelium/main.go`:

```go
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
	mcpserver "github.com/closer/mycelium/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	brokerPort string
	brokerHost string
)

var rootCmd = &cobra.Command{
	Use:   "mycelium",
	Short: "Inter-session communication for Claude Code",
}

var brokerCmd = &cobra.Command{
	Use:   "broker",
	Short: "Start the broker daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		b := broker.New()
		b.StartCleanup(30*time.Second, 30*time.Second)
		addr := fmt.Sprintf("%s:%s", brokerHost, brokerPort)
		fmt.Fprintf(os.Stderr, "Broker listening on %s\n", addr)
		return http.ListenAndServe(addr, b.Handler())
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server (stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		s := mcpserver.NewServer(brokerURL)
		return s.Run()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show broker status and peer list",
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		c := client.New(brokerURL)
		if !c.IsAlive() {
			fmt.Println("Broker is not running")
			return nil
		}
		fmt.Println("Broker is running")
		// Register a temporary peer to list others
		id, err := c.Germinate(os.Getpid(), "/tmp/mycelium-status", "", "status check")
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer c.Wither(id)
		peers, err := c.Discover(id, "machine")
		if err != nil {
			return fmt.Errorf("failed to discover: %w", err)
		}
		if len(peers) == 0 {
			fmt.Println("No active peers")
			return nil
		}
		fmt.Printf("\nActive peers (%d):\n", len(peers))
		for _, p := range peers {
			fmt.Printf("  %s  %s  %s\n", p.ID[:8], p.CWD, p.Summary)
		}
		return nil
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <peer_id> <message>",
	Short: "Send a message to a peer (debug)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		c := client.New(brokerURL)
		id, err := c.Germinate(os.Getpid(), "/tmp/mycelium-cli", "", "CLI")
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer c.Wither(id)
		msgID, err := c.Signal(id, args[0], strings.Join(args[1:], " "))
		if err != nil {
			return fmt.Errorf("failed to send: %w", err)
		}
		fmt.Printf("Sent (id: %s)\n", msgID)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&brokerPort, "port", getEnvDefault("MYCELIUM_PORT", "7890"), "broker port")
	rootCmd.PersistentFlags().StringVar(&brokerHost, "host", getEnvDefault("MYCELIUM_HOST", "localhost"), "broker host")
	rootCmd.AddCommand(brokerCmd, serveCmd, statusCmd, sendCmd)
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

Add imports: `"strings"`, `"time"`.

- [ ] **Step 2: Verify it builds**

Run: `go build ./cmd/mycelium`
Expected: No errors

- [ ] **Step 3: Verify help output**

Run: `go run ./cmd/mycelium --help`
Expected: Shows `broker`, `serve`, `status`, `send` subcommands

- [ ] **Step 4: Commit**

```bash
git add cmd/mycelium/main.go
git commit -m "feat: add CLI subcommands (broker, serve, status, send)"
```

---

### Task 8: Integration Test

**Files:**
- Create: `integration_test.go`

- [ ] **Step 1: Write end-to-end integration test**

```go
// integration_test.go
package mycelium_test

import (
	"net/http/httptest"
	"testing"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
	"github.com/closer/mycelium/internal/types"
)

func TestIntegration_FullWorkflow(t *testing.T) {
	// Start broker
	b := broker.New()
	srv := httptest.NewServer(b.Handler())
	defer srv.Close()

	// Create two clients (simulating two MCP servers)
	c1 := client.New(srv.URL)
	c2 := client.New(srv.URL)

	// Germinate
	id1, err := c1.Germinate(1001, "/project/frontend", "git@github.com:user/frontend.git", "React frontend")
	if err != nil {
		t.Fatalf("c1 germinate: %v", err)
	}
	id2, err := c2.Germinate(1002, "/project/backend", "git@github.com:user/backend.git", "Go backend")
	if err != nil {
		t.Fatalf("c2 germinate: %v", err)
	}

	// Discover — machine scope should find each other
	peers, err := c1.Discover(id1, "machine")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(peers) != 1 || peers[0].ID != id2 {
		t.Fatalf("expected to find c2, got %v", peers)
	}

	// Discover — repo scope should NOT find each other (different repos)
	peers, err = c1.Discover(id1, "repo")
	if err != nil {
		t.Fatalf("discover repo: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers in repo scope, got %d", len(peers))
	}

	// Signal: c1 → c2
	_, err = c1.Signal(id1, id2, "API schema ready")
	if err != nil {
		t.Fatalf("signal: %v", err)
	}

	// Sense: c2 reads messages
	msgs, err := c2.Sense(id2)
	if err != nil {
		t.Fatalf("sense: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Body != "API schema ready" {
		t.Fatalf("expected message, got %v", msgs)
	}

	// Topic: c2 subscribes to "deploy"
	c2.Attach(id2, "deploy")

	// Sporulate to "deploy" topic
	c1.Sporulate(id1, "deploying v1.0", "deploy", "machine")

	msgs, err = c2.Sense(id2)
	if err != nil {
		t.Fatalf("sense after sporulate: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Topic != "deploy" {
		t.Fatalf("expected deploy topic message, got %v", msgs)
	}

	// Task delegation: c1 nurtures c2
	taskID, err := c1.Nurture(id1, id2, "implement /users endpoint", "REST API, return JSON")
	if err != nil {
		t.Fatalf("nurture: %v", err)
	}

	// c2 checks tasks
	tasks, err := c2.Survey(id2)
	if err != nil {
		t.Fatalf("survey: %v", err)
	}
	if len(tasks.Received) != 1 || tasks.Received[0].ID != taskID {
		t.Fatalf("expected received task, got %v", tasks)
	}

	// c2 completes task
	err = c2.Fruit(taskID, types.TaskStatusCompleted, "endpoint implemented with tests")
	if err != nil {
		t.Fatalf("fruit: %v", err)
	}

	// c1 should receive notification
	msgs, err = c1.Sense(id1)
	if err != nil {
		t.Fatalf("sense notification: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(msgs))
	}

	// Verify task status
	tasks, err = c1.Survey(id1)
	if err != nil {
		t.Fatalf("survey delegated: %v", err)
	}
	if len(tasks.Delegated) != 1 || tasks.Delegated[0].Status != types.TaskStatusCompleted {
		t.Fatalf("expected completed task, got %v", tasks.Delegated)
	}

	// Wither
	c1.Wither(id1)
	peers, err = c2.Discover(id2, "machine")
	if err != nil {
		t.Fatalf("discover after wither: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers after wither, got %d", len(peers))
	}
}
```

- [ ] **Step 2: Run integration test**

Run: `go test -v -run TestIntegration ./`
Expected: PASS

- [ ] **Step 3: Run all tests**

Run: `go test ./...`
Expected: All PASS

- [ ] **Step 4: Commit**

```bash
git add integration_test.go
git commit -m "test: add end-to-end integration test"
```

---

### Task 9: Final Wiring and Manual Test

**Files:**
- No new files

- [ ] **Step 1: Build the binary**

Run: `go build -o mycelium ./cmd/mycelium`
Expected: `mycelium` binary created

- [ ] **Step 2: Start broker in background**

Run: `./mycelium broker &`
Expected: "Broker listening on localhost:7890"

- [ ] **Step 3: Check status**

Run: `./mycelium status`
Expected: "Broker is running" + "No active peers"

- [ ] **Step 4: Kill broker**

Run: `kill %1`

- [ ] **Step 5: Add .gitignore**

```
mycelium
```

- [ ] **Step 6: Final commit**

```bash
git add .gitignore
git commit -m "chore: add .gitignore"
```
