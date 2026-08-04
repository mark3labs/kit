//go:build ignore

// Kit "Ask User Question" extension
//
// Gives the agent a claude-code style `ask_question` tool: the model can stop
// and ask the user a multiple-choice question, rendered as a modal popup
// docked above the composer. Keyboard driven:
//
//	j / down    move down          k / up        move up
//	space       pick / toggle       enter         submit
//	1-9         jump to option      g / G         first / last
//	a / n       all / none (multi-select only)
//	o           write a custom answer (when allow_other is set)
//	esc / q     cancel
//
// While the popup is open every keystroke is swallowed by an editor
// interceptor, so it behaves as a true modal — nothing leaks into the
// composer. Note that Kit consumes `esc` itself while a turn is running
// (it means "cancel the turn"), so `q` is the dependable dismiss key.
//
// Colors follow the active theme via ctx.GetTheme(), read per frame so a
// /theme switch repaints the popup live.
//
// Install:
//
//	kit -e examples/extensions/ask-question.go
//
// or copy it to ~/.config/kit/extensions/ to load it automatically.
//
// Try it without an LLM:  /ask-demo
//
// YAEGI NOTE: every helper is declared ABOVE the code that references it.
// A bare reference to a function declared later in the file silently yields
// zero values.
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	ext "kit/ext"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type askOption struct {
	Label string `json:"label"`
	Desc  string `json:"description"`
}

type askQuestion struct {
	Header      string      `json:"header"`
	Question    string      `json:"question"`
	Options     []askOption `json:"options"`
	MultiSelect bool        `json:"multi_select"`
	AllowOther  bool        `json:"allow_other"`
}

type askAnswer struct {
	Values    []string
	FreeText  string
	Cancelled bool
}

type askState struct {
	q       askQuestion
	index   int // 1-based question number
	total   int
	cursor  int
	checked []bool

	typing bool   // free-text entry mode
	buf    string // free-text buffer

	// ctx is held so the render path can read the live theme every frame,
	// which is what makes a mid-prompt /theme switch repaint correctly.
	ctx  ext.Context
	done chan askAnswer
	// stop closes when the modal is torn down, retiring the theme watcher.
	stop chan struct{}
}

// ---------------------------------------------------------------------------
// Package state
// ---------------------------------------------------------------------------

var (
	askMu     sync.Mutex
	askActive *askState

	ctxMu    sync.Mutex
	hostCtx  ext.Context
	hasHost  bool
	widgetID = "ask-question"
)

// ---------------------------------------------------------------------------
// Theme-driven painting
//
// Widget Render output is used verbatim by Kit, so nothing styles it for us.
// ctx.GetTheme() reports the active theme's colors as hex, and ThemeColors.ANSI
// turns them into escape sequences. An unset color degrades to plain text
// rather than leaking escapes, so a partial theme is safe.
// ---------------------------------------------------------------------------

