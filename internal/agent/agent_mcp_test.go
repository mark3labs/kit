package agent

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/config"
)

// mockModel is a minimal LanguageModel that satisfies the interface
// without making real API calls. Used to test tool management wiring.
type mockModel struct{}

func (m *mockModel) Generate(_ context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}
func (m *mockModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	return nil, nil
}
func (m *mockModel) GenerateObject(_ context.Context, _ fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}
func (m *mockModel) StreamObject(_ context.Context, _ fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}
func (m *mockModel) Provider() string { return "mock" }
func (m *mockModel) Model() string    { return "mock-model" }

// testdataDir returns the absolute path to the tools testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "tools", "testdata")
}

// echoServerConfig returns an MCPServerConfig for the test echo MCP server.
func echoServerConfig(t *testing.T) config.MCPServerConfig {
	t.Helper()
	script := filepath.Join(testdataDir(t), "echo_server.py")
	if _, err := os.Stat(script); err != nil {
		t.Skipf("echo_server.py not found: %v", err)
	}
	return config.MCPServerConfig{
		Command: []string{"python3", script},
	}
}

// mockAuthHandler is a minimal MCPAuthHandler for testing that auth handler
// propagation works without requiring a real OAuth server.
type mockAuthHandler struct {
	redirectURI string
}

func (h *mockAuthHandler) RedirectURI() string { return h.redirectURI }
func (h *mockAuthHandler) HandleAuth(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// newTestAgent creates a minimal Agent with a mock model and no core tools,
// suitable for testing MCP server management without an API key.
func newTestAgent() *Agent {
	model := &mockModel{}
	a := &Agent{
		model:        model,
		coreTools:    nil,
		extraTools:   nil,
		maxSteps:     10,
		systemPrompt: "test",
		fantasyAgent: fantasy.NewAgent(model),
	}
	return a
}

func TestAgent_AddMCPServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a := newTestAgent()
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Initially no MCP tools.
	if a.GetMCPToolCount() != 0 {
		t.Fatalf("Expected 0 MCP tools initially, got %d", a.GetMCPToolCount())
	}

	// Add a server.
	count, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddMCPServer failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tools, got %d", count)
	}

	// Verify tools are in the agent's tool list.
	if a.GetMCPToolCount() != 2 {
		t.Errorf("Expected 2 MCP tools, got %d", a.GetMCPToolCount())
	}

	allTools := a.GetTools()
	toolNames := make(map[string]bool)
	for _, tool := range allTools {
		toolNames[tool.Info().Name] = true
	}
	if !toolNames["echo__echo"] {
		t.Error("Expected tool 'echo__echo' in agent tools")
	}
	if !toolNames["echo__greet"] {
		t.Error("Expected tool 'echo__greet' in agent tools")
	}

	// Verify loaded server names.
	names := a.GetLoadedServerNames()
	found := false
	for _, n := range names {
		if n == "echo" {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected 'echo' in loaded server names: %v", names)
	}
}

func TestAgent_RemoveMCPServer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a := newTestAgent()
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add then remove.
	_, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddMCPServer failed: %v", err)
	}

	err = a.RemoveMCPServer("echo")
	if err != nil {
		t.Fatalf("RemoveMCPServer failed: %v", err)
	}

	// Verify tools removed.
	if a.GetMCPToolCount() != 0 {
		t.Errorf("Expected 0 MCP tools after removal, got %d", a.GetMCPToolCount())
	}

	// Verify agent's tool list has no MCP tools.
	for _, tool := range a.GetTools() {
		if strings.Contains(tool.Info().Name, "echo__") {
			t.Errorf("Found leftover tool after removal: %s", tool.Info().Name)
		}
	}
}

func TestAgent_RemoveMCPServer_NoToolManager(t *testing.T) {
	a := newTestAgent()
	defer func() { _ = a.Close() }()

	err := a.RemoveMCPServer("nonexistent")
	if err == nil {
		t.Fatal("Expected error when no tool manager exists")
	}
	if !strings.Contains(err.Error(), "no MCP servers loaded") {
		t.Errorf("Expected 'no MCP servers loaded' error, got: %v", err)
	}
}

func TestAgent_AddMCPServer_CreatesToolManager(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a := newTestAgent()
	defer func() { _ = a.Close() }()

	// Initially no tool manager.
	if a.GetMCPToolManager() != nil {
		t.Fatal("Expected nil tool manager initially")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)
	_, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddMCPServer failed: %v", err)
	}

	// Tool manager should now exist.
	if a.GetMCPToolManager() == nil {
		t.Fatal("Expected tool manager to be created by AddMCPServer")
	}
}

func TestAgent_AddRemoveAdd_MCP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a := newTestAgent()
	defer func() { _ = a.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add → Remove → Add cycle.
	_, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("First add failed: %v", err)
	}

	err = a.RemoveMCPServer("echo")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	count, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("Re-add failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tools on re-add, got %d", count)
	}
	if a.GetMCPToolCount() != 2 {
		t.Errorf("Expected 2 MCP tools after re-add, got %d", a.GetMCPToolCount())
	}
}

// TestAgent_AddMCPServer_InheritsAuthHandler verifies that AddMCPServer()
// propagates the agent's authHandler and tokenStoreFactory to a newly created
// MCPToolManager (fix for issue #3).
func TestAgent_AddMCPServer_InheritsAuthHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	handler := &mockAuthHandler{redirectURI: "http://localhost:9999/oauth/callback"}

	model := &mockModel{}
	a := &Agent{
		model:             model,
		coreTools:         nil,
		extraTools:        nil,
		maxSteps:          10,
		systemPrompt:      "test",
		fantasyAgent:      fantasy.NewAgent(model),
		authHandler:       handler,
		tokenStoreFactory: nil, // nil is fine; we just test authHandler propagation
	}
	defer func() { _ = a.Close() }()

	// Initially no tool manager.
	if a.GetMCPToolManager() != nil {
		t.Fatal("Expected nil tool manager initially")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)
	_, err := a.AddMCPServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddMCPServer failed: %v", err)
	}

	// Tool manager should now exist and have the auth handler set.
	tm := a.GetMCPToolManager()
	if tm == nil {
		t.Fatal("Expected tool manager to be created by AddMCPServer")
	}

	// Verify the auth handler was propagated by checking the field directly.
	if tm.GetAuthHandler() == nil {
		t.Fatal("Expected auth handler to be propagated to tool manager")
	}
}
