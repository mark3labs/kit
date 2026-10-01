package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/codemode"
)

// CodeModeToolName is the name of the code mode core tool.
const CodeModeToolName = "codemode"

// CodeModeConfig configures the code mode tool.
type CodeModeConfig struct {
	// Limits bounds each script run. Zero fields use the defaults.
	Limits codemode.Limits
	// Policy decides which tools the model sees directly and which only
	// scripts can call.
	Policy codemode.Policy
	// CatalogBudget caps the bytes of the tool catalog in the tool
	// description. Zero uses 12000.
	CatalogBudget int
}

// WithCodeMode configures the code mode tool. Other tools ignore it.
func WithCodeMode(cfg CodeModeConfig) ToolOption {
	return func(c *ToolConfig) {
		c.CodeMode = &cfg
	}
}

// ToolSetObserver is implemented by tools that need the agent's live tool
// set. The agent calls ObserveToolSet with the full composed tool set
// (wrapped, so nested calls pass through extension and SDK hooks) before
// each step, and drops every tool for which ModelVisible returns false from
// the tools it sends to the model.
type ToolSetObserver interface {
	ObserveToolSet(tools []fantasy.AgentTool)
	ModelVisible(name string) bool
}

// CodeModeTool runs model-written JavaScript that calls other tools. See
// package codemode.
type CodeModeTool struct {
	cfg             CodeModeConfig
	store           *codemode.Store
	providerOptions fantasy.ProviderOptions

	mu          sync.RWMutex
	tools       map[string]fantasy.AgentTool
	catalog     *codemode.Catalog
	description string
	fingerprint string
}

var _ ToolSetObserver = (*CodeModeTool)(nil)

// NewCodeModeTool creates the code mode core tool.
func NewCodeModeTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	var cm CodeModeConfig
	if cfg.CodeMode != nil {
		cm = *cfg.CodeMode
	}
	cm.Limits = cm.Limits.WithDefaults()
	t := &CodeModeTool{cfg: cm, store: codemode.NewStore(), catalog: codemode.NewCatalog(nil)}
	t.description = t.buildDescription(t.catalog)
	return t
}

// Info implements fantasy.AgentTool. The description carries the live
// catalog, refreshed by ObserveToolSet.
func (t *CodeModeTool) Info() fantasy.ToolInfo {
	t.mu.RLock()
	desc := t.description
	t.mu.RUnlock()
	return fantasy.ToolInfo{
		Name:        CodeModeToolName,
		Description: desc,
		Parameters: map[string]any{
			"code": map[string]any{
				"type":        "string",
				"description": "JavaScript to run. It runs inside an async function: use await, and use return to send the result.",
			},
		},
		Required: []string{"code"},
	}
}

// ProviderOptions implements fantasy.AgentTool.
func (t *CodeModeTool) ProviderOptions() fantasy.ProviderOptions { return t.providerOptions }

// SetProviderOptions implements fantasy.AgentTool.
func (t *CodeModeTool) SetProviderOptions(o fantasy.ProviderOptions) { t.providerOptions = o }

// Store returns the store that keeps store()/load() values across runs.
func (t *CodeModeTool) Store() *codemode.Store { return t.store }

// ModelVisible implements ToolSetObserver.
func (t *CodeModeTool) ModelVisible(name string) bool {
	if name == CodeModeToolName {
		return true
	}
	return t.cfg.Policy.Resolve(name).VisibleToModel()
}

