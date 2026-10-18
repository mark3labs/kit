package test

import (
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// TestMCPToolsReadyCrossesYaegiBoundary verifies that OnMCPToolsReady reaches
// an interpreted handler with the server list intact, and that the
// HideMCPStartupMessages and PrintBlockOpts.Markdown fields reach the host.
// Yaegi yields zero values for anything missing from symbols.go, so the test
// asserts on real field data.
func TestMCPToolsReadyCrossesYaegiBoundary(t *testing.T) {
	src := `package main

import (
	"strings"

	"kit/ext"
)

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetUIVisibility(ext.UIVisibility{HideMCPStartupMessages: true})
	})

	api.OnMCPToolsReady(func(e ext.MCPToolsReadyEvent, ctx ext.Context) {
		var rows []string
		for _, s := range e.Servers {
			rows = append(rows, s.Name+": "+strings.Join(s.Tools, ","))
		}
		ctx.PrintBlock(ext.PrintBlockOpts{Text: strings.Join(rows, "\n"), Markdown: true})
	})
}
`

	harness := New(t)
	harness.LoadString(src, "mcp-ready.go")

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "t"}); err != nil {
		t.Fatalf("Emit(SessionStart) error = %v", err)
	}
	if _, err := harness.Emit(extensions.MCPToolsReadyEvent{Servers: []extensions.MCPServerInfo{
		{Name: "snapper", Tools: []string{"create", "undochange"}},
		{Name: "zypper", Tools: []string{"search"}},
	}}); err != nil {
		t.Fatalf("Emit(MCPToolsReady) error = %v", err)
	}

	mock := harness.Context()
	if mock.UIVisibility == nil || !mock.UIVisibility.HideMCPStartupMessages {
		t.Errorf("UIVisibility = %+v; want HideMCPStartupMessages set", mock.UIVisibility)
	}
	if len(mock.PrintBlocks) != 1 {
		t.Fatalf("got %d print blocks; want 1", len(mock.PrintBlocks))
	}
	block := mock.PrintBlocks[0]
	if want := "snapper: create,undochange\nzypper: search"; block.Text != want {
		t.Errorf("block text:\n got %q\nwant %q", block.Text, want)
	}
	if !block.Markdown {
		t.Error("PrintBlockOpts.Markdown did not cross the interpreter boundary")
	}
}
