package client_test

import (
	"net/http/httptest"
	"testing"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
	"github.com/closer/mycelium/internal/types"
)

func setupTestBroker(t *testing.T) (*client.Client, func()) {
	t.Helper()
	b := broker.New()
	srv := httptest.NewServer(b.Handler())
	c := client.New(srv.URL)
	return c, srv.Close
}

func TestClient_GerminateAndDiscover(t *testing.T) {
	c, teardown := setupTestBroker(t)
	defer teardown()

	p1, err := c.Germinate(1, "/tmp/p1", "repo1", "peer one")
	if err != nil {
		t.Fatalf("germinate p1: %v", err)
	}
	p2, err := c.Germinate(2, "/tmp/p2", "repo2", "peer two")
	if err != nil {
		t.Fatalf("germinate p2: %v", err)
	}

	peers, err := c.Discover(p1, "")
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	found := false
	for _, p := range peers {
		if p.ID == p2 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected p2 (%s) in discover results, got %+v", p2, peers)
	}
}

func TestClient_SignalAndSense(t *testing.T) {
	c, teardown := setupTestBroker(t)
	defer teardown()

	p1, err := c.Germinate(1, "/tmp/p1", "", "sender")
	if err != nil {
		t.Fatalf("germinate p1: %v", err)
	}
	p2, err := c.Germinate(2, "/tmp/p2", "", "receiver")
	if err != nil {
		t.Fatalf("germinate p2: %v", err)
	}

	want := "hello from client"
	_, err = c.Signal(p1, p2, want)
	if err != nil {
		t.Fatalf("signal: %v", err)
	}

	msgs, err := c.Sense(p2)
	if err != nil {
		t.Fatalf("sense: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Body != want {
		t.Errorf("expected body %q, got %q", want, msgs[0].Body)
	}
}

func TestClient_NurtureAndFruit(t *testing.T) {
	c, teardown := setupTestBroker(t)
	defer teardown()

	p1, err := c.Germinate(1, "/tmp/p1", "", "manager")
	if err != nil {
		t.Fatalf("germinate p1: %v", err)
	}
	p2, err := c.Germinate(2, "/tmp/p2", "", "worker")
	if err != nil {
		t.Fatalf("germinate p2: %v", err)
	}

	taskID, err := c.Nurture(p1, p2, "write tests", "")
	if err != nil {
		t.Fatalf("nurture: %v", err)
	}

	if err := c.Fruit(taskID, types.TaskStatusCompleted, "done"); err != nil {
		t.Fatalf("fruit: %v", err)
	}

	survey, err := c.Survey(p1)
	if err != nil {
		t.Fatalf("survey: %v", err)
	}

	if len(survey.Delegated) != 1 {
		t.Fatalf("expected 1 delegated task, got %d", len(survey.Delegated))
	}
	if survey.Delegated[0].Status != types.TaskStatusCompleted {
		t.Errorf("expected status completed, got %s", survey.Delegated[0].Status)
	}
}
