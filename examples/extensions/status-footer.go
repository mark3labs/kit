//go:build ignore

package main

// status-footer replaces Kit's built-in status bar with a configurable,
// single-line footer showing mode, model, context usage, cache hit rate, cost
// (with prompt-cache savings), a clock, and per-turn timing.
//
// It demonstrates composing several extension APIs into a piece of persistent
// chrome: SetUIVisibility to reclaim the built-in bar, SetFooter for the
// replacement line, GetContextStats/GetSessionUsage/GetModelCapabilities for
// data, and a slash command for configuration.
//
// Usage: kit -e examples/extensions/status-footer.go
//
//	/footer                      show current state
//	/footer on|off|toggle        enable/disable
//	/footer show|hide <field>    toggle one field
//	/footer fields a,b,c         set the visible set
//	/footer reset                restore defaults
//
// Fields: mode, model, context, bar, cache, cost, clock, timer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"kit/ext"
)

// allFields is the canonical display order.
var allFields = []string{"mode", "model", "context", "bar", "cache", "cost", "clock", "timer"}

// dropOrder lists fields to shed when the line does not fit, least important
// first. The model name and context gauge are the last to go because they are
// the two facts that are not recoverable from anywhere else on screen.
var dropOrder = []string{"clock", "cache", "timer", "cost", "mode", "bar", "context", "model"}

const sep = " · "

var (
	mu      sync.Mutex
	enabled = true
	visible = map[string]bool{}

	turnStart    time.Time
	lastTurnDur  time.Duration
	totalTurnDur time.Duration

	cachedWidth = 80
)

// ---------------------------------------------------------------------------
// Persistence
//
// Extensions have no API for global (non-session) preferences: SetState is
// session-scoped and SetOption is a runtime-only override. So this keeps its
// own small JSON file alongside Kit's config.
// ---------------------------------------------------------------------------

type prefs struct {
	Enabled bool     `json:"enabled"`
	Fields  []string `json:"fields"`
}

func prefsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "kit", "status-footer.json")
}

func loadPrefs() {
	for _, f := range allFields {
		visible[f] = true
	}
	path := prefsPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // first run: defaults
	}
	var p prefs
	if json.Unmarshal(data, &p) != nil {
		return
	}
	enabled = p.Enabled
	// A persisted empty selection is meaningful (user hid everything), so
	// only a nil slice falls back to defaults.
	if p.Fields != nil {
		for _, f := range allFields {
			visible[f] = false
		}
		for _, f := range p.Fields {
			if _, ok := visible[f]; ok {
				visible[f] = true
			}
		}
	}
}

func savePrefs() error {
	path := prefsPath()
	if path == "" {
		return fmt.Errorf("cannot resolve config dir")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var on []string
	for _, f := range allFields {
		if visible[f] {
			on = append(on, f)
		}
	}
	if on == nil {
		on = []string{} // distinguish "none" from "unset" on reload
	}
	data, err := json.Marshal(prefs{Enabled: enabled, Fields: on})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ---------------------------------------------------------------------------
// Width handling
//
// The footer receives a plain string and Kit renders it at terminal width, so
// an over-long line wraps and silently steals a scrollback row. Extensions are
// not handed the width, so query the tty directly and truncate ourselves.
// ---------------------------------------------------------------------------

func detectWidth() int {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return cachedWidth
	}
	defer tty.Close()

	cmd := exec.Command("stty", "size")
	cmd.Stdin = tty
	out, err := cmd.Output()
	if err != nil {
		return cachedWidth
	}
	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		return cachedWidth
	}
	cols, err := strconv.Atoi(parts[1])
	if err != nil || cols <= 0 {
		return cachedWidth
	}
	return cols
}

// dispWidth measures rendered columns. Emoji and CJK occupy two cells, so
// counting runes (let alone bytes) would under-measure and reintroduce wrap.
func dispWidth(s string) int {
	w := 0
	for _, r := range s {
		switch {
		case r >= 0x1F300 && r <= 0x1FAFF, // emoji & pictographs
			r >= 0x2600 && r <= 0x27BF, // misc symbols/dingbats
			r >= 0x1100 && r <= 0x115F, // hangul jamo
			r >= 0x2E80 && r <= 0xA4CF, // CJK
			r >= 0xAC00 && r <= 0xD7A3, // hangul syllables
			r >= 0xF900 && r <= 0xFAFF,
			r >= 0xFF00 && r <= 0xFF60:
			w += 2
		default:
			w++
		}
	}
	return w
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if dispWidth(s) <= max {
		return s
	}
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := dispWidth(string(r))
		if w+rw > max-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	b.WriteRune('…')
	return b.String()
}

