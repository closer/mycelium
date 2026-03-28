package mcp

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

// GenerateSummary builds a human-readable summary of the current working directory
// using the git branch and recent commits.
func GenerateSummary(cwd string) string {
	dir := filepath.Base(cwd)

	branchOut, err := exec.Command("git", "-C", cwd, "branch", "--show-current").Output()
	branch := ""
	if err == nil {
		branch = strings.TrimSpace(string(branchOut))
	}

	commits := ""
	logOut, err := exec.Command("git", "-C", cwd, "log", "--oneline", "-3").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(logOut)), "\n")
		quoted := make([]string, 0, len(lines))
		for _, l := range lines {
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

// Server wraps the MCP server and holds broker client state.
type Server struct {
	client *client.Client
	peerID string
}

// NewServer creates a new Server targeting the given broker URL.
func NewServer(brokerURL string) *Server {
	return &Server{
		client: client.New(brokerURL),
	}
}

// Run is the main entry point that starts the MCP stdio server.
func (s *Server) Run() error {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	repoOut, err := exec.Command("git", "-C", cwd, "remote", "get-url", "origin").Output()
	repo := ""
	if err == nil {
		repo = strings.TrimSpace(string(repoOut))
	}

	summary := GenerateSummary(cwd)

	if !s.client.IsAlive() {
		if err := s.startBroker(); err != nil {
			return fmt.Errorf("broker unavailable and could not start: %w", err)
		}
	}

	peerID, err := s.client.Germinate(os.Getpid(), cwd, repo, summary)
	if err != nil {
		return fmt.Errorf("germinate: %w", err)
	}
	s.peerID = peerID

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if s.peerID != "" {
			if err := s.client.Wither(s.peerID); err != nil {
				log.Printf("wither error: %v", err)
			}
		}
		cancel()
	}()

	go s.heartbeat(ctx)

	mcpServer := server.NewMCPServer("mycelium", "0.1.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	s.registerTools(mcpServer)

	return server.ServeStdio(mcpServer)
}

// startBroker launches the broker as a detached background process and waits
// up to 2 seconds for it to become ready.
func (s *Server) startBroker() error {
	exe, _ := os.Executable()
	cmd := exec.Command(exe, "broker")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if s.client.IsAlive() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("broker did not become ready within 2s")
}

// heartbeat sends periodic pulses to the broker.
func (s *Server) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.client.Pulse(s.peerID); err != nil {
				log.Printf("pulse error: %v", err)
			}
		}
	}
}

