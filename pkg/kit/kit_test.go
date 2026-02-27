package kit_test

import (
	"context"
	"os"
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestNew(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	// Test default initialization
	host, err := kit.New(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create Kit with defaults: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() == "" {
		t.Error("Model string should not be empty")
	}
}

func TestNewWithOptions(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	opts := &kit.Options{
		Model:    "anthropic/claude-sonnet-4-5-20250929",
		MaxSteps: 5,
		Quiet:    true,
	}

	host, err := kit.New(ctx, opts)
	if err != nil {
		t.Fatalf("Failed to create Kit with options: %v", err)
	}
	defer func() { _ = host.Close() }()

	if host.GetModelString() != opts.Model {
		t.Errorf("Expected model %s, got %s", opts.Model, host.GetModelString())
	}
}

func TestSessionManagement(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("Skipping test: ANTHROPIC_API_KEY not set")
	}

	ctx := context.Background()

	host, err := kit.New(ctx, &kit.Options{Quiet: true})
	if err != nil {
		t.Fatalf("Failed to create Kit: %v", err)
	}
	defer func() { _ = host.Close() }()

	// Test clear session
	host.ClearSession()
	mgr := host.GetSessionManager()
	if mgr.MessageCount() != 0 {
		t.Error("Session should be empty after clear")
	}

	// Test save/load session (would need actual implementation)
	tempFile := t.TempDir() + "/session.json"

	// Add a message first
	_, err = host.Prompt(ctx, "test message")
	if err == nil { // Only if we have a working model
		if err := host.SaveSession(tempFile); err != nil {
			t.Errorf("Failed to save session: %v", err)
		}

		// Clear and reload
		host.ClearSession()
		if err := host.LoadSession(tempFile); err != nil {
			t.Errorf("Failed to load session: %v", err)
		}
	}
}