// ---------------------------------------------------------------------------
// Field rendering
// ---------------------------------------------------------------------------

func fmtTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return strconv.Itoa(n)
	}
}

func shortModel(m string) string {
	if i := strings.LastIndex(m, "/"); i >= 0 {
		return m[i+1:]
	}
	return m
}

// buildFields renders each visible field to a string, keyed by name.
func buildFields(ctx ext.Context) map[string]string {
	stats := ctx.GetContextStats()
	usage := ctx.GetSessionUsage()
	caps, _ := ctx.GetModelCapabilities("") // "" = current model
	p := caps.Pricing

	out := map[string]string{}

	if visible["mode"] {
		out["mode"] = "[KIT]"
	}
	if visible["model"] {
		if m := shortModel(ctx.Model); m != "" {
			out["model"] = m
		}
	}
	if visible["context"] {
		out["context"] = "ctx " + fmtTokens(stats.EstimatedTokens)
	}
	if visible["bar"] {
		pct := stats.UsagePercent * 100
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
		filled := int(math.Round(pct / 10))
		if filled > 10 {
			filled = 10
		}
		out["bar"] = fmt.Sprintf("[%s%s] %.0f%%",
			strings.Repeat("█", filled), strings.Repeat("░", 10-filled), pct)
	}
	if visible["cache"] {
		total := usage.TotalInputTokens + usage.TotalCacheReadTokens
		var ratio float64
		if total > 0 {
			ratio = float64(usage.TotalCacheReadTokens) / float64(total) * 100
		}
		out["cache"] = fmt.Sprintf("cache %s (%.0f%%)", fmtTokens(usage.TotalCacheReadTokens), ratio)
	}
	if visible["cost"] {
		switch {
		case usage.IsOAuth:
			// Subscription credentials are not billed per token, so a dollar
			// figure would be meaningless rather than merely zero.
			out["cost"] = "$0.00 sub"
		case !p.Known:
			// Local models and custom endpoints have no registry pricing;
			// rendering "$0.00000" would imply a real, free request.
			out["cost"] = "cost n/a"
		default:
			s := fmt.Sprintf("$%.4f", usage.TotalCost)
			if p.HasCacheRead {
				saved := float64(usage.TotalCacheReadTokens) * (p.Input - p.CacheRead) / 1e6
				if retail := usage.TotalCost + saved; retail > 0 && saved > 0 {
					s += fmt.Sprintf(" (saved ~%.0f%%)", saved/retail*100)
				}
			}
			out["cost"] = s
		}
	}
	if visible["clock"] {
		out["clock"] = "[" + strings.ToLower(time.Now().Format("3:04pm")) + "]"
	}
	if visible["timer"] {
		d, tot := lastTurnDur, totalTurnDur
		if !turnStart.IsZero() {
			d = time.Since(turnStart)
			tot = totalTurnDur + d
		}
		if d > 0 || tot > 0 {
			out["timer"] = fmt.Sprintf("%s/%s", compactDur(d), compactDur(tot))
		}
	}
	return out
}

func compactDur(d time.Duration) string {
	s := int(d.Seconds())
	if s < 60 {
		return strconv.Itoa(s) + "s"
	}
	return fmt.Sprintf("%dm%02ds", s/60, s%60)
}

// assemble joins the visible fields, progressively dropping the least
// important ones until the line fits, then hard-truncating as a last resort.
func assemble(parts map[string]string, width int) string {
	dropped := map[string]bool{}

	join := func() string {
		var ordered []string
		for _, f := range allFields {
			if v, ok := parts[f]; ok && !dropped[f] && v != "" {
				ordered = append(ordered, v)
			}
		}
		return strings.Join(ordered, sep)
	}

	// Kit adds one column of left padding even with NoBorder.
	budget := width - 2
	line := join()
	for i := 0; dispWidth(line) > budget && i < len(dropOrder); i++ {
		dropped[dropOrder[i]] = true
		line = join()
	}
	return truncate(line, budget)
}

func render(ctx ext.Context) {
	mu.Lock()
	if !enabled {
		mu.Unlock()
		ctx.RemoveFooter()
		return
	}
	cachedWidth = detectWidth()
	text := assemble(buildFields(ctx), cachedWidth)
	mu.Unlock()

	ctx.SetFooter(ext.HeaderFooterConfig{
		Content: ext.WidgetContent{Text: text},
		Style:   ext.WidgetStyle{NoBorder: true},
	})
}

