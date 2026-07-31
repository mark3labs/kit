//go:build ignore

// Kit Arbitrary UI Demo
//
// Demonstrates the capabilities added while addressing the MVM investigation
// findings:
//
//  1. WidgetContent.Render  — arbitrary per-frame UI, output used verbatim
//  2. WidgetContent.Markdown — previously declared but silently ignored
//  3. ToolRenderConfig.BorderColor / Background — previously dead fields
//  4. ctx.PromptMultiSelect — previously documented but never wired
//
// NOTE ON YAEGI: every helper function is declared ABOVE the code that
// references it. Referencing a func by name from inside a closure where the
// func is declared later in the file silently yields zero values.
package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	ext "kit/ext"
)

// ---------------------------------------------------------------------------
// State (package-level vars survive across event callbacks)
// ---------------------------------------------------------------------------

var (
	startedAt   = time.Now()
	toolCalls   = 0
	tokensIn    = 0
	tokensOut   = 0
	lastTool    = "—"
	history     = []float64{}
	showDash    = true
	panelStyles = []string{"gradient", "sparkline", "gauge"}
	activeStyle = 0
)

// ---------------------------------------------------------------------------
// ANSI helpers — extensions cannot import lipgloss, so we emit codes directly
// ---------------------------------------------------------------------------

func esc(code string) string { return "\033[" + code + "m" }

func reset() string { return esc("0") }

// fg256 returns a foreground color escape from the xterm-256 palette.
func fg256(n int) string { return fmt.Sprintf("\033[38;5;%dm", n) }

// bg256 returns a background color escape from the xterm-256 palette.
func bg256(n int) string { return fmt.Sprintf("\033[48;5;%dm", n) }

func bold(s string) string { return esc("1") + s + reset() }

func dim(s string) string { return esc("2") + s + reset() }

// colorize wraps s in a 256-color foreground.
func colorize(n int, s string) string { return fg256(n) + s + reset() }

// visLen returns the printable width of s, ignoring ANSI escape sequences.
// Kit measures widget height with lipgloss, but we need our own width math
// to build aligned box drawing.
func visLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// pad right-pads s with spaces to the given visible width.
func pad(s string, width int) string {
	if d := width - visLen(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// ---------------------------------------------------------------------------
// Drawing primitives
// ---------------------------------------------------------------------------

// gradientBar draws a horizontal bar whose color sweeps through the palette.
func gradientBar(width int, phase float64) string {
	if width < 1 {
		return ""
	}
	palette := []int{57, 63, 69, 75, 81, 87, 123, 159, 195}
	var b strings.Builder
	for i := 0; i < width; i++ {
		t := float64(i)/float64(width) + phase
		idx := int(math.Abs(math.Sin(t*math.Pi)) * float64(len(palette)-1))
		b.WriteString(fg256(palette[idx]) + "█")
	}
	b.WriteString(reset())
	return b.String()
}

// sparkline renders values as a unicode bar chart sized to width.
func sparkline(values []float64, width int) string {
	glyphs := []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}
	if width < 1 {
		return ""
	}
	if len(values) == 0 {
		return dim(strings.Repeat("▁", width))
	}

	start := 0
	if len(values) > width {
		start = len(values) - width
	}
	window := values[start:]

	hi := 0.0
	for _, v := range window {
		if v > hi {
			hi = v
		}
	}
	if hi == 0 {
		hi = 1
	}

	var b strings.Builder
	for _, v := range window {
		lvl := int((v / hi) * float64(len(glyphs)-1))
		if lvl < 0 {
			lvl = 0
		}
		color := 82
		switch {
		case lvl >= 6:
			color = 203
		case lvl >= 4:
			color = 214
		}
		b.WriteString(fg256(color) + glyphs[lvl])
	}
	b.WriteString(reset())
	return b.String()
}

// gauge draws a labelled progress meter.
func gauge(label string, frac float64, width int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	inner := width - visLen(label) - 8
	if inner < 1 {
		return label
	}
	filled := int(frac * float64(inner))

	color := 82
	switch {
	case frac > 0.85:
		color = 203
	case frac > 0.6:
		color = 214
	}

	bar := fg256(color) + strings.Repeat("━", filled) + reset() +
		dim(strings.Repeat("━", inner-filled))
	return fmt.Sprintf("%s %s %3.0f%%", dim(label), bar, frac*100)
}

// spinnerFrame returns an animated braille spinner glyph.
func spinnerFrame(t time.Time) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	return frames[int(t.UnixNano()/1e8)%len(frames)]
}

// ---------------------------------------------------------------------------
// The dashboard — a full arbitrary-UI widget drawn per frame
// ---------------------------------------------------------------------------

