package broker

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/closer/mycelium/internal/types"
)

// Broker wraps Store and exposes HTTP handlers.
type Broker struct {
	store *Store
}

// New creates a Broker with a fresh Store.
func New() *Broker {
	return &Broker{store: NewStore()}
}

// Handler returns an http.ServeMux with all routes registered.
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

// StartCleanup starts a goroutine that periodically removes stale peers.
func (b *Broker) StartCleanup(interval, timeout time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			b.store.CleanupStalePeers(timeout)
		}
	}()
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- handlers ---

func (b *Broker) handleGerminate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PID     int    `json:"pid"`
		CWD     string `json:"cwd"`
		Repo    string `json:"repo"`
		Summary string `json:"summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	peers := b.store.ListPeers(req.PeerID, req.Scope)
	if peers == nil {
		peers = []types.Peer{}
	}
	writeJSON(w, map[string]any{"peers": peers})
}

func (b *Broker) handleSignal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	type enriched struct {
		types.Message
		FromSummary string `json:"from_summary"`
	}

	msgs := b.store.DrainMessages(req.PeerID)
	result := make([]enriched, 0, len(msgs))
	for _, m := range msgs {
		var summary string
		if p, ok := b.store.GetPeer(m.From); ok {
			summary = p.Summary
		}
		result = append(result, enriched{Message: m, FromSummary: summary})
	}
	writeJSON(w, map[string]any{"messages": result})
}

func (b *Broker) handleAttach(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
		Topic  string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
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
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !b.store.CompleteTask(req.TaskID, req.Status, req.Result) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (b *Broker) handleSurvey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PeerID string `json:"peer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	delegated, received := b.store.ListTasks(req.PeerID)
	if delegated == nil {
		delegated = []types.Task{}
	}
	if received == nil {
		received = []types.Task{}
	}
	writeJSON(w, map[string]any{"delegated": delegated, "received": received})
}
