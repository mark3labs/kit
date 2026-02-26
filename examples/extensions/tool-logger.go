//go:build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"kit/ext"
)

// Init registers handlers that log all tool calls and session lifecycle
// events to /tmp/kit-tool-log.txt.
func Init(api ext.API) {
	logFile := "/tmp/kit-tool-log.txt"

	// Log every tool call before execution.
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "[%s] CALL tool=%s model=%s\n",
				time.Now().Format(time.RFC3339), tc.ToolName, ctx.Model)
		}
		return nil
	})

	// Log tool results after execution.
	api.OnToolResult(func(tr ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			status := "ok"
			if tr.IsError {
				status = "error"
			}
			fmt.Fprintf(f, "[%s] RESULT tool=%s status=%s bytes=%d\n",
				time.Now().Format(time.RFC3339), tr.ToolName, status, len(tr.Content))
		}
		return nil // don't modify the result
	})

	// Log session start/shutdown.
	api.OnSessionStart(func(se ext.SessionStartEvent, ctx ext.Context) {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "[%s] SESSION_START cwd=%s\n",
				time.Now().Format(time.RFC3339), ctx.CWD)
		}
	})

	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			defer f.Close()
			fmt.Fprintf(f, "[%s] SESSION_SHUTDOWN\n",
				time.Now().Format(time.RFC3339))
		}
	})

	// "!time" — prints the current time as a styled info block.
	// "!status" — prints a custom block with green border and subtitle.
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		switch ie.Text {
		case "!time":
			ctx.PrintInfo("Current time: " + time.Now().Format(time.RFC3339))
			return &ext.InputResult{Action: "handled"}

		case "!status":
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        "Session active\nModel: " + ctx.Model + "\nCWD: " + ctx.CWD,
				BorderColor: "#a6e3a1",
				Subtitle:    "tool-logger extension",
			})
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
