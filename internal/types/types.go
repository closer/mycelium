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
