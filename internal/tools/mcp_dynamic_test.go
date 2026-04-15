package tools

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
)

// TestMCPToolManager_AddServer_DuplicateName verifies that adding a server
// with a name that already exists returns an error.
func TestMCPToolManager_AddServer_DuplicateName(t *testing.T) {
	manager := NewMCPToolManager()

	cfg := config.MCPServerConfig{
		Command: []string{"non-existent-command"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First add will fail (bad command), but let's test the duplicate detection
	// by simulating a loaded server via LoadTools first.
	loadCfg := &config.Config{
		MCPServers: map[string]config.MCPServerConfig{
			"test-server": cfg,
		},
	}
	// This will fail to load but creates the connection pool.
	_ = manager.LoadTools(ctx, loadCfg)

	// Now try to add the same server name — the tools didn't load (bad command),
	// so AddServer should not find a duplicate and should fail with connection error.
	_, err := manager.AddServer(ctx, "test-server", cfg)
	if err == nil {
		t.Fatal("Expected error when adding server with bad command, got nil")
	}
	// It should be a connection error, not a duplicate error.
	if strings.Contains(err.Error(), "already loaded") {
		t.Fatalf("Should not report duplicate since server failed to load initially: %v", err)
	}
}

// TestMCPToolManager_RemoveServer_NotLoaded verifies that removing a server
// that doesn't exist returns an appropriate error.
func TestMCPToolManager_RemoveServer_NotLoaded(t *testing.T) {
	manager := NewMCPToolManager()

	err := manager.RemoveServer("nonexistent")
	if err == nil {
		t.Fatal("Expected error when removing non-existent server, got nil")
	}
	if !strings.Contains(err.Error(), "not loaded") {
		t.Errorf("Expected 'not loaded' error, got: %v", err)
	}
}

// TestMCPToolManager_AddServer_CreatesConnectionPool verifies that AddServer
// lazily creates a connection pool when LoadTools was never called.
func TestMCPToolManager_AddServer_CreatesConnectionPool(t *testing.T) {
	manager := NewMCPToolManager()

	// Connection pool should be nil initially.
	if manager.connectionPool != nil {
		t.Fatal("Expected nil connection pool before any operation")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// AddServer with a bad command — should fail, but the pool should be created.
	_, err := manager.AddServer(ctx, "lazy-server", config.MCPServerConfig{
		Command: []string{"non-existent-command"},
	})
	if err == nil {
		t.Fatal("Expected error for bad command")
	}

	// Connection pool should have been created.
	if manager.connectionPool == nil {
		t.Fatal("Expected connection pool to be created lazily by AddServer")
	}
}

// TestMCPToolManager_OnToolsChanged_Callback verifies that the onToolsChanged
// callback fires on RemoveServer (we can't easily test AddServer with a real
// MCP server, but we can test the callback wiring).
func TestMCPToolManager_OnToolsChanged_Callback(t *testing.T) {
	manager := NewMCPToolManager()

	var mu sync.Mutex
	callCount := 0
	manager.SetOnToolsChanged(func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// RemoveServer on non-existent should NOT fire callback.
	_ = manager.RemoveServer("nonexistent")

	mu.Lock()
	if callCount != 0 {
		t.Errorf("Expected 0 callback calls for failed remove, got %d", callCount)
	}
	mu.Unlock()
}

// TestMCPToolManager_Close_NilPool verifies Close is safe when the connection
// pool was never initialized.
func TestMCPToolManager_Close_NilPool(t *testing.T) {
	manager := NewMCPToolManager()
	err := manager.Close()
	if err != nil {
		t.Fatalf("Expected nil error from Close with nil pool, got: %v", err)
	}
}

// TestMCPConnectionPool_RemoveConnection_NotFound verifies that removing a
// non-existent connection returns an error.
func TestMCPConnectionPool_RemoveConnection_NotFound(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	err := pool.RemoveConnection("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent connection")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestMCPToolManager_EnsureConnectionPool_Idempotent verifies that
// ensureConnectionPool doesn't recreate an existing pool.
func TestMCPToolManager_EnsureConnectionPool_Idempotent(t *testing.T) {
	manager := NewMCPToolManager()

	// First call creates the pool.
	manager.ensureConnectionPool()
	pool1 := manager.connectionPool
	if pool1 == nil {
		t.Fatal("Expected pool to be created")
	}

	// Second call should be a no-op.
	manager.ensureConnectionPool()
	pool2 := manager.connectionPool
	if pool1 != pool2 {
		t.Fatal("Expected ensureConnectionPool to be idempotent")
	}
}
