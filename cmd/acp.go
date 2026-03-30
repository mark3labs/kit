package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
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
	// writes responses to stdout. We wrap stdin with a normalizer that
	// fills in optional fields the SDK's generated validation requires
	// (e.g. mcpServers) so clients that omit them still work.
	conn := acp.NewAgentSideConnection(agent, os.Stdout, newACPNormalizer(os.Stdin))

	// Wire the connection back to the agent so it can send session updates.
	agent.SetAgentConnection(conn)

	// Enable debug logging to stderr if requested.
	if debugMode {
		conn.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})))
		// Also set charmbracelet/log level for acpserver package logging
		log.SetLevel(log.DebugLevel)
	}

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

// acpNormalizer wraps an io.Reader carrying newline-delimited JSON-RPC and
// patches incoming messages so that fields the SDK validates as required —
// but that some clients (e.g. Zed) omit — are defaulted. This avoids
// InvalidParams errors without forking the SDK.
type acpNormalizer struct {
	scanner *bufio.Scanner
	buf     bytes.Buffer // leftover bytes from the last normalized line
}

func newACPNormalizer(r io.Reader) *acpNormalizer {
	const maxMsg = 10 * 1024 * 1024 // 10 MB, matches SDK buffer
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 1024*1024), maxMsg)
	return &acpNormalizer{scanner: s}
}

// Read satisfies io.Reader. It feeds one normalized JSON line (plus newline)
// per underlying scan, buffering across short caller reads.
func (n *acpNormalizer) Read(p []byte) (int, error) {
	// Drain any leftover bytes from the previous line first.
	if n.buf.Len() > 0 {
		return n.buf.Read(p)
	}

	if !n.scanner.Scan() {
		if err := n.scanner.Err(); err != nil {
			return 0, err
		}
		return 0, io.EOF
	}

	line := n.scanner.Bytes()
	normalized := normalizeACPLine(line)
	n.buf.Write(normalized)
	n.buf.WriteByte('\n')
	return n.buf.Read(p)
}

// normalizeACPLine ensures session/new and session/load params contain an
// mcpServers array. Returns the original line unchanged for all other methods.
func normalizeACPLine(line []byte) []byte {
	// Quick check: if it already contains mcpServers, nothing to do.
	if bytes.Contains(line, []byte(`"mcpServers"`)) {
		return line
	}

	// Only bother parsing if the method could be session/new or session/load.
	if !bytes.Contains(line, []byte(`"session/new"`)) &&
		!bytes.Contains(line, []byte(`"session/load"`)) {
		return line
	}

	var msg struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(line, &msg); err != nil {
		return line
	}
	if msg.Method != "session/new" && msg.Method != "session/load" {
		return line
	}

	// Patch params to include mcpServers: [].
	var params map[string]json.RawMessage
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return line
	}
	if _, ok := params["mcpServers"]; ok {
		return line
	}
	params["mcpServers"] = json.RawMessage(`[]`)

	patched, err := json.Marshal(params)
	if err != nil {
		return line
	}
	msg.Params = patched

	out, err := json.Marshal(msg)
	if err != nil {
		return line
	}
	return out
}