// registerTools adds all MCP tool definitions to the server.
func (s *Server) registerTools(mcpServer *server.MCPServer) {
	// discover
	discoverTool := gomcp.NewTool("discover",
		gomcp.WithDescription("Discover other active peers in the mycelium network"),
		gomcp.WithString("scope",
			gomcp.Description("Scope of discovery"),
			gomcp.Enum("machine", "repo"),
			gomcp.DefaultString("machine"),
		),
	)
	mcpServer.AddTool(discoverTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		scope := req.GetString("scope", "machine")
		peers, err := s.client.Discover(s.peerID, scope)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		data, err := jsonMarshal(peers)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(string(data)), nil
	})

	// signal
	signalTool := gomcp.NewTool("signal",
		gomcp.WithDescription("Send a direct message to another peer"),
		gomcp.WithString("to",
			gomcp.Description("Target peer ID"),
			gomcp.Required(),
		),
		gomcp.WithString("body",
			gomcp.Description("Message body"),
			gomcp.Required(),
		),
	)
	mcpServer.AddTool(signalTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
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
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(fmt.Sprintf("Message sent (id: %s)", msgID)), nil
	})

	// sporulate
	sporulateTool := gomcp.NewTool("sporulate",
		gomcp.WithDescription("Broadcast a message to all subscribers of a topic"),
		gomcp.WithString("body",
			gomcp.Description("Message body"),
			gomcp.Required(),
		),
		gomcp.WithString("topic",
			gomcp.Description("Topic to broadcast to"),
		),
		gomcp.WithString("scope",
			gomcp.Description("Scope of broadcast"),
			gomcp.Enum("machine", "repo"),
			gomcp.DefaultString("machine"),
		),
	)
	mcpServer.AddTool(sporulateTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		body, err := req.RequireString("body")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		topic := req.GetString("topic", "")
		scope := req.GetString("scope", "machine")
		if err := s.client.Sporulate(s.peerID, body, topic, scope); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText("Spores released"), nil
	})

	// sense
	senseTool := gomcp.NewTool("sense",
		gomcp.WithDescription("Drain and return queued messages for this peer"),
	)
	mcpServer.AddTool(senseTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		msgs, err := s.client.Sense(s.peerID)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if len(msgs) == 0 {
			return gomcp.NewToolResultText("No new signals"), nil
		}
		data, err := jsonMarshal(msgs)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(string(data)), nil
	})

	// identify
	identifyTool := gomcp.NewTool("identify",
		gomcp.WithDescription("Update the summary for this peer"),
		gomcp.WithString("summary",
			gomcp.Description("New summary string"),
			gomcp.Required(),
		),
	)
	mcpServer.AddTool(identifyTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		summary, err := req.RequireString("summary")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.Identify(s.peerID, summary); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText("Colony identified"), nil
	})

	// attach
	attachTool := gomcp.NewTool("attach",
		gomcp.WithDescription("Subscribe this peer to a topic"),
		gomcp.WithString("topic",
			gomcp.Description("Topic to subscribe to"),
			gomcp.Required(),
		),
	)
	mcpServer.AddTool(attachTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		topic, err := req.RequireString("topic")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.Attach(s.peerID, topic); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(fmt.Sprintf("Attached to topic: %s", topic)), nil
	})

	// detach
	detachTool := gomcp.NewTool("detach",
		gomcp.WithDescription("Unsubscribe this peer from a topic"),
		gomcp.WithString("topic",
			gomcp.Description("Topic to unsubscribe from"),
			gomcp.Required(),
		),
	)
	mcpServer.AddTool(detachTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		topic, err := req.RequireString("topic")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		if err := s.client.Detach(s.peerID, topic); err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(fmt.Sprintf("Detached from topic: %s", topic)), nil
	})

	// nurture
	nurtureTool := gomcp.NewTool("nurture",
		gomcp.WithDescription("Delegate a task to another peer"),
		gomcp.WithString("to",
			gomcp.Description("Target peer ID"),
			gomcp.Required(),
		),
		gomcp.WithString("description",
			gomcp.Description("Task description"),
			gomcp.Required(),
		),
		gomcp.WithString("context",
			gomcp.Description("Optional context for the task"),
		),
	)
	mcpServer.AddTool(nurtureTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		to, err := req.RequireString("to")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		description, err := req.RequireString("description")
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		taskContext := req.GetString("context", "")
		taskID, err := s.client.Nurture(s.peerID, to, description, taskContext)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(fmt.Sprintf("Task delegated (id: %s)", taskID)), nil
	})

	// fruit
	fruitTool := gomcp.NewTool("fruit",
		gomcp.WithDescription("Report the result of a delegated task"),
		gomcp.WithString("task_id",
			gomcp.Description("Task ID to report on"),
			gomcp.Required(),
		),
		gomcp.WithString("status",
			gomcp.Description("Task completion status"),
			gomcp.Required(),
			gomcp.Enum("completed", "failed"),
		),
		gomcp.WithString("result",
			gomcp.Description("Result or error message"),
			gomcp.Required(),
		),
	)
	mcpServer.AddTool(fruitTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
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
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText("Fruit borne"), nil
	})

	// survey
	surveyTool := gomcp.NewTool("survey",
		gomcp.WithDescription("List all tasks delegated by and received by this peer"),
	)
	mcpServer.AddTool(surveyTool, func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		result, err := s.client.Survey(s.peerID)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		data, err := jsonMarshal(result)
		if err != nil {
			return gomcp.NewToolResultError(err.Error()), nil
		}
		return gomcp.NewToolResultText(string(data)), nil
	})
}

func jsonMarshal(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
