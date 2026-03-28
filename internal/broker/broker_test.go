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
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestBroker_GerminateAndDiscover(t *testing.T) {
	b := broker.New()
	h := b.Handler()

	// register peer 1
	w1 := postJSON(t, h, "/germinate", map[string]any{"pid": 1, "cwd": "/tmp/p1"})
	if w1.Code != http.StatusOK {
		t.Fatalf("germinate p1: status %d", w1.Code)
	}
	var r1 struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w1, &r1)

	// register peer 2
	w2 := postJSON(t, h, "/germinate", map[string]any{"pid": 2, "cwd": "/tmp/p2"})
	if w2.Code != http.StatusOK {
		t.Fatalf("germinate p2: status %d", w2.Code)
	}
	var r2 struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w2, &r2)

	// discover from p1 with scope "machine"
	wd := postJSON(t, h, "/discover", map[string]any{"peer_id": r1.PeerID, "scope": "machine"})
	if wd.Code != http.StatusOK {
		t.Fatalf("discover: status %d", wd.Code)
	}
	var dr struct {
		Peers []any `json:"peers"`
	}
	decodeJSON(t, wd, &dr)

	if len(dr.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(dr.Peers))
	}
}

func TestBroker_SignalAndSense(t *testing.T) {
	b := broker.New()
	h := b.Handler()

	// register two peers
	w1 := postJSON(t, h, "/germinate", map[string]any{"pid": 10, "cwd": "/tmp/p1"})
	var r1 struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w1, &r1)

	w2 := postJSON(t, h, "/germinate", map[string]any{"pid": 20, "cwd": "/tmp/p2"})
	var r2 struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w2, &r2)

	// signal p1 -> p2
	ws := postJSON(t, h, "/signal", map[string]any{"from": r1.PeerID, "to": r2.PeerID, "body": "hello"})
	if ws.Code != http.StatusOK {
		t.Fatalf("signal: status %d", ws.Code)
	}

	// sense from p2
	wse := postJSON(t, h, "/sense", map[string]any{"peer_id": r2.PeerID})
	if wse.Code != http.StatusOK {
		t.Fatalf("sense: status %d", wse.Code)
	}
	var sr struct {
		Messages []struct {
			Body string `json:"body"`
		} `json:"messages"`
	}
	decodeJSON(t, wse, &sr)

	if len(sr.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sr.Messages))
	}
	if sr.Messages[0].Body != "hello" {
		t.Fatalf("expected body 'hello', got %q", sr.Messages[0].Body)
	}
}

func TestBroker_WitherCleanup(t *testing.T) {
	b := broker.New()
	h := b.Handler()

	// register a peer
	w := postJSON(t, h, "/germinate", map[string]any{"pid": 99, "cwd": "/tmp/pw"})
	var r struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w, &r)

	// wither the peer
	ww := postJSON(t, h, "/wither", map[string]any{"peer_id": r.PeerID})
	if ww.Code != http.StatusOK {
		t.Fatalf("wither: status %d", ww.Code)
	}

	// discover with a different peer — but since there are no peers left, use a fake ID
	// register a second peer to do the discovery
	w2 := postJSON(t, h, "/germinate", map[string]any{"pid": 100, "cwd": "/tmp/pw2"})
	var r2 struct {
		PeerID string `json:"peer_id"`
	}
	decodeJSON(t, w2, &r2)

	wd := postJSON(t, h, "/discover", map[string]any{"peer_id": r2.PeerID, "scope": "machine"})
	if wd.Code != http.StatusOK {
		t.Fatalf("discover: status %d", wd.Code)
	}
	var dr struct {
		Peers []any `json:"peers"`
	}
	decodeJSON(t, wd, &dr)

	if len(dr.Peers) != 0 {
		t.Fatalf("expected 0 peers after wither, got %d", len(dr.Peers))
	}
}