// ---------------------------------------------------------------------------
// Command
// ---------------------------------------------------------------------------

func statusLine() string {
	var on []string
	for _, f := range allFields {
		if visible[f] {
			on = append(on, f)
		}
	}
	state := "on"
	if !enabled {
		state = "off"
	}
	if len(on) == 0 {
		return fmt.Sprintf("footer %s · no fields selected", state)
	}
	return fmt.Sprintf("footer %s · fields: %s", state, strings.Join(on, ", "))
}

func known(f string) bool {
	_, ok := visible[f]
	return ok
}

func Init(api ext.API) {
	loadPrefs()

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		// Reclaim the row used by the built-in status bar.
		ctx.SetUIVisibility(ext.UIVisibility{HideStatusBar: true})
		render(ctx)

		// The clock and live turn timer need their own tick; Kit has no
		// periodic extension event. This also picks up terminal resizes,
		// since the width is re-read on each render.
		go func() {
			t := time.NewTicker(time.Second)
			defer t.Stop()
			for range t.C {
				render(ctx)
			}
		}()
	})

	api.OnAgentStart(func(_ ext.AgentStartEvent, ctx ext.Context) {
		mu.Lock()
		turnStart = time.Now()
		mu.Unlock()
		render(ctx)
	})

	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		mu.Lock()
		if !turnStart.IsZero() {
			lastTurnDur = time.Since(turnStart)
			totalTurnDur += lastTurnDur
			turnStart = time.Time{}
		}
		mu.Unlock()
		render(ctx)
	})

	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		ctx.RemoveFooter()
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "footer",
		Description: "Configure the status footer (on/off, show/hide <field>, fields a,b,c, reset)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			fields := strings.Fields(strings.TrimSpace(args))

			mu.Lock()
			var msg string
			persist := true

			switch {
			case len(fields) == 0:
				msg = statusLine()
				persist = false

			case fields[0] == "on":
				enabled = true
				msg = statusLine()

			case fields[0] == "off":
				enabled = false
				msg = statusLine()

			case fields[0] == "toggle":
				enabled = !enabled
				msg = statusLine()

			case fields[0] == "reset":
				enabled = true
				for _, f := range allFields {
					visible[f] = true
				}
				msg = "footer reset · " + statusLine()

			case fields[0] == "fields" && len(fields) > 1:
				sel := strings.Split(strings.Join(fields[1:], ""), ",")
				var bad []string
				next := map[string]bool{}
				for _, f := range sel {
					f = strings.TrimSpace(f)
					if f == "" {
						continue
					}
					if !known(f) {
						bad = append(bad, f)
						continue
					}
					next[f] = true
				}
				if len(bad) > 0 {
					mu.Unlock()
					return "", fmt.Errorf("unknown field(s): %s (valid: %s)",
						strings.Join(bad, ", "), strings.Join(allFields, ", "))
				}
				for _, f := range allFields {
					visible[f] = next[f]
				}
				msg = statusLine()

			case (fields[0] == "show" || fields[0] == "hide") && len(fields) > 1:
				f := fields[1]
				if !known(f) {
					mu.Unlock()
					return "", fmt.Errorf("unknown field %q (valid: %s)", f, strings.Join(allFields, ", "))
				}
				visible[f] = fields[0] == "show"
				msg = statusLine()

			case known(fields[0]):
				// "/footer cost" toggles that field.
				visible[fields[0]] = !visible[fields[0]]
				msg = statusLine()

			default:
				mu.Unlock()
				return "", fmt.Errorf("usage: /footer [on|off|toggle|reset|fields <a,b>|show <f>|hide <f>|<field>]")
			}
			mu.Unlock()

			if persist {
				if err := savePrefs(); err != nil {
					// Report rather than swallow: the user would otherwise
					// think the change survived restart.
					ctx.PrintError("footer: could not save preferences: " + err.Error())
				}
			}
			render(ctx)
			ctx.PrintInfo(msg)
			return "", nil
		},
		Complete: func(prefix string, ctx ext.Context) []string {
			opts := append([]string{"on", "off", "toggle", "reset", "fields", "show", "hide"}, allFields...)
			var out []string
			for _, o := range opts {
				if prefix == "" || strings.HasPrefix(o, strings.ToLower(prefix)) {
					out = append(out, o)
				}
			}
			sort.Strings(out)
			return out
		},
	})
}