// renderDashboard draws a bordered panel sized exactly to the width Kit gives
// us. Kit takes this string verbatim, so every glyph and color is ours.
func renderDashboard(width int) string {
	if !showDash || width < 24 {
		return ""
	}

	now := time.Now()
	phase := float64(now.UnixNano()) / 1e9
	inner := width - 4

	title := bold(colorize(213, "◆ KIT LIVE")) + " " +
		dim("arbitrary-ui demo") + " " + colorize(120, spinnerFrame(now))
	uptime := dim(fmt.Sprintf("%s", now.Sub(startedAt).Truncate(time.Second)))

	head := pad(title, inner-visLen(uptime)) + uptime

	var body string
	switch panelStyles[activeStyle%len(panelStyles)] {
	case "gradient":
		body = gradientBar(inner, phase/3)
	case "sparkline":
		body = sparkline(history, inner)
	default:
		frac := math.Abs(math.Sin(phase / 2))
		body = gauge("load", frac, inner)
	}

	stats := fmt.Sprintf("%s %s   %s %s   %s %s",
		dim("tools"), colorize(117, fmt.Sprintf("%d", toolCalls)),
		dim("last"), colorize(150, lastTool),
		dim("tok"), colorize(180, fmt.Sprintf("%d/%d", tokensIn, tokensOut)),
	)

	hint := dim("/ui style · /ui toggle · /ui pick")

	edge := colorize(61, "─")
	top := colorize(61, "╭") + strings.Repeat(edge, width-2) + colorize(61, "╮")
	bot := colorize(61, "╰") + strings.Repeat(edge, width-2) + colorize(61, "╯")
	side := colorize(61, "│")

	row := func(s string) string {
		return side + " " + pad(s, inner) + " " + side
	}

	return strings.Join([]string{
		top,
		row(head),
		row(""),
		row(body),
		row(""),
		row(stats),
		row(hint),
		bot,
	}, "\n")
}

// ---------------------------------------------------------------------------
// Tool renderer — exercises the previously-dead BorderColor/Background fields
// ---------------------------------------------------------------------------

func renderPingHeader(toolArgs string, width int) string {
	return colorize(213, "◆ ") + dim("custom header from extension")
}

func renderPingBody(toolResult string, isError bool, width int) string {
	badge := bg256(57) + fg256(231) + " ARBITRARY " + reset()
	return badge + " " + colorize(159, toolResult) + "\n" +
		dim("↑ this block's stripe + background come from ToolRenderConfig")
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "ui_ping",
		Description: "Demo tool showing a custom tool renderer with border and background.",
		Execute: func(input string) (string, error) {
			toolCalls++
			lastTool = "ui_ping"
			history = append(history, float64(len(input)%17+3))
			return "pong: " + input, nil
		},
	})

	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:     "ui_ping",
		DisplayName:  "UI Ping",
		BorderColor:  "#c678dd",
		Background:   "#1b1b2b",
		RenderHeader: func(a string, w int) string { return renderPingHeader(a, w) },
		RenderBody:   func(r string, e bool, w int) string { return renderPingBody(r, e, w) },
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "ui",
		Description: "Control the demo dashboard: style | toggle | pick | md",
		Execute: func(args string, ctx ext.Context) (string, error) {
			switch strings.TrimSpace(args) {
			case "toggle":
				showDash = !showDash
				ctx.PrintInfo(fmt.Sprintf("dashboard: %v", showDash))
				return "", nil

			case "style":
				activeStyle++
				ctx.PrintInfo("panel style: " + panelStyles[activeStyle%len(panelStyles)])
				return "", nil

			case "md":
				ctx.SetHeader(ext.HeaderFooterConfig{
					Content: ext.WidgetContent{
						Markdown: true,
						Text: "# Markdown header\n\n" +
							"This is **bold**, *italic*, and `inline code` — " +
							"rendered because `WidgetContent.Markdown` is now honoured.\n\n" +
							"- previously this field was silently ignored\n" +
							"- now it routes through the markdown renderer",
					},
					Style: ext.WidgetStyle{BorderColor: "#89b4fa"},
				})
				ctx.PrintInfo("markdown header set (was a dead field before)")
				return "", nil

			case "pick":
				res := ctx.PromptMultiSelect(ext.PromptMultiSelectConfig{
					Message:         "Which panels should the dashboard cycle through?",
					Options:         []string{"gradient", "sparkline", "gauge"},
					DefaultSelected: []int{0, 1, 2},
				})
				if res.Cancelled {
					ctx.PrintInfo("multi-select cancelled")
					return "", nil
				}
				if len(res.Values) == 0 {
					ctx.PrintInfo("nothing selected — keeping current set")
					return "", nil
				}
				panelStyles = res.Values
				activeStyle = 0
				ctx.PrintInfo("panels: " + strings.Join(res.Values, ", ") +
					fmt.Sprintf("  (indices %v)", res.Indices))
				return "", nil

			default:
				ctx.PrintInfo("usage: /ui style | toggle | pick | md")
				return "", nil
			}
		},
	})

	// The dashboard itself: a per-frame Render callback. Kit uses whatever
	// string this returns verbatim, which is what makes it arbitrary UI.
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "uidemo:dashboard",
			Placement: ext.WidgetAbove,
			Priority:  10,
			Style:     ext.WidgetStyle{NoBorder: true},
			Content: ext.WidgetContent{
				// 15Hz: smooth enough for the spinner and gradient without
				// asking for the full 30fps clock.
				RefreshHz: 15,
				Render:    func(width int) string { return renderDashboard(width) },
			},
		})

		ctx.SetFooter(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{
				// The footer only reflects width, so it needs no clock of
				// its own; it repaints whenever anything else renders.
				Render: func(width int) string {
					left := colorize(141, "▌") + dim(" arbitrary-ui demo")
					right := dim(fmt.Sprintf("%d cols", width))
					return pad(left, width-visLen(right)) + right
				},
			},
			Style: ext.WidgetStyle{NoBorder: true},
		})
	})

	api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		toolCalls++
		lastTool = e.ToolName
		history = append(history, float64(len(e.ToolName)%13+2))
		if len(history) > 400 {
			history = history[len(history)-400:]
		}
		return nil
	})

	api.OnLLMUsage(func(e ext.LLMUsageEvent, ctx ext.Context) {
		tokensIn += e.InputTokens
		tokensOut += e.OutputTokens
		history = append(history, float64(e.OutputTokens%23+2))
		if len(history) > 400 {
			history = history[len(history)-400:]
		}
	})
}
