//go:build ignore

// dev-reload.go — Extension Hot-Reload example extension for Kit.
//
// Demonstrates ctx.ReloadExtensions() which hot-reloads all extensions
// from disk without restarting Kit. This is invaluable during extension
// development: edit your extension source, then type /reload to pick up
// changes immediately.
//
// Event handlers, slash commands, tool renderers, message renderers, and
// keyboard shortcuts update immediately. Extension-defined tools are NOT
// updated (they are baked into the agent at creation time and require a
// restart).
//
// Commands:
//   /reload   — hot-reload all extensions from disk

package main

import (
	"fmt"
	"time"

	ext "kit/ext"
)

var loadedAt string

func Init(api ext.API) {
	loadedAt = time.Now().Format("15:04:05")

	api.RegisterCommand(ext.CommandDef{
		Name:        "reload",
		Description: "Hot-reload all extensions from disk",
		Execute: func(args string, ctx ext.Context) (string, error) {
			ctx.Print("Reloading extensions...")
			err := ctx.ReloadExtensions()
			if err != nil {
				return "", fmt.Errorf("reload failed: %w", err)
			}
			return "Extensions reloaded successfully.", nil
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "load-time",
		Description: "Show when this extension was loaded",
		Execute: func(args string, ctx ext.Context) (string, error) {
			return fmt.Sprintf("This extension was loaded at %s", loadedAt), nil
		},
	})

	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print(fmt.Sprintf("[dev-reload] Extension loaded at %s", loadedAt))
	})
}