// ObserveToolSet implements ToolSetObserver. It rebuilds the catalog and the
// description when the tool set changed.
func (t *CodeModeTool) ObserveToolSet(all []fantasy.AgentTool) {
	byName := make(map[string]fantasy.AgentTool, len(all))
	specs := make([]codemode.ToolSpec, 0, len(all))
	var fp strings.Builder
	for _, tool := range all {
		info := tool.Info()
		if info.Name == CodeModeToolName {
			continue
		}
		byName[info.Name] = tool
		specs = append(specs, codemode.ToolSpec{
			Name:        info.Name,
			Description: info.Description,
			Parameters:  info.Parameters,
			Required:    info.Required,
			Exposure:    t.cfg.Policy.Resolve(info.Name),
		})
		fmt.Fprintf(&fp, "%s:%d:%d;", info.Name, len(info.Description), len(info.Parameters))
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.tools = byName
	if fp.String() == t.fingerprint {
		return
	}
	t.fingerprint = fp.String()
	t.catalog = codemode.NewCatalog(specs)
	t.description = t.buildDescription(t.catalog)
}

func (t *CodeModeTool) buildDescription(c *codemode.Catalog) string {
	lim := t.cfg.Limits
	var b strings.Builder
	b.WriteString(`Run a JavaScript program that calls other tools. Use it to chain dependent calls, run independent calls in parallel, loop over many items, and filter large results before they reach you. Only the script's return value, text() output and console logs reach you — nested tool results do not. Prefer it over many direct tool calls when you would otherwise make 3 or more calls or read large outputs only to extract a few facts.

Rules:
- The code runs inside an async function. Use await. Use return to send the result (strings as-is, other values as JSON).
- Call a tool as: const out = await tools.<name>({ ...args }). Tools from MCP servers are grouped: tools.<server>.<tool>(args). Every call takes one object argument and resolves to the tool's text output; use JSON.parse when the tool returns JSON.
- A failed tool call throws an Error with name "ToolError" (properties: tool, output). Catch it with try/catch, or use Promise.allSettled.
- Run independent calls together with Promise.all / Promise.allSettled.
- There is no filesystem, network, process, timer or module access except through tools.
`)
	fmt.Fprintf(&b, "- Limits per script: %s, %d tool calls (%d at once), %d KB output.\n",
		lim.Timeout, lim.MaxToolCalls, lim.MaxConcurrency, lim.MaxOutputBytes/1000)
	b.WriteString(`
Helpers: text(value) adds to the output; console.log/warn/error(...) add logs; call(name, args) calls a tool by its full name; searchTools(query, {limit, namespace}) and describeTool(name) find tools and show their full input schema; ALL_TOOLS lists every callable tool; store(key, value) and load(key) keep JSON values across scripts (store(key, undefined) deletes); sleep(ms) waits.

Example:
const files = (await tools.find({ pattern: "*_test.go" })).split("\n").filter(f => f.endsWith(".go"))
const results = await Promise.allSettled(files.slice(0, 20).map(f => tools.read({ path: f })))
return files.filter((f, i) => results[i].status === "fulfilled" && !results[i].value.includes("t.Parallel"))
`)
	catalog, omitted := c.RenderCatalog(t.cfg.CatalogBudget)
	if catalog == "" {
		b.WriteString("\nNo tools are available to scripts yet.")
		return b.String()
	}
	b.WriteString("\nCallable tools (short form = you also have it as a direct tool with the same arguments):\n")
	b.WriteString(catalog)
	if omitted > 0 {
		fmt.Fprintf(&b, "\n// … %d more tools are not listed. Find them with searchTools(\"keywords\").", omitted)
	}
	return b.String()
}

type codeModeArgs struct {
	Code string `json:"code"`
}

// CodeModeRunInfo is the structured metadata attached to each code mode
// result.
type CodeModeRunInfo struct {
	OK        bool            `json:"ok"`
	ErrorKind string          `json:"error_kind,omitempty"`
	WallTime  time.Duration   `json:"wall_time_ns"`
	Calls     []codemode.Call `json:"calls,omitempty"`
	FullPath  string          `json:"full_output_path,omitempty"`
}

// Run implements fantasy.AgentTool.
func (t *CodeModeTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args codeModeArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if strings.TrimSpace(args.Code) == "" {
		return fantasy.NewTextErrorResponse("code is required"), nil
	}

	t.mu.RLock()
	tools, catalog := t.tools, t.catalog
	t.mu.RUnlock()

	// Progress goes out as one chunk per line, like shell output. All lines
	// use the stdout stream: the UI renders stderr after stdout, which
	// would break the order of the call log.
	progress := toolOutputCallbackFromContext(ctx)
	emit := func(text string) {
		if progress == nil {
			return
		}
		for line := range strings.SplitSeq(text, "\n") {
			progress(call.ID, CodeModeToolName, line, false)
		}
	}

	invoke := func(ctx context.Context, c codemode.Call) (codemode.InvokeResult, error) {
		tool, ok := tools[c.Name]
		if !ok {
			return codemode.InvokeResult{}, fmt.Errorf("tool %s is no longer available", c.Name)
		}
		// Nested calls must not stream into the transcript: the UI shows
		// one streaming block per top-level tool call. Progress lines come
		// from the observer below instead.
		nctx := ContextWithToolOutputCallback(ctx, nil)
		resp, err := tool.Run(nctx, fantasy.ToolCall{ID: c.ID, Name: c.Name, Input: c.Args})
		if err != nil {
			return codemode.InvokeResult{}, err
		}
		text := unwrapMCPResult(resp.Content)
		if len(resp.Data) > 0 {
			text = fmt.Sprintf("[%s result (%s, %d bytes) cannot be passed into a script; call %s directly to see it]",
				resp.Type, resp.MediaType, len(resp.Data), c.Name)
		}
		return codemode.InvokeResult{Text: text, IsError: resp.IsError}, nil
	}

	res := codemode.Run(ctx, codemode.Program{
		Code:         args.Code,
		Catalog:      catalog,
		Invoke:       invoke,
		Limits:       t.cfg.Limits,
		Store:        t.store,
		CallIDPrefix: call.ID,
		Observer: codemode.Observer{
			OnCallStart: func(c codemode.Call) {
				emit(strings.TrimSpace("▸ " + c.Path + " " + oneLineArgs(c.Args, 120)))
			},
			OnCallEnd: func(c codemode.Call) {
				switch c.Status {
				case codemode.CallOK:
					emit(fmt.Sprintf("✓ %s (%s)", c.Path, roundDuration(c.Duration)))
				default:
					emit(fmt.Sprintf("✗ %s (%s): %s", c.Path, roundDuration(c.Duration), firstLineOf(c.Error)))
				}
			},
			OnLog: emit,
		},
	})

	// A cancelled turn unwinds the agent loop, like every other tool.
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}

	text, fullPath := formatCodeModeResult(res, t.cfg.Limits.MaxOutputBytes)
	var resp fantasy.ToolResponse
	if res.OK {
		resp = fantasy.NewTextResponse(text)
	} else {
		resp = fantasy.NewTextErrorResponse(text)
	}
	info := CodeModeRunInfo{OK: res.OK, WallTime: res.WallTime, Calls: res.Calls, FullPath: fullPath}
	if res.Error != nil {
		info.ErrorKind = string(res.Error.Kind)
	}
	return fantasy.WithResponseMetadata(resp, map[string]any{"codemode": info}), nil
}