// runeLen returns the printable width of s ignoring ANSI escapes.
func runeLen(s string) int {
	n := 0
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			// Truecolor escapes end at 'm'; that is the only form emitted here.
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// wrapText breaks s into lines no wider than width columns.
func wrapText(s string, width int) []string {
	if width < 12 {
		width = 12
	}
	out := []string{}
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := ""
		for _, w := range words {
			switch {
			case line == "":
				line = w
			case len([]rune(line))+1+len([]rune(w)) <= width:
				line += " " + w
			default:
				out = append(out, line)
				line = w
			}
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func padRight(s string, w int) string {
	d := w - runeLen(s)
	if d <= 0 {
		return s
	}
	return s + strings.Repeat(" ", d)
}

func truncTo(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return ""
	}
	return string(r[:w-1]) + "…"
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// renderAsk draws the popup body. Runs on every frame, so it must be cheap.
//
// The widget already draws a left border in the theme's accent, so nothing
// here adds a second vertical rule — the title is distinguished by weight and
// color instead. width is the content width with the border already subtracted.
func renderAsk(width int) string {
	askMu.Lock()
	st := askActive
	askMu.Unlock()
	if st == nil {
		return ""
	}
	if width < 24 {
		width = 24
	}

	// Live read: a /theme switch mid-prompt repaints in the new colors.
	th := st.ctx.GetTheme()

	lines := []string{}

	// ---- title row -------------------------------------------------------
	head := strings.TrimSpace(st.q.Header)
	if head == "" {
		head = "Question"
	}
	title := th.ANSIBold(th.Accent, truncTo(head, width-8))
	if st.total > 1 {
		counter := fmt.Sprintf("%d/%d", st.index, st.total)
		// Leave one column spare: a row that exactly fills the content width
		// wraps, costing a blank line of scrollback.
		gap := width - runeLen(title) - len(counter) - 1
		if gap < 1 {
			gap = 1
		}
		title += strings.Repeat(" ", gap) + th.ANSI(th.VeryMuted, counter)
	}
	lines = append(lines, title)

	// ---- question --------------------------------------------------------
	for _, l := range wrapText(st.q.Question, width) {
		lines = append(lines, th.ANSI(th.Text, l))
	}
	lines = append(lines, "")

	// ---- options ---------------------------------------------------------
	labelW := 0
	for _, o := range st.q.Options {
		if n := len([]rune(o.Label)); n > labelW {
			labelW = n
		}
	}
	if labelW > 28 {
		labelW = 28
	}
	// Cursor (2) + box (3) + space (1) + label, then a gap before the note.
	descCol := 2 + 3 + 1 + labelW + 2

	for i, o := range st.q.Options {
		sel := i == st.cursor && !st.typing
		on := st.checked[i]

		box := "( )"
		if st.q.MultiSelect {
			box = "[ ]"
			if on {
				box = "[x]"
			}
		} else if on {
			box = "(o)"
		}
		if on {
			box = th.ANSI(th.Success, box)
		} else {
			box = th.ANSI(th.Muted, box)
		}

		cur := "  "
		label := truncTo(o.Label, labelW)
		if sel {
			cur = th.ANSIBold(th.Accent, "> ")
			label = th.ANSIBold(th.Accent, padRight(label, labelW))
		} else {
			label = th.ANSI(th.Text, padRight(label, labelW))
		}

		row := cur + box + " " + label
		desc := strings.TrimSpace(o.Desc)
		if desc == "" {
			lines = append(lines, row)
			continue
		}
		if room := width - descCol; room >= 8 {
			lines = append(lines, row+"  "+th.ANSI(th.Muted, truncTo(desc, room)))
		} else {
			lines = append(lines, row)
			for _, l := range wrapText(desc, width-4) {
				lines = append(lines, "    "+th.ANSI(th.Muted, l))
			}
		}
	}

	// ---- "other" row -----------------------------------------------------
	if st.q.AllowOther {
		sel := st.cursor == len(st.q.Options) && !st.typing
		cur := "  "
		label := "Other — write my own answer"
		if sel {
			cur = th.ANSIBold(th.Accent, "> ")
			label = th.ANSIBold(th.Warning, label)
		} else {
			label = th.ANSI(th.Warning, label)
		}
		lines = append(lines, cur+th.ANSI(th.Muted, " ✎ ")+label)
	}

	// ---- free-text editor ------------------------------------------------
	if st.typing {
		lines = append(lines, "")
		lines = append(lines,
			th.ANSI(th.Warning, "✎ ")+
				th.ANSI(th.Text, st.buf)+
				th.ANSI(th.Accent, "█"))
	}

	// ---- hints -----------------------------------------------------------
	lines = append(lines, "")
	hint := ""
	if st.typing {
		hint = "type your answer · enter submit · esc back"
	} else if st.q.MultiSelect {
		n := 0
		for _, c := range st.checked {
			if c {
				n++
			}
		}
		hint = fmt.Sprintf("%d selected · space toggle · a all · n none · enter submit · esc/q cancel", n)
	} else {
		hint = "j/k ↑↓ move · space/enter pick · 1-9 jump · esc/q cancel"
	}
	lines = append(lines, th.ANSI(th.VeryMuted, truncTo(hint, width)))

	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Key handling
// ---------------------------------------------------------------------------

// finish delivers the answer and tears down the modal state. Must be called
// with askMu held by the caller's logic released — it locks internally.
func finish(st *askState, ans askAnswer) {
	askMu.Lock()
	if askActive == st {
		askActive = nil
	}
	askMu.Unlock()
	select {
	case st.done <- ans:
	default:
	}
}

// syncSingle keeps a single-select question's radio marker on the cursor, so
// plain j/k movement is enough to choose — space is only a confirmation.
func syncSingle(st *askState) {
	if st.q.MultiSelect || st.cursor >= len(st.checked) {
		return
	}
	for i := range st.checked {
		st.checked[i] = i == st.cursor
	}
}

func collectAnswer(st *askState) askAnswer {
	vals := []string{}
	if st.q.MultiSelect {
		for i, c := range st.checked {
			if c {
				vals = append(vals, st.q.Options[i].Label)
			}
		}
	} else {
		// Single select: the cursor is the answer.
		idx := st.cursor
		if idx >= len(st.q.Options) {
			idx = -1
			for i, c := range st.checked {
				if c {
					idx = i
				}
			}
		}
		if idx >= 0 && idx < len(st.q.Options) {
			vals = append(vals, st.q.Options[idx].Label)
		}
	}
	return askAnswer{Values: vals}
}

// handleAskKey processes one keypress while the modal is open.
// Runs synchronously on the TUI event loop — keep it fast.
func handleAskKey(key string, _ string) ext.EditorKeyAction {
	askMu.Lock()
	st := askActive
	askMu.Unlock()

	consumed := ext.EditorKeyAction{Type: ext.EditorKeyConsumed}
	if st == nil {
		return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
	}

	maxIdx := len(st.q.Options) - 1
	if st.q.AllowOther {
		maxIdx++
	}

	// ---- free-text mode --------------------------------------------------
	if st.typing {
		switch key {
		case "enter":
			txt := strings.TrimSpace(st.buf)
			if txt == "" {
				st.typing = false
				return consumed
			}
			finish(st, askAnswer{Values: []string{txt}, FreeText: txt})
			return consumed
		case "esc":
			st.typing = false
			st.buf = ""
			return consumed
		case "backspace":
			r := []rune(st.buf)
			if len(r) > 0 {
				st.buf = string(r[:len(r)-1])
			}
			return consumed
		case "ctrl+u":
			st.buf = ""
			return consumed
		case " ", "space":
			st.buf += " "
			return consumed
		}
		if len([]rune(key)) == 1 {
			st.buf += key
		}
		return consumed
	}

	// ---- list mode -------------------------------------------------------
	switch key {
	case "up", "k", "ctrl+p":
		if st.cursor > 0 {
			st.cursor--
		}
		syncSingle(st)
		return consumed
	case "down", "j", "ctrl+n", "tab":
		if st.cursor < maxIdx {
			st.cursor++
		}
		syncSingle(st)
		return consumed
	case "g", "home":
		st.cursor = 0
		syncSingle(st)
		return consumed
	case "G", "end":
		st.cursor = maxIdx
		syncSingle(st)
		return consumed
	case " ", "space":
		if st.q.AllowOther && st.cursor == len(st.q.Options) {
			st.typing = true
			return consumed
		}
		if st.q.MultiSelect {
			if st.cursor < len(st.checked) {
				st.checked[st.cursor] = !st.checked[st.cursor]
			}
			return consumed
		}
		// Single select: space picks the highlighted option and submits.
		syncSingle(st)
		finish(st, collectAnswer(st))
		return consumed
	case "a":
		if st.q.MultiSelect {
			for i := range st.checked {
				st.checked[i] = true
			}
		}
		return consumed
	case "n":
		if st.q.MultiSelect {
			for i := range st.checked {
				st.checked[i] = false
			}
		}
		return consumed
	case "o":
		if st.q.AllowOther {
			st.cursor = len(st.q.Options)
			st.typing = true
		}
		return consumed
	case "enter":
		if st.q.AllowOther && st.cursor == len(st.q.Options) {
			st.typing = true
			return consumed
		}
		finish(st, collectAnswer(st))
		return consumed
	case "esc", "q", "ctrl+d":
		// NOTE: while the agent is mid-turn Kit consumes "esc" itself (turn
		// cancel) before extensions see it, so "q" is the reliable dismiss.
		finish(st, askAnswer{Cancelled: true})
		return consumed
	}

	// Number keys jump straight to an option (and submit in single mode).
	if len(key) == 1 && key >= "1" && key <= "9" {
		i := int(key[0] - '1')
		if i < len(st.q.Options) {
			st.cursor = i
			if st.q.MultiSelect {
				st.checked[i] = !st.checked[i]
			} else {
				syncSingle(st)
				finish(st, collectAnswer(st))
			}
		}
		return consumed
	}

	// Swallow everything else — this is a modal.
	return consumed
}

// ---------------------------------------------------------------------------
// Modal driver
// ---------------------------------------------------------------------------

// setAskWidget installs, or refreshes, the popup widget.
//
// WidgetConfig.Style is captured when it is set, unlike Content.Render which
// runs per frame — so the border keeps whatever accent was current at install
// time. Re-installing is how it picks up a theme switch.
func setAskWidget(ctx ext.Context) {
	ctx.SetWidget(ext.WidgetConfig{
		ID:        widgetID,
		Placement: ext.WidgetAbove,
		Priority:  -100,
		Content: ext.WidgetContent{
			Render: func(width int) string { return renderAsk(width) },
		},
		Style: ext.WidgetStyle{BorderColor: ctx.GetTheme().Accent},
	})
}

// watchTheme re-installs the widget when the accent changes, so the border
// tracks a /theme switch made while the modal is open. It polls rather than
// subscribing because there is no extension-facing theme-change event; at
// 400ms the cost is a hex-string compare per tick.
func watchTheme(ctx ext.Context, stop chan struct{}) {
	accent := ctx.GetTheme().Accent
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if cur := ctx.GetTheme().Accent; cur != accent {
				accent = cur
				setAskWidget(ctx)
			}
		}
	}
}

// ask shows the popup for one question and blocks until the user answers.
func ask(ctx ext.Context, q askQuestion, index, total int) askAnswer {
	if len(q.Options) == 0 && !q.AllowOther {
		return askAnswer{Cancelled: true}
	}

	st := &askState{
		q:       q,
		index:   index,
		total:   total,
		checked: make([]bool, len(q.Options)),
		ctx:     ctx,
		done:    make(chan askAnswer, 1),
		stop:    make(chan struct{}),
	}
	if !q.MultiSelect && len(q.Options) > 0 {
		st.checked[0] = true
	}

	askMu.Lock()
	if askActive != nil {
		askMu.Unlock()
		return askAnswer{Cancelled: true}
	}
	askActive = st
	askMu.Unlock()

	setAskWidget(ctx)
	go watchTheme(ctx, st.stop)

	ctx.SetEditor(ext.EditorConfig{
		HandleKey: func(key string, text string) ext.EditorKeyAction {
			return handleAskKey(key, text)
		},
		Render: func(width int, defaultContent string) string {
			askMu.Lock()
			open := askActive != nil
			askMu.Unlock()
			if !open {
				return defaultContent
			}
			th := ctx.GetTheme()
			return th.ANSI(th.VeryMuted, "  waiting for your answer…")
		},
	})

	ans := <-st.done
	close(st.stop)

	askMu.Lock()
	if askActive == st {
		askActive = nil
	}
	askMu.Unlock()

	ctx.ResetEditor()
	ctx.RemoveWidget(widgetID)
	return ans
}

func formatAnswer(q askQuestion, a askAnswer) string {
	if a.Cancelled {
		return "(cancelled by user)"
	}
	if a.FreeText != "" {
		return "custom answer: " + a.FreeText
	}
	if len(a.Values) == 0 {
		return "(no selection)"
	}
	return strings.Join(a.Values, ", ")
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	// Tools receive no Context, so stash the most recent one.
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctxMu.Lock()
		hostCtx, hasHost = ctx, true
		ctxMu.Unlock()
	})
	api.OnAgentStart(func(_ ext.AgentStartEvent, ctx ext.Context) {
		ctxMu.Lock()
		hostCtx, hasHost = ctx, true
		ctxMu.Unlock()
	})

	// Make sure a stray modal never survives a turn that was cancelled.
	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		askMu.Lock()
		st := askActive
		askMu.Unlock()
		if st != nil {
			finish(st, askAnswer{Cancelled: true})
			ctx.ResetEditor()
			ctx.RemoveWidget(widgetID)
		}
	})

	api.RegisterTool(ext.ToolDef{
		Name: "ask_question",
		Description: `Ask the user one or more multiple-choice questions and wait for their answer.

Use this when you need a decision only the user can make: an ambiguous
requirement, a choice between valid approaches, a missing preference. Do NOT
use it for things you can determine yourself by reading the codebase.

Each question is shown as a keyboard-driven popup. Provide 2-4 concrete,
mutually exclusive options with short labels and a one-line description each.
Set multi_select when several answers may apply. Set allow_other to let the
user type a free-form answer instead of picking an option.`,
		Parameters: `{
  "type": "object",
  "properties": {
    "questions": {
      "type": "array",
      "description": "Questions to ask, presented one at a time (max 4).",
      "items": {
        "type": "object",
        "properties": {
          "header": {
            "type": "string",
            "description": "Very short topic label shown in the popup title, e.g. 'Database'."
          },
          "question": {
            "type": "string",
            "description": "The question to ask the user."
          },
          "options": {
            "type": "array",
            "description": "The choices offered to the user (2-4 recommended).",
            "items": {
              "type": "object",
              "properties": {
                "label": {"type": "string", "description": "Short option label."},
                "description": {"type": "string", "description": "One-line explanation of the option."}
              },
              "required": ["label"]
            }
          },
          "multi_select": {
            "type": "boolean",
            "description": "Allow selecting several options. Default false."
          },
          "allow_other": {
            "type": "boolean",
            "description": "Offer an 'Other' entry so the user can type a custom answer. Default false."
          }
        },
        "required": ["question", "options"]
      }
    }
  },
  "required": ["questions"]
}`,
		Execute: func(input string) (string, error) {
			var params struct {
				Questions []askQuestion `json:"questions"`
				// Single-question convenience form.
				Question    string      `json:"question"`
				Header      string      `json:"header"`
				Options     []askOption `json:"options"`
				MultiSelect bool        `json:"multi_select"`
				AllowOther  bool        `json:"allow_other"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", fmt.Errorf("invalid parameters: %w", err)
			}

			qs := params.Questions
			if len(qs) == 0 && strings.TrimSpace(params.Question) != "" {
				qs = []askQuestion{{
					Header:      params.Header,
					Question:    params.Question,
					Options:     params.Options,
					MultiSelect: params.MultiSelect,
					AllowOther:  params.AllowOther,
				}}
			}
			if len(qs) == 0 {
				return "", fmt.Errorf("at least one question with options is required")
			}
			if len(qs) > 4 {
				qs = qs[:4]
			}

			ctxMu.Lock()
			ctx := hostCtx
			ok := hasHost
			ctxMu.Unlock()
			if !ok || !ctx.Interactive {
				return "", fmt.Errorf("ask_question requires an interactive session")
			}

			var out strings.Builder
			out.WriteString("The user answered:\n")
			for i, q := range qs {
				a := ask(ctx, q, i+1, len(qs))
				fmt.Fprintf(&out, "\nQ: %s\nA: %s\n", q.Question, formatAnswer(q, a))
				if a.Cancelled {
					out.WriteString("\nThe user dismissed the remaining questions. " +
						"Stop and ask them how they would like to proceed.\n")
					return out.String(), nil
				}
			}
			return out.String(), nil
		},
	})

	// Pretty tool-call rendering.
	api.RegisterToolRenderer(ext.ToolRenderConfig{
		ToolName:    "ask_question",
		DisplayName: "Ask User",
		RenderHeader: func(toolArgs string, width int) string {
			var p struct {
				Questions []struct {
					Header   string `json:"header"`
					Question string `json:"question"`
				} `json:"questions"`
				Question string `json:"question"`
			}
			_ = json.Unmarshal([]byte(toolArgs), &p)
			if len(p.Questions) > 0 {
				q := p.Questions[0].Question
				if q == "" {
					q = p.Questions[0].Header
				}
				if len(p.Questions) > 1 {
					return fmt.Sprintf("%s (+%d more)", q, len(p.Questions)-1)
				}
				return q
			}
			if p.Question != "" {
				return p.Question
			}
			return "asking the user…"
		},
	})

	// ---- demo command, exercises the popup without an LLM ----------------
	api.RegisterCommand(ext.CommandDef{
		Name:        "ask-demo",
		Description: "Demo the ask_question popup (single, multi, free-text)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			ctxMu.Lock()
			hostCtx, hasHost = ctx, true
			ctxMu.Unlock()

			qs := []askQuestion{
				{
					Header:   "Database",
					Question: "Which datastore should the new service use?",
					Options: []askOption{
						{Label: "PostgreSQL", Desc: "relational, strong consistency"},
						{Label: "SQLite", Desc: "zero-ops, single file"},
						{Label: "Redis", Desc: "in-memory, cache-first"},
					},
					AllowOther: true,
				},
				{
					Header:      "Scope",
					Question:    "Which parts should I refactor in this pass?",
					MultiSelect: true,
					Options: []askOption{
						{Label: "handlers", Desc: "HTTP layer"},
						{Label: "storage", Desc: "database access"},
						{Label: "tests", Desc: "table-driven rewrite"},
						{Label: "docs", Desc: "README + godoc"},
					},
				},
			}

			var out strings.Builder
			for i, q := range qs {
				a := ask(ctx, q, i+1, len(qs))
				if a.Cancelled {
					out.WriteString(fmt.Sprintf("Q%d cancelled.\n", i+1))
					break
				}
				fmt.Fprintf(&out, "Q%d %s -> %s\n", i+1, q.Header, formatAnswer(q, a))
			}
			return strings.TrimSpace(out.String()), nil
		},
	})
}
