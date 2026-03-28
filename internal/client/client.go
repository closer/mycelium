package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/closer/mycelium/internal/types"
)

// Client is an HTTP client for the mycelium broker.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new Client targeting the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// post marshals body to JSON, POSTs to baseURL+path, checks status 200,
// and decodes the response into result if non-nil.
func (c *Client) post(path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e) //nolint:errcheck
		if e.Error != "" {
			return fmt.Errorf("broker error: %s", e.Error)
		}
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}
	return nil
}

// EnrichedMessage is a Message with the sender's summary attached.
type EnrichedMessage struct {
	types.Message
	FromSummary string `json:"from_summary"`
}

// SurveyResult holds tasks delegated by and received by a peer.
type SurveyResult struct {
	Delegated []types.Task `json:"delegated"`
	Received  []types.Task `json:"received"`
}

// Germinate registers a new peer and returns its peer_id.
func (c *Client) Germinate(pid int, cwd, repo, summary string) (string, error) {
	req := struct {
		PID     int    `json:"pid"`
		CWD     string `json:"cwd"`
		Repo    string `json:"repo"`
		Summary string `json:"summary"`
	}{PID: pid, CWD: cwd, Repo: repo, Summary: summary}
	var resp struct {
		PeerID string `json:"peer_id"`
	}
	if err := c.post("/germinate", req, &resp); err != nil {
		return "", err
	}
	return resp.PeerID, nil
}

// Pulse sends a heartbeat for the given peer.
func (c *Client) Pulse(peerID string) error {
	return c.post("/pulse", map[string]string{"peer_id": peerID}, nil)
}

// Wither unregisters the given peer.
func (c *Client) Wither(peerID string) error {
	return c.post("/wither", map[string]string{"peer_id": peerID}, nil)
}

// Identify updates the summary for the given peer.
func (c *Client) Identify(peerID, summary string) error {
	return c.post("/identify", map[string]string{"peer_id": peerID, "summary": summary}, nil)
}

// Discover returns all visible peers for peerID within the given scope.
func (c *Client) Discover(peerID, scope string) ([]types.Peer, error) {
	req := map[string]string{"peer_id": peerID, "scope": scope}
	var resp struct {
		Peers []types.Peer `json:"peers"`
	}
	if err := c.post("/discover", req, &resp); err != nil {
		return nil, err
	}
	return resp.Peers, nil
}

// Signal sends a direct message from one peer to another and returns the message_id.
func (c *Client) Signal(from, to, body string) (string, error) {
	req := map[string]string{"from": from, "to": to, "body": body}
	var resp struct {
		MessageID string `json:"message_id"`
	}
	if err := c.post("/signal", req, &resp); err != nil {
		return "", err
	}
	return resp.MessageID, nil
}

// Sporulate broadcasts a message to all subscribers of the given topic.
func (c *Client) Sporulate(from, body, topic, scope string) error {
	req := map[string]string{"from": from, "body": body, "topic": topic, "scope": scope}
	return c.post("/sporulate", req, nil)
}

// Sense drains and returns queued messages for the given peer.
func (c *Client) Sense(peerID string) ([]EnrichedMessage, error) {
	var resp struct {
		Messages []EnrichedMessage `json:"messages"`
	}
	if err := c.post("/sense", map[string]string{"peer_id": peerID}, &resp); err != nil {
		return nil, err
	}
	return resp.Messages, nil
}

// Attach subscribes the peer to the given topic.
func (c *Client) Attach(peerID, topic string) error {
	return c.post("/attach", map[string]string{"peer_id": peerID, "topic": topic}, nil)
}

// Detach unsubscribes the peer from the given topic.
func (c *Client) Detach(peerID, topic string) error {
	return c.post("/detach", map[string]string{"peer_id": peerID, "topic": topic}, nil)
}

// Nurture creates a task delegated from one peer to another and returns the task_id.
func (c *Client) Nurture(from, to, description, context string) (string, error) {
	req := map[string]string{
		"from":        from,
		"to":          to,
		"description": description,
		"context":     context,
	}
	var resp struct {
		TaskID string `json:"task_id"`
	}
	if err := c.post("/nurture", req, &resp); err != nil {
		return "", err
	}
	return resp.TaskID, nil
}

// Fruit marks a task as completed or failed.
func (c *Client) Fruit(taskID string, status types.TaskStatus, result string) error {
	req := struct {
		TaskID string           `json:"task_id"`
		Status types.TaskStatus `json:"status"`
		Result string           `json:"result"`
	}{TaskID: taskID, Status: status, Result: result}
	return c.post("/fruit", req, nil)
}

// Survey returns all tasks delegated by and received by the given peer.
func (c *Client) Survey(peerID string) (SurveyResult, error) {
	var result SurveyResult
	if err := c.post("/survey", map[string]string{"peer_id": peerID}, &result); err != nil {
		return SurveyResult{}, err
	}
	return result, nil
}

// IsAlive returns true if the broker is reachable.
func (c *Client) IsAlive() bool {
	resp, err := c.httpClient.Get(c.baseURL + "/pulse")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}
