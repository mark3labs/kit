package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// extensionFiles returns all single-file extensions in the current directory.
// It skips test files, the test template, and files without an Init function.
func extensionFiles(t *testing.T) []string {
	t.Helper()

	skip := map[string]bool{
		"extension_test_template.go": true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}

	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || skip[name] {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}
		if !strings.Contains(string(src), "func Init(") {
			continue
		}
		files = append(files, name)
	}

	if len(files) == 0 {
		t.Fatal("no extensions found — check the directory")
	}
	return files
}

// TestAllExtensions_Lifecycle verifies that every extension survives a full
// SessionStart → SessionShutdown round-trip without errors.
func TestAllExtensions_Lifecycle(t *testing.T) {
	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			_, err := harness.Emit(extensions.SessionStartEvent{
				SessionID: "smoke-test-session",
			})
			if err != nil {
				t.Fatalf("SessionStart error: %v", err)
			}

			_, err = harness.Emit(extensions.SessionShutdownEvent{})
			if err != nil {
				t.Fatalf("SessionShutdown error: %v", err)
			}
		})
	}
}

// TestAllExtensions_CommandSanity checks that every registered command has
// a non-empty name, a non-empty description, no spaces in the name, no
// leading slash, a non-nil Execute function, and no duplicate names.
func TestAllExtensions_CommandSanity(t *testing.T) {
	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			cmds := harness.RegisteredCommands()
			seen := make(map[string]bool)
			for _, cmd := range cmds {
				if cmd.Name == "" {
					t.Error("command has empty name")
				}
				if strings.Contains(cmd.Name, " ") {
					t.Errorf("command %q contains spaces", cmd.Name)
				}
				if strings.HasPrefix(cmd.Name, "/") {
					t.Errorf("command %q has leading slash (framework adds it)", cmd.Name)
				}
				if cmd.Description == "" {
					t.Errorf("command %q has empty description", cmd.Name)
				}
				if cmd.Execute == nil {
					t.Errorf("command %q has nil Execute function", cmd.Name)
				}
				if seen[cmd.Name] {
					t.Errorf("duplicate command name %q", cmd.Name)
				}
				seen[cmd.Name] = true
			}
		})
	}
}

// TestAllExtensions_ToolSanity checks that every registered tool has a
// non-empty name, a non-empty description, at least one executor, valid
// JSON in its Parameters field, and no duplicate names.
func TestAllExtensions_ToolSanity(t *testing.T) {
	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			tools := harness.RegisteredTools()
			seen := make(map[string]bool)
			for _, tool := range tools {
				if tool.Name == "" {
					t.Error("tool has empty name")
				}
				if tool.Description == "" {
					t.Errorf("tool %q has empty description", tool.Name)
				}
				if tool.Execute == nil && tool.ExecuteWithContext == nil {
					t.Errorf("tool %q has no executor (both Execute and ExecuteWithContext are nil)", tool.Name)
				}
				if tool.Parameters != "" && !json.Valid([]byte(tool.Parameters)) {
					t.Errorf("tool %q has invalid JSON in Parameters: %s", tool.Name, tool.Parameters)
				}
				if seen[tool.Name] {
					t.Errorf("duplicate tool name %q", tool.Name)
				}
				seen[tool.Name] = true
			}
		})
	}
}

// TestAllExtensions_ZeroValueEvents fires every event type (as zero-value
// structs) at each extension and verifies no errors are returned. Extensions
// should be resilient to events they don't handle and to events with empty
// fields.
func TestAllExtensions_ZeroValueEvents(t *testing.T) {
	// Build the set of zero-value events for every event type.
	zeroEvents := []extensions.Event{
		extensions.ToolCallEvent{},
		extensions.ToolExecutionStartEvent{},
		extensions.ToolExecutionEndEvent{},
		extensions.ToolOutputEvent{},
		extensions.ToolResultEvent{},
		extensions.InputEvent{},
		extensions.BeforeAgentStartEvent{},
		extensions.AgentStartEvent{},
		extensions.AgentEndEvent{},
		extensions.MessageStartEvent{},
		extensions.MessageUpdateEvent{},
		extensions.MessageEndEvent{},
		extensions.SessionStartEvent{},
		extensions.SessionShutdownEvent{},
		extensions.ModelChangeEvent{},
		extensions.ContextPrepareEvent{},
		extensions.BeforeForkEvent{},
		extensions.BeforeSessionSwitchEvent{},
		extensions.BeforeCompactEvent{},
		extensions.SubagentStartEvent{},
		extensions.SubagentChunkEvent{},
		extensions.SubagentEndEvent{},
	}

	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			for _, ev := range zeroEvents {
				_, err := harness.Emit(ev)
				if err != nil {
					t.Errorf("event %T returned error: %v", ev, err)
				}
			}
		})
	}
}

// TestAllExtensions_WidgetSanity emits SessionStart and then checks that
// any widgets set during initialization have non-empty IDs and valid
// placements.
func TestAllExtensions_WidgetSanity(t *testing.T) {
	validPlacements := map[extensions.WidgetPlacement]bool{
		"above": true,
		"below": true,
	}

	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			// Trigger SessionStart so extensions that set widgets on init do so.
			_, _ = harness.Emit(extensions.SessionStartEvent{
				SessionID: "widget-sanity-test",
			})

			// Widgets is an exported field on MockContext; reads are safe
			// here because Emit returned synchronously.
			for id, w := range harness.Context().Widgets {
				if w.ID == "" {
					t.Errorf("widget stored with key %q has empty ID", id)
				}
				if w.ID != id {
					t.Errorf("widget key %q doesn't match widget ID %q", id, w.ID)
				}
				if !validPlacements[w.Placement] {
					t.Errorf("widget %q has invalid placement %q (want \"above\" or \"below\")", id, w.Placement)
				}
			}
		})
	}
}

// TestAllExtensions_IdempotentLifecycle verifies that receiving SessionStart
// twice and SessionShutdown twice doesn't cause errors — extensions should
// be defensive about repeated lifecycle events.
func TestAllExtensions_IdempotentLifecycle(t *testing.T) {
	for _, file := range extensionFiles(t) {
		t.Run(file, func(t *testing.T) {
			harness := test.New(t)
			harness.LoadFile(file)

			for i := range 2 {
				_, err := harness.Emit(extensions.SessionStartEvent{
					SessionID: "idempotent-test",
				})
				if err != nil {
					t.Fatalf("SessionStart #%d error: %v", i+1, err)
				}
			}

			for i := range 2 {
				_, err := harness.Emit(extensions.SessionShutdownEvent{})
				if err != nil {
					t.Fatalf("SessionShutdown #%d error: %v", i+1, err)
				}
			}
		})
	}
}
