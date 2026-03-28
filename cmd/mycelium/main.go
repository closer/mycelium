package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/closer/mycelium/internal/broker"
	"github.com/closer/mycelium/internal/client"
	mcpserver "github.com/closer/mycelium/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	brokerPort string
	brokerHost string
)

var rootCmd = &cobra.Command{
	Use:   "mycelium",
	Short: "Inter-session communication for Claude Code",
}

var brokerCmd = &cobra.Command{
	Use:   "broker",
	Short: "Start the broker daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		b := broker.New()
		b.StartCleanup(30*time.Second, 30*time.Second)
		addr := fmt.Sprintf("%s:%s", brokerHost, brokerPort)
		fmt.Fprintf(os.Stderr, "Broker listening on %s\n", addr)
		return http.ListenAndServe(addr, b.Handler())
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server (stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		s := mcpserver.NewServer(brokerURL)
		return s.Run()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show broker status and peer list",
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		c := client.New(brokerURL)
		if !c.IsAlive() {
			fmt.Println("Broker is not running")
			return nil
		}
		fmt.Println("Broker is running")
		id, err := c.Germinate(os.Getpid(), "/tmp/mycelium-status", "", "status check")
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer c.Wither(id)
		peers, err := c.Discover(id, "machine")
		if err != nil {
			return fmt.Errorf("failed to discover: %w", err)
		}
		if len(peers) == 0 {
			fmt.Println("No active peers")
			return nil
		}
		fmt.Printf("\nActive peers (%d):\n", len(peers))
		for _, p := range peers {
			fmt.Printf("  %s  %s  %s\n", p.ID[:8], p.CWD, p.Summary)
		}
		return nil
	},
}

var sendCmd = &cobra.Command{
	Use:   "send <peer_id> <message>",
	Short: "Send a message to a peer (debug)",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		brokerURL := fmt.Sprintf("http://%s:%s", brokerHost, brokerPort)
		c := client.New(brokerURL)
		id, err := c.Germinate(os.Getpid(), "/tmp/mycelium-cli", "", "CLI")
		if err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer c.Wither(id)
		msgID, err := c.Signal(id, args[0], strings.Join(args[1:], " "))
		if err != nil {
			return fmt.Errorf("failed to send: %w", err)
		}
		fmt.Printf("Sent (id: %s)\n", msgID)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&brokerPort, "port", getEnvDefault("MYCELIUM_PORT", "7890"), "broker port")
	rootCmd.PersistentFlags().StringVar(&brokerHost, "host", getEnvDefault("MYCELIUM_HOST", "localhost"), "broker host")
	rootCmd.AddCommand(brokerCmd, serveCmd, statusCmd, sendCmd)
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
