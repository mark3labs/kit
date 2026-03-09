package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	acp "github.com/coder/acp-go-sdk"

	"github.com/mark3labs/kit/internal/acpserver"
	"github.com/spf13/cobra"
)

var acpCmd = &cobra.Command{
	Use:   "acp",
	Short: "Start Kit as an ACP agent server",
	Long: `Start Kit as an ACP (Agent Client Protocol) agent server.

Communicates over stdio (stdin/stdout) using JSON-RPC 2.0 with
newline-delimited JSON, compatible with OpenCode and other ACP clients.

The server exposes Kit's LLM execution, tool system, and session
management via the Agent Client Protocol. Sessions are persisted
to Kit's standard JSONL session files.`,
	RunE: runACP,
}

func init() {
	rootCmd.AddCommand(acpCmd)
}

func runACP(cmd *cobra.Command, _ []string) error {
	// Create the ACP agent implementation.
	agent := acpserver.NewAgent()
	defer agent.Close()

	// Create the stdio connection. The SDK reads JSON-RPC from stdin and
	// writes responses to stdout.
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)

	// Wire the connection back to the agent so it can send session updates.
	agent.SetAgentConnection(conn)

	// Enable debug logging to stderr if requested.
	if debugMode {
		conn.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
	}

	fmt.Fprintln(os.Stderr, "kit: ACP server ready on stdio")

	// Wait for either the client to disconnect or a signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-conn.Done():
		fmt.Fprintln(os.Stderr, "kit: ACP client disconnected")
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "kit: received %s, shutting down\n", sig)
	}

	return nil
}
