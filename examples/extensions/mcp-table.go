//go:build ignore

package main

import (
	"strings"

	"kit/ext"
)

// mcpStartupMode reads the mcp-startup option: "table" (the default), "none"
// or "default". Any other value counts as "table".
func mcpStartupMode(ctx ext.Context) string {
	mode := ctx.GetOption("mcp-startup")
	if mode == "none" || mode == "default" {
		return mode
	}
	return "table"
}

// mcpTable renders the loaded MCP servers as a two-column markdown table.
func mcpTable(servers []ext.MCPServerInfo) string {
	var b strings.Builder
	b.WriteString("| MCP server | Tools |\n|---|---|\n")
	for _, s := range servers {
		tools := "no tools"
		if len(s.Tools) > 0 {
			tools = strings.Join(s.Tools, ", ")
		}
		b.WriteString("| " + s.Name + " | " + tools + " |\n")
	}
	return b.String()
}

// Init replaces the message Kit prints for each MCP server that has loaded
// with one table of all servers and their tools, printed when loading has
// finished. Failure messages are not hidden, and a server that failed to
// load is not in the table.
//
// The mcp-startup option selects what you see:
//
//	table    one table for all servers (default)
//	none     no message for servers that loaded
//	default  Kit's message for each server
//
// Set it in the config file:
//
//	options:
//	  mcp-startup: none
//
// or for one run: KIT_OPT_MCP_STARTUP=none kit
//
// Usage: kit -e examples/extensions/mcp-table.go
func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "mcp-startup",
		Description: "How loaded MCP servers are shown: table, none or default",
		Default:     "table",
	})

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		if mcpStartupMode(ctx) != "default" {
			// SetUIVisibility sets all flags at once. An extension that
			// also hides other chrome sets those flags in the same call.
			ctx.SetUIVisibility(ext.UIVisibility{HideMCPStartupMessages: true})
		}
	})

	api.OnMCPToolsReady(func(e ext.MCPToolsReadyEvent, ctx ext.Context) {
		if mcpStartupMode(ctx) != "table" || len(e.Servers) == 0 {
			return
		}
		ctx.PrintBlock(ext.PrintBlockOpts{Text: mcpTable(e.Servers), Markdown: true})
	})
}