// formatCodeModeResult renders the model-facing text. When logs plus output
// exceed maxBytes, the full text goes to a temporary file and the result
// names that file.
func formatCodeModeResult(res codemode.Result, maxBytes int) (string, string) {
	var head strings.Builder
	if res.OK {
		fmt.Fprintf(&head, "Script completed in %s.", roundDuration(res.WallTime))
	} else {
		d := res.Error
		fmt.Fprintf(&head, "Script failed after %s: %s", roundDuration(res.WallTime), d.Kind)
		if d.Line > 0 {
			fmt.Fprintf(&head, " at line %d, column %d", d.Line, d.Column)
		}
		fmt.Fprintf(&head, "\n%s", d.Message)
		if d.Hint != "" {
			fmt.Fprintf(&head, "\nHint: %s", d.Hint)
		}
	}
	if s := summarizeCalls(res.Calls); s != "" {
		fmt.Fprintf(&head, "\nTool calls: %s", s)
	}

	var body strings.Builder
	if len(res.Logs) > 0 {
		body.WriteString("\n\nLogs:\n")
		body.WriteString(strings.Join(res.Logs, "\n"))
	}
	switch {
	case res.Output != "" && res.OK:
		body.WriteString("\n\nOutput:\n")
		body.WriteString(res.Output)
	case res.Output != "":
		body.WriteString("\n\nOutput before the failure:\n")
		body.WriteString(res.Output)
	case res.OK:
		body.WriteString("\n\n(no output: use return or text() to send results)")
	}
	if res.CaptureTruncated {
		body.WriteString("\n\n[the script printed more than the capture limit; the rest was dropped]")
	}

	full := body.String()
	if len(full) <= maxBytes {
		return head.String() + full, ""
	}
	path := ""
	if f, err := os.CreateTemp("", "kit-codemode-*.txt"); err == nil {
		_, werr := f.WriteString(strings.TrimLeft(full, "\n"))
		cerr := f.Close()
		if werr == nil && cerr == nil {
			path = f.Name()
		}
	}
	// Cut on a rune boundary.
	i := maxBytes
	for i > 0 && !utf8.RuneStart(full[i]) {
		i--
	}
	cut := full[:i]
	note := fmt.Sprintf("\n\n[output truncated: %d of %d bytes shown", len(cut), len(full))
	if path != "" {
		note += "; full output saved to " + path + " (use the read tool, or return less data)"
	}
	return head.String() + cut + note + "]", path
}

