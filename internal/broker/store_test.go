package broker

import (
	"testing"
	"time"

	"github.com/closer/mycelium/internal/types"
)

// --- Peer operations ---

func TestStore_RegisterAndGetPeer(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1234, "/home/user/project", "github.com/org/repo", "working on feature X")
	s.RegisterPeer(p)

	got, ok := s.GetPeer(p.ID)
	if !ok {
		t.Fatal("expected peer to be found")
	}
	if got.ID != p.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, p.ID)
	}
	if got.PID != 1234 {
		t.Errorf("PID mismatch: got %d, want %d", got.PID, 1234)
	}
	if got.Repo != "github.com/org/repo" {
		t.Errorf("Repo mismatch: got %s", got.Repo)
	}
	if got.Summary != "working on feature X" {
		t.Errorf("Summary mismatch: got %s", got.Summary)
	}
}

func TestStore_ListPeers_Machine(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo-a", "")
	p2 := types.NewPeer(2, "/b", "repo-b", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	peers := s.ListPeers(p1.ID, "machine")
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].ID != p2.ID {
		t.Errorf("expected p2, got %s", peers[0].ID)
	}
}

func TestStore_ListPeers_Repo(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo-same", "")
	p2 := types.NewPeer(2, "/b", "repo-same", "")
	p3 := types.NewPeer(3, "/c", "repo-other", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	peers := s.ListPeers(p1.ID, "repo")
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].ID != p2.ID {
		t.Errorf("expected p2, got %s", peers[0].ID)
	}
}

func TestStore_UnregisterPeer(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1, "/a", "repo", "")
	s.RegisterPeer(p)
	s.UnregisterPeer(p.ID)

	_, ok := s.GetPeer(p.ID)
	if ok {
		t.Fatal("expected peer to be removed")
	}
}

func TestStore_UpdateSummary(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1, "/a", "repo", "old summary")
	s.RegisterPeer(p)

	s.UpdateSummary(p.ID, "new summary")

	got, _ := s.GetPeer(p.ID)
	if got.Summary != "new summary" {
		t.Errorf("expected 'new summary', got %s", got.Summary)
	}
}

func TestStore_Pulse(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1, "/a", "repo", "")
	s.RegisterPeer(p)

	before := time.Now()
	time.Sleep(time.Millisecond)
	s.Pulse(p.ID)
	time.Sleep(time.Millisecond)
	after := time.Now()

	got, _ := s.GetPeer(p.ID)
	if got.LastPulse.Before(before) || got.LastPulse.After(after) {
		t.Errorf("LastPulse not updated correctly: %v", got.LastPulse)
	}
}

// --- Message/topic operations ---

