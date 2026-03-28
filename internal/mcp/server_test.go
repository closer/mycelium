package mcp_test

import (
	"testing"

	mcpserver "github.com/closer/mycelium/internal/mcp"
)

func TestGenerateSummary(t *testing.T) {
	summary := mcpserver.GenerateSummary("/tmp")
	if summary == "" {
		t.Fatal("summary should not be empty")
	}
}
