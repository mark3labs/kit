package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "testdata")
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

// TestMCPToolManager_AddServer_Integration tests adding a real MCP server
// at runtime and verifying tools are loaded.
func TestMCPToolManager_AddServer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	manager := NewMCPToolManager()
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Track callbacks.
	var mu sync.Mutex
	var loadedServer string
	var loadedCount int
	toolsChangedCount := 0

	manager.SetOnServerLoaded(func(name string, count int, err error) {
		mu.Lock()
		loadedServer = name
		loadedCount = count
		mu.Unlock()
	})
	manager.SetOnToolsChanged(func() {
		mu.Lock()
		toolsChangedCount++
		mu.Unlock()
	})

	// Add the server.
	count, err := manager.AddServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 tools from echo server, got %d", count)
	}

	// Verify callbacks fired.
	mu.Lock()
	if loadedServer != "echo" {
		t.Errorf("Expected onServerLoaded for 'echo', got %q", loadedServer)
	}
	if loadedCount != 2 {
		t.Errorf("Expected onServerLoaded count=2, got %d", loadedCount)
	}
	if toolsChangedCount != 1 {
		t.Errorf("Expected onToolsChanged called once, got %d", toolsChangedCount)
	}
	mu.Unlock()

	// Verify tools are accessible.
	tools := manager.GetTools()
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	// Verify tool names are prefixed.
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Info().Name] = true
	}
	if !toolNames["echo__echo"] {
		t.Error("Expected tool 'echo__echo'")
	}
	if !toolNames["echo__greet"] {
		t.Error("Expected tool 'echo__greet'")
	}

	// Verify server appears in loaded names.
	names := manager.GetLoadedServerNames()
	if !slices.Contains(names, "echo") {
		t.Errorf("Expected 'echo' in loaded server names, got: %v", names)
	}
}

// TestMCPToolManager_RemoveServer_Integration tests removing a real MCP server
// and verifying tools are cleaned up.
func TestMCPToolManager_RemoveServer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	manager := NewMCPToolManager()
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add the server first.
	count, err := manager.AddServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 tools, got %d", count)
	}

	var mu sync.Mutex
	toolsChangedCount := 0
	manager.SetOnToolsChanged(func() {
		mu.Lock()
		toolsChangedCount++
		mu.Unlock()
	})

	// Remove the server.
	err = manager.RemoveServer("echo")
	if err != nil {
		t.Fatalf("RemoveServer failed: %v", err)
	}

	// Verify tools are gone.
	tools := manager.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after removal, got %d", len(tools))
	}

	// Verify callback fired.
	mu.Lock()
	if toolsChangedCount != 1 {
		t.Errorf("Expected onToolsChanged called once, got %d", toolsChangedCount)
	}
	mu.Unlock()

	// Verify server is gone from loaded names.
	names := manager.GetLoadedServerNames()
	for _, n := range names {
		if n == "echo" {
			t.Error("Server 'echo' should not appear in loaded names after removal")
		}
	}

	// Removing again should error.
	err = manager.RemoveServer("echo")
	if err == nil {
		t.Fatal("Expected error removing already-removed server")
	}
	if !strings.Contains(err.Error(), "not loaded") {
		t.Errorf("Expected 'not loaded' error, got: %v", err)
	}
}

// TestMCPToolManager_AddRemoveMultiple_Integration tests adding and removing
// multiple servers, verifying tool isolation.
func TestMCPToolManager_AddRemoveMultiple_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	manager := NewMCPToolManager()
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add two servers with the same binary but different names.
	count1, err := manager.AddServer(ctx, "server-a", cfg)
	if err != nil {
		t.Fatalf("AddServer server-a failed: %v", err)
	}
	count2, err := manager.AddServer(ctx, "server-b", cfg)
	if err != nil {
		t.Fatalf("AddServer server-b failed: %v", err)
	}

	totalTools := count1 + count2
	if totalTools != 4 {
		t.Fatalf("Expected 4 total tools (2+2), got %d", totalTools)
	}

	tools := manager.GetTools()
	if len(tools) != 4 {
		t.Fatalf("Expected 4 tools, got %d", len(tools))
	}

	// Remove server-a, verify server-b tools remain.
	err = manager.RemoveServer("server-a")
	if err != nil {
		t.Fatalf("RemoveServer server-a failed: %v", err)
	}

	tools = manager.GetTools()
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools after removing server-a, got %d", len(tools))
	}

	// Remaining tools should all be from server-b.
	for _, tool := range tools {
		if !strings.HasPrefix(tool.Info().Name, "server-b__") {
			t.Errorf("Expected tool from server-b, got: %s", tool.Info().Name)
		}
	}

	// Remove server-b.
	err = manager.RemoveServer("server-b")
	if err != nil {
		t.Fatalf("RemoveServer server-b failed: %v", err)
	}

	tools = manager.GetTools()
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after removing all servers, got %d", len(tools))
	}
}

// TestMCPToolManager_AddServer_DuplicateDetection_Integration tests that
// adding a server with the same name as an already loaded server errors.
func TestMCPToolManager_AddServer_DuplicateDetection_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	manager := NewMCPToolManager()
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add the server.
	_, err := manager.AddServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("First AddServer failed: %v", err)
	}

	// Try to add again with the same name.
	_, err = manager.AddServer(ctx, "echo", cfg)
	if err == nil {
		t.Fatal("Expected error adding duplicate server")
	}
	if !strings.Contains(err.Error(), "already loaded") {
		t.Errorf("Expected 'already loaded' error, got: %v", err)
	}
}

// TestMCPToolManager_AddAfterRemove_Integration tests that a server can be
// re-added after being removed.
func TestMCPToolManager_AddAfterRemove_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	manager := NewMCPToolManager()
	defer func() { _ = manager.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := echoServerConfig(t)

	// Add, remove, re-add.
	_, err := manager.AddServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("First AddServer failed: %v", err)
	}

	err = manager.RemoveServer("echo")
	if err != nil {
		t.Fatalf("RemoveServer failed: %v", err)
	}

	count, err := manager.AddServer(ctx, "echo", cfg)
	if err != nil {
		t.Fatalf("Re-AddServer failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 tools on re-add, got %d", count)
	}

	tools := manager.GetTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools after re-add, got %d", len(tools))
	}
}
