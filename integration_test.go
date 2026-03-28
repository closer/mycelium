package mycelium_test

import (
	"net/http/httptest"
	"testing"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
	"github.com/closer/mycelium/internal/types"
)

func TestIntegration_FullWorkflow(t *testing.T) {
	// 1. Start broker with httptest.NewServer
	b := broker.New()
	srv := httptest.NewServer(b.Handler())
	defer srv.Close()

	// 2. Create 2 clients: c1 (frontend, repo A) and c2 (backend, repo B)
	c1 := client.New(srv.URL)
	c2 := client.New(srv.URL)

	// 3. Germinate both
	id1, err := c1.Germinate(1001, "/workspace/frontend", "repo-a", "frontend Claude instance")
	if err != nil {
		t.Fatalf("c1 germinate: %v", err)
	}
	id2, err := c2.Germinate(1002, "/workspace/backend", "repo-b", "backend Claude instance")
	if err != nil {
		t.Fatalf("c2 germinate: %v", err)
	}

	// 4. Discover machine scope — should find each other
	peers, err := c1.Discover(id1, "machine")
	if err != nil {
		t.Fatalf("c1 discover machine: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("c1 discover machine: want 1 peer, got %d", len(peers))
	}
	if peers[0].ID != id2 {
		t.Errorf("c1 discover machine: expected c2 peer_id %s, got %s", id2, peers[0].ID)
	}

	// 5. Discover repo scope — should NOT find each other (different repos)
	repoPeers, err := c1.Discover(id1, "repo")
	if err != nil {
		t.Fatalf("c1 discover repo: %v", err)
	}
	if len(repoPeers) != 0 {
		t.Fatalf("c1 discover repo: want 0 peers (different repos), got %d", len(repoPeers))
	}

	// 6. Signal c1→c2 "API schema ready", sense c2, verify message
	_, err = c1.Signal(id1, id2, "API schema ready")
	if err != nil {
		t.Fatalf("c1 signal: %v", err)
	}
	msgs, err := c2.Sense(id2)
	if err != nil {
		t.Fatalf("c2 sense after signal: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("c2 sense: want 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != "API schema ready" {
		t.Errorf("c2 sense: want body %q, got %q", "API schema ready", msgs[0].Body)
	}
	if msgs[0].From != id1 {
		t.Errorf("c2 sense: want from %s, got %s", id1, msgs[0].From)
	}

	// 7. c2 subscribes to "deploy" topic
	if err := c2.Attach(id2, "deploy"); err != nil {
		t.Fatalf("c2 attach: %v", err)
	}

	// 8. c1 sporulates "deploying v1.0" on "deploy" topic, machine scope
	if err := c1.Sporulate(id1, "deploying v1.0", "deploy", "machine"); err != nil {
		t.Fatalf("c1 sporulate: %v", err)
	}

	// 9. c2 senses, verify topic message received
	topicMsgs, err := c2.Sense(id2)
	if err != nil {
		t.Fatalf("c2 sense after sporulate: %v", err)
	}
	if len(topicMsgs) != 1 {
		t.Fatalf("c2 sense topic: want 1 message, got %d", len(topicMsgs))
	}
	if topicMsgs[0].Body != "deploying v1.0" {
		t.Errorf("c2 sense topic: want body %q, got %q", "deploying v1.0", topicMsgs[0].Body)
	}
	if topicMsgs[0].Topic != "deploy" {
		t.Errorf("c2 sense topic: want topic %q, got %q", "deploy", topicMsgs[0].Topic)
	}

	// 10. c1 nurtures c2 "implement /users endpoint" with context "REST API, return JSON"
	taskID, err := c1.Nurture(id1, id2, "implement /users endpoint", "REST API, return JSON")
	if err != nil {
		t.Fatalf("c1 nurture: %v", err)
	}
	if taskID == "" {
		t.Fatal("c1 nurture: got empty task_id")
	}

	// 11. c2 surveys, verify received task
	survey2, err := c2.Survey(id2)
	if err != nil {
		t.Fatalf("c2 survey: %v", err)
	}
	if len(survey2.Received) != 1 {
		t.Fatalf("c2 survey received: want 1, got %d", len(survey2.Received))
	}
	task := survey2.Received[0]
	if task.ID != taskID {
		t.Errorf("c2 survey: want task_id %s, got %s", taskID, task.ID)
	}
	if task.Description != "implement /users endpoint" {
		t.Errorf("c2 survey: want description %q, got %q", "implement /users endpoint", task.Description)
	}
	if task.Status != types.TaskStatusPending {
		t.Errorf("c2 survey: want status pending, got %s", task.Status)
	}

	// 12. c2 fruits the task as completed "endpoint implemented with tests"
	if err := c2.Fruit(taskID, types.TaskStatusCompleted, "endpoint implemented with tests"); err != nil {
		t.Fatalf("c2 fruit: %v", err)
	}

	// 13. c1 senses, verify notification message
	notifMsgs, err := c1.Sense(id1)
	if err != nil {
		t.Fatalf("c1 sense after fruit: %v", err)
	}
	if len(notifMsgs) != 1 {
		t.Fatalf("c1 sense notification: want 1 message, got %d", len(notifMsgs))
	}
	if notifMsgs[0].From != id2 {
		t.Errorf("c1 sense notification: want from %s, got %s", id2, notifMsgs[0].From)
	}

	// 14. c1 surveys, verify task status is completed
	survey1, err := c1.Survey(id1)
	if err != nil {
		t.Fatalf("c1 survey: %v", err)
	}
	if len(survey1.Delegated) != 1 {
		t.Fatalf("c1 survey delegated: want 1, got %d", len(survey1.Delegated))
	}
	completedTask := survey1.Delegated[0]
	if completedTask.Status != types.TaskStatusCompleted {
		t.Errorf("c1 survey: want status completed, got %s", completedTask.Status)
	}
	if completedTask.Result != "endpoint implemented with tests" {
		t.Errorf("c1 survey: want result %q, got %q", "endpoint implemented with tests", completedTask.Result)
	}

	// 15. c1 withers
	if err := c1.Wither(id1); err != nil {
		t.Fatalf("c1 wither: %v", err)
	}

	// 16. c2 discovers machine scope, verify 0 peers
	peersAfter, err := c2.Discover(id2, "machine")
	if err != nil {
		t.Fatalf("c2 discover after wither: %v", err)
	}
	if len(peersAfter) != 0 {
		t.Fatalf("c2 discover after wither: want 0 peers, got %d", len(peersAfter))
	}
}
