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
