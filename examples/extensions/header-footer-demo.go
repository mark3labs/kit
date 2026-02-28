//go:build ignore

package main

import (
	"fmt"
	"time"

	"kit/ext"
)

// Init demonstrates the custom header/footer system. The header shows
// project context (branch, CWD) and the footer shows a running summary
// of agent activity. Slash commands toggle them on/off.
func Init(api ext.API) {
	var turnCount int
	var lastResponse string

	// Show a custom header with project context when the session starts.
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetHeader(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{
				Text: fmt.Sprintf("Project: %s  |  Model: %s  |  %s",
					ctx.CWD, ctx.Model, time.Now().Format("Jan 2, 15:04")),
			},
			Style: ext.WidgetStyle{
				BorderColor: "#89b4fa",
			},
		})

		ctx.SetFooter(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{
				Text: "Ready  |  0 turns",
			},
			Style: ext.WidgetStyle{
				BorderColor: "#a6e3a1",
			},
		})
	})

	// Update footer after each agent turn with activity summary.
	api.OnAgentEnd(func(ae ext.AgentEndEvent, ctx ext.Context) {
		turnCount++
		lastResponse = ae.Response
		if len(lastResponse) > 60 {
			lastResponse = lastResponse[:57] + "..."
		}

		ctx.SetFooter(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{
				Text: fmt.Sprintf("Turns: %d  |  Last: %s  |  %s",
					turnCount, ae.StopReason, time.Now().Format("15:04:05")),
			},
			Style: ext.WidgetStyle{
				BorderColor: "#a6e3a1",
			},
		})
	})

	// /header-off — remove the custom header.
	api.RegisterCommand(ext.CommandDef{
		Name:        "header-off",
		Description: "Remove the custom header",
		Execute: func(_ string, ctx ext.Context) (string, error) {
			ctx.RemoveHeader()
			return "Header removed.", nil
		},
	})

	// /header-on — restore the custom header.
	api.RegisterCommand(ext.CommandDef{
		Name:        "header-on",
		Description: "Restore the custom header",
		Execute: func(_ string, ctx ext.Context) (string, error) {
			ctx.SetHeader(ext.HeaderFooterConfig{
				Content: ext.WidgetContent{
					Text: fmt.Sprintf("Project: %s  |  Model: %s  |  %s",
						ctx.CWD, ctx.Model, time.Now().Format("Jan 2, 15:04")),
				},
				Style: ext.WidgetStyle{
					BorderColor: "#89b4fa",
				},
			})
			return "Header restored.", nil
		},
	})

	// /footer-off — remove the custom footer.
	api.RegisterCommand(ext.CommandDef{
		Name:        "footer-off",
		Description: "Remove the custom footer",
		Execute: func(_ string, ctx ext.Context) (string, error) {
			ctx.RemoveFooter()
			return "Footer removed.", nil
		},
	})

	// /footer-on — restore the custom footer.
	api.RegisterCommand(ext.CommandDef{
		Name:        "footer-on",
		Description: "Restore the custom footer",
		Execute: func(_ string, ctx ext.Context) (string, error) {
			ctx.SetFooter(ext.HeaderFooterConfig{
				Content: ext.WidgetContent{
					Text: fmt.Sprintf("Turns: %d  |  %s", turnCount, time.Now().Format("15:04:05")),
				},
				Style: ext.WidgetStyle{
					BorderColor: "#a6e3a1",
				},
			})
			return "Footer restored.", nil
		},
	})

	// Clean up on shutdown.
	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		ctx.RemoveHeader()
		ctx.RemoveFooter()
	})
}
