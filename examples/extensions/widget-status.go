//go:build ignore

package main

import (
	"fmt"
	"time"

	"kit/ext"
)

// Init demonstrates the widget system by showing a persistent status
// widget above the input area. The widget updates on each agent turn
// to show a running count of tool calls and the last tool used.
func Init(api ext.API) {
	var toolCallCount int
	var lastToolName string

	// Show initial status widget when session starts.
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "widget-status:info",
			Placement: ext.WidgetAbove,
			Content: ext.WidgetContent{
				Text: fmt.Sprintf("Session started  |  CWD: %s  |  Model: %s", ctx.CWD, ctx.Model),
			},
			Style: ext.WidgetStyle{
				BorderColor: "#89b4fa",
			},
		})
	})

	// Update the widget after each tool call with a running count.
	api.OnToolResult(func(tr ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
		toolCallCount++
		lastToolName = tr.ToolName

		status := "ok"
		if tr.IsError {
			status = "error"
		}

		ctx.SetWidget(ext.WidgetConfig{
			ID:        "widget-status:info",
			Placement: ext.WidgetAbove,
			Content: ext.WidgetContent{
				Text: fmt.Sprintf(
					"Tools: %d calls  |  Last: %s (%s)  |  %s",
					toolCallCount, lastToolName, status,
					time.Now().Format("15:04:05"),
				),
			},
			Style: ext.WidgetStyle{
				BorderColor: "#a6e3a1",
			},
		})
		return nil
	})

	// "!widget-off" — removes the status widget.
	// "!widget-on"  — restores the status widget.
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		switch ie.Text {
		case "!widget-off":
			ctx.RemoveWidget("widget-status:info")
			ctx.PrintInfo("Status widget removed.")
			return &ext.InputResult{Action: "handled"}

		case "!widget-on":
			ctx.SetWidget(ext.WidgetConfig{
				ID:        "widget-status:info",
				Placement: ext.WidgetAbove,
				Content: ext.WidgetContent{
					Text: fmt.Sprintf("Tools: %d calls  |  %s", toolCallCount, time.Now().Format("15:04:05")),
				},
				Style: ext.WidgetStyle{
					BorderColor: "#a6e3a1",
				},
			})
			ctx.PrintInfo("Status widget restored.")
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})

	// Clean up widget on shutdown.
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		ctx.RemoveWidget("widget-status:info")
	})
}