func TestStore_SignalAndSense(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo", "")
	p2 := types.NewPeer(2, "/b", "repo", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	msg := types.NewMessage(p1.ID, p2.ID, "hello", "")
	s.EnqueueMessage(msg)

	msgs := s.DrainMessages(p2.ID)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "hello" {
		t.Errorf("body mismatch: got %s", msgs[0].Body)
	}
	if msgs[0].From != p1.ID {
		t.Errorf("from mismatch: got %s", msgs[0].From)
	}

	// drain again — should be empty
	msgs2 := s.DrainMessages(p2.ID)
	if len(msgs2) != 0 {
		t.Fatalf("expected empty after drain, got %d", len(msgs2))
	}
}

func TestStore_Sporulate_Machine(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo-a", "")
	p2 := types.NewPeer(2, "/b", "repo-b", "")
	p3 := types.NewPeer(3, "/c", "repo-c", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	s.Broadcast(p1.ID, "broadcast msg", "", "machine")

	m2 := s.DrainMessages(p2.ID)
	m3 := s.DrainMessages(p3.ID)
	m1 := s.DrainMessages(p1.ID)

	if len(m2) != 1 {
		t.Errorf("p2 expected 1 message, got %d", len(m2))
	}
	if len(m3) != 1 {
		t.Errorf("p3 expected 1 message, got %d", len(m3))
	}
	if len(m1) != 0 {
		t.Errorf("p1 (sender) expected 0 messages, got %d", len(m1))
	}
}

func TestStore_Sporulate_Topic(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo", "")
	p2 := types.NewPeer(2, "/b", "repo", "")
	p3 := types.NewPeer(3, "/c", "repo", "")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)
	s.RegisterPeer(p3)

	s.Subscribe(p2.ID, "deploy")

	s.Broadcast(p1.ID, "deploying", "deploy", "machine")

	m2 := s.DrainMessages(p2.ID)
	m3 := s.DrainMessages(p3.ID)

	if len(m2) != 1 {
		t.Errorf("p2 (subscribed) expected 1 message, got %d", len(m2))
	}
	if len(m3) != 0 {
		t.Errorf("p3 (not subscribed) expected 0 messages, got %d", len(m3))
	}
}

func TestStore_AttachDetach(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1, "/a", "repo", "")
	s.RegisterPeer(p)

	s.Subscribe(p.ID, "deploy")
	s.Subscribe(p.ID, "test")

	got, _ := s.GetPeer(p.ID)
	if len(got.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(got.Topics))
	}

	s.Unsubscribe(p.ID, "deploy")

	got2, _ := s.GetPeer(p.ID)
	if len(got2.Topics) != 1 {
		t.Errorf("expected 1 topic after unsubscribe, got %d", len(got2.Topics))
	}
	if got2.Topics[0] != "test" {
		t.Errorf("expected 'test' topic remaining, got %s", got2.Topics[0])
	}
}

// --- Task operations ---

func TestStore_NurtureAndFruit(t *testing.T) {
	s := NewStore()
	p1 := types.NewPeer(1, "/a", "repo", "delegator")
	p2 := types.NewPeer(2, "/b", "repo", "executor")
	s.RegisterPeer(p1)
	s.RegisterPeer(p2)

	task := types.NewTask(p1.ID, p2.ID, "run tests", "run all unit tests")
	s.CreateTask(task)

	// survey from delegator side
	delegated, received := s.ListTasks(p1.ID)
	if len(delegated) != 1 {
		t.Errorf("p1 delegated: expected 1, got %d", len(delegated))
	}
	if len(received) != 0 {
		t.Errorf("p1 received: expected 0, got %d", len(received))
	}

	// survey from executor side
	delegated2, received2 := s.ListTasks(p2.ID)
	if len(delegated2) != 0 {
		t.Errorf("p2 delegated: expected 0, got %d", len(delegated2))
	}
	if len(received2) != 1 {
		t.Errorf("p2 received: expected 1, got %d", len(received2))
	}

	// complete task
	ok := s.CompleteTask(task.ID, types.TaskStatusCompleted, "all tests passed")
	if !ok {
		t.Fatal("CompleteTask returned false")
	}

	got, exists := s.GetTask(task.ID)
	if !exists {
		t.Fatal("task not found after completion")
	}
	if got.Status != types.TaskStatusCompleted {
		t.Errorf("status mismatch: got %s", got.Status)
	}
	if got.Result != "all tests passed" {
		t.Errorf("result mismatch: got %s", got.Result)
	}

	// delegator should receive notification message
	msgs := s.DrainMessages(p1.ID)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 notification message, got %d", len(msgs))
	}
	expectedBody := `Task "run tests" completed: all tests passed`
	if msgs[0].Body != expectedBody {
		t.Errorf("notification body mismatch:\n  got:  %s\n  want: %s", msgs[0].Body, expectedBody)
	}
}

// --- Cleanup ---

func TestStore_CleanupStalePeers(t *testing.T) {
	s := NewStore()
	p := types.NewPeer(1, "/a", "repo", "")
	s.RegisterPeer(p)

	// set LastPulse to 60s ago
	s.SetLastPulse(p.ID, time.Now().Add(-60*time.Second))

	removed := s.CleanupStalePeers(30 * time.Second)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	_, ok := s.GetPeer(p.ID)
	if ok {
		t.Fatal("expected peer to be removed after cleanup")
	}
}
