package broker

import (
	"fmt"
	"sync"
	"time"

	"github.com/closer/mycelium/internal/types"
)

// Store holds all in-memory state for peers, messages, and tasks.
type Store struct {
	mu       sync.RWMutex
	peers    map[string]types.Peer
	messages map[string][]types.Message // keyed by recipient peer ID
	tasks    map[string]types.Task
}

// NewStore creates and returns an empty Store.
func NewStore() *Store {
	return &Store{
		peers:    make(map[string]types.Peer),
		messages: make(map[string][]types.Message),
		tasks:    make(map[string]types.Task),
	}
}

// --- Peer operations ---

// RegisterPeer adds or replaces a peer.
func (s *Store) RegisterPeer(p types.Peer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.peers[p.ID] = p
}

// GetPeer returns the peer with the given ID.
func (s *Store) GetPeer(id string) (types.Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[id]
	return p, ok
}

// UnregisterPeer removes the peer and cleans up its messages and tasks.
func (s *Store) UnregisterPeer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, id)
	delete(s.messages, id)
	// clean up tasks where this peer is from or to
	for tid, task := range s.tasks {
		if task.From == id || task.To == id {
			delete(s.tasks, tid)
		}
	}
}

// ListPeers returns peers visible from selfID under the given scope.
// scope "machine" — all peers except self.
// scope "repo"    — peers with the same Repo as self, except self.
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
			if p.Repo == self.Repo {
				result = append(result, p)
			}
		default: // "machine"
			result = append(result, p)
		}
	}
	return result
}

// UpdateSummary updates the Summary field of the given peer.
func (s *Store) UpdateSummary(id, summary string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.Summary = summary
		s.peers[id] = p
	}
}

// Pulse updates LastPulse to now for the given peer.
func (s *Store) Pulse(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.LastPulse = time.Now()
		s.peers[id] = p
	}
}

// --- Message / topic operations ---

// EnqueueMessage appends msg to the recipient's queue.
func (s *Store) EnqueueMessage(msg types.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[msg.To] = append(s.messages[msg.To], msg)
}

// DrainMessages returns and removes all queued messages for peerID.
func (s *Store) DrainMessages(peerID string) []types.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.messages[peerID]
	delete(s.messages, peerID)
	return msgs
}

// Broadcast sends body to all peers in scope (or topic subscribers if topic is set).
// The sender (fromID) is excluded.
func (s *Store) Broadcast(fromID, body, topic, scope string) {
	targets := s.ListPeers(fromID, scope)
	for _, p := range targets {
		if topic != "" && !containsString(p.Topics, topic) {
			continue
		}
		msg := types.NewMessage(fromID, p.ID, body, topic)
		s.EnqueueMessage(msg)
	}
}

// Subscribe adds a topic to the peer's Topics list.
func (s *Store) Subscribe(peerID, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[peerID]
	if !ok {
		return
	}
	if !containsString(p.Topics, topic) {
		p.Topics = append(p.Topics, topic)
		s.peers[peerID] = p
	}
}

// Unsubscribe removes a topic from the peer's Topics list.
func (s *Store) Unsubscribe(peerID, topic string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[peerID]
	if !ok {
		return
	}
	p.Topics = removeString(p.Topics, topic)
	s.peers[peerID] = p
}

// --- Task operations ---

// CreateTask stores a new task.
func (s *Store) CreateTask(task types.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

// GetTask returns the task with the given ID.
func (s *Store) GetTask(id string) (types.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

// CompleteTask marks the task as done and enqueues a notification to the delegator.
// Returns false if the task is not found.
func (s *Store) CompleteTask(taskID string, status types.TaskStatus, result string) bool {
	s.mu.Lock()
	task, ok := s.tasks[taskID]
	if !ok {
		s.mu.Unlock()
		return false
	}
	task.Status = status
	task.Result = result
	s.tasks[taskID] = task
	delegatorID := task.From
	description := task.Description
	s.mu.Unlock()

	body := fmt.Sprintf("Task %q %s: %s", description, string(status), result)
	msg := types.NewMessage(task.To, delegatorID, body, "")
	s.EnqueueMessage(msg)
	return true
}

// ListTasks returns (delegated, received) task slices for the given peer.
func (s *Store) ListTasks(peerID string) ([]types.Task, []types.Task) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var delegated, received []types.Task
	for _, t := range s.tasks {
		switch {
		case t.From == peerID:
			delegated = append(delegated, t)
		case t.To == peerID:
			received = append(received, t)
		}
	}
	return delegated, received
}

// --- Cleanup ---

// SetLastPulse sets the LastPulse time for a peer (used in testing).
func (s *Store) SetLastPulse(id string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.peers[id]; ok {
		p.LastPulse = t
		s.peers[id] = p
	}
}

// CleanupStalePeers removes peers whose LastPulse is older than timeout.
// Also cleans up their messages and tasks. Returns the number removed.
func (s *Store) CleanupStalePeers(timeout time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	deadline := time.Now().Add(-timeout)
	count := 0
	for id, p := range s.peers {
		if p.LastPulse.Before(deadline) {
			delete(s.peers, id)
			delete(s.messages, id)
			for tid, task := range s.tasks {
				if task.From == id || task.To == id {
					delete(s.tasks, tid)
				}
			}
			count++
		}
	}
	return count
}

// --- Helpers ---

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := slice[:0:0]
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}