// summarizeCalls renders "read ×3, github.list_issues ×1 (1 failed)".
func summarizeCalls(calls []codemode.Call) string {
	if len(calls) == 0 {
		return ""
	}
	type agg struct {
		n, failed, cancelled int
		first                int
	}
	m := map[string]*agg{}
	for i, c := range calls {
		a, ok := m[c.Path]
		if !ok {
			a = &agg{first: i}
			m[c.Path] = a
		}
		a.n++
		switch c.Status {
		case codemode.CallError:
			a.failed++
		case codemode.CallCancelled:
			a.cancelled++
		}
	}
	paths := make([]string, 0, len(m))
	for p := range m {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool { return m[paths[i]].first < m[paths[j]].first })
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		a := m[p]
		s := fmt.Sprintf("%s ×%d", p, a.n)
		var notes []string
		if a.failed > 0 {
			notes = append(notes, fmt.Sprintf("%d failed", a.failed))
		}
		if a.cancelled > 0 {
			notes = append(notes, fmt.Sprintf("%d cancelled", a.cancelled))
		}
		if len(notes) > 0 {
			s += " (" + strings.Join(notes, ", ") + ")"
		}
		parts = append(parts, s)
	}
	return fmt.Sprintf("%d — %s", len(calls), strings.Join(parts, ", "))
}

// unwrapMCPResult turns the JSON-encoded MCP CallToolResult that MCP tools
// return into what a script wants: the text parts joined by newlines (the
// same text the model sees for a direct call), or the structured content as
// JSON when the server sent no text. Media parts become short placeholders.
// Text that is not an MCP result is returned unchanged.
func unwrapMCPResult(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{") || !strings.Contains(trimmed, `"content"`) {
		return s
	}
	var r struct {
		Content []struct {
			Type     string `json:"type"`
			Text     string `json:"text"`
			MimeType string `json:"mimeType"`
			Resource *struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			} `json:"resource"`
		} `json:"content"`
		Structured json.RawMessage `json:"structuredContent"`
	}
	if err := json.Unmarshal([]byte(trimmed), &r); err != nil || r.Content == nil {
		return s
	}
	hasText := false
	for _, c := range r.Content {
		hasText = hasText || (c.Type == "text" && c.Text != "")
	}
	if !hasText && len(r.Structured) > 0 && string(r.Structured) != "null" {
		return string(r.Structured)
	}
	parts := make([]string, 0, len(r.Content))
	for _, c := range r.Content {
		switch c.Type {
		case "text":
			parts = append(parts, c.Text)
		case "resource":
			if c.Resource != nil && c.Resource.Text != "" {
				parts = append(parts, c.Resource.Text)
			} else if c.Resource != nil {
				parts = append(parts, "[resource "+c.Resource.URI+"]")
			}
		default:
			parts = append(parts, fmt.Sprintf("[%s content (%s) omitted]", c.Type, c.MimeType))
		}
	}
	return strings.Join(parts, "\n")
}

func firstLineOf(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}
	return s
}

func oneLineArgs(args string, maxLen int) string {
	if args == "{}" || args == "" {
		return ""
	}
	s := strings.Join(strings.Fields(args), " ")
	if r := []rune(s); len(r) > maxLen {
		s = string(r[:maxLen-1]) + "…"
	}
	return s
}

func roundDuration(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(10 * time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(time.Millisecond)
	}
	return d.Round(time.Microsecond)
}
