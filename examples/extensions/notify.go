//go:build ignore

package main

import (
	"os/exec"
	"runtime"

	"kit/ext"
)

// Init sends a desktop notification when the agent finishes responding.
// Useful for long-running tasks — get notified without watching the terminal.
// Inspired by Pi's notify.ts.
//
// Supports: Linux (notify-send), macOS (osascript).
//
// Usage: kit -e examples/extensions/notify.go
func Init(api ext.API) {
	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		sendNotification("Kit", "Agent finished responding")
	})
}

func sendNotification(title, body string) {
	switch runtime.GOOS {
	case "linux":
		// Uses notify-send (libnotify) — available on most Linux desktops.
		_ = exec.Command("notify-send", "-a", "Kit", title, body).Start()
	case "darwin":
		// Uses macOS built-in osascript for native notifications.
		script := `display notification "` + body + `" with title "` + title + `"`
		_ = exec.Command("osascript", "-e", script).Start()
	}
}
