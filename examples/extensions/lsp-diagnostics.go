//go:build ignore

// lsp-diagnostics.go — LSP-powered diagnostics for Kit's edit tool.
//
// Starts language servers on demand and surfaces diagnostics after file edits:
//  1. After an edit, notify the LSP server of the file change
//  2. Wait for the server to publish fresh diagnostics
//  3. Append diagnostic output to the edit tool's result
//
// This gives the LLM immediate feedback when an edit introduces errors,
// enabling it to self-correct without a separate build/lint step.
//
// Features:
//   - Auto-starts LSP servers per language on first file edit
//   - Post-edit diagnostics appended to edit tool results
//   - Pre/post diff highlights newly introduced errors
//   - Persistent TUI widget showing diagnostic counts
//   - lsp_diagnostics tool callable by the LLM
//   - lsp_hover tool for type/documentation lookups
//   - /lsp command to view active server status
//   - /lsp-check <file> command for manual diagnostics
//
// Configuration (via options, KIT_OPT_* env vars, or .kit.yml):
//
//	lsp-go       Go server command       (default: gopls)
//	lsp-ts       TypeScript server cmd    (default: typescript-language-server --stdio)
//	lsp-python   Python server command    (default: pylsp)
//	lsp-rust     Rust server command      (default: rust-analyzer)
//
// Usage:
//
//	kit -e examples/extensions/lsp-diagnostics.go
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"kit/ext"
)

// ── Package-level state ─────────────────────────────────────────

var (
	manager      *lspManager
	preEditCache map[string][]diagEntry // abs path → diagnostics before edit
	cacheMu      sync.Mutex
)

// ── Diagnostic entry ────────────────────────────────────────────

// diagEntry is a single diagnostic message from an LSP server.
type diagEntry struct {
	File     string
	Line     int // 0-indexed (LSP convention)
	Char     int
	EndLine  int
	EndChar  int
	Severity int // 1=Error 2=Warning 3=Info 4=Hint
	Source   string
	Message  string
}

func (d diagEntry) icon() string {
	switch d.Severity {
	case 1:
		return "E"
	case 2:
		return "W"
	case 3:
		return "I"
	case 4:
		return "H"
	default:
		return "?"
	}
}

// key returns a comparable identity string for diffing diagnostics.
// Note: line shifts from edits above a diagnostic cause false "new" entries;
// this is acceptable — the diff is a hint, not a guarantee.
func (d diagEntry) key() string {
	return fmt.Sprintf("%d:%d:%d:%s", d.Line, d.Char, d.Severity, d.Message)
}

// ── LSP Client ──────────────────────────────────────────────────

// lspClient manages a single LSP server process and its JSON-RPC transport.
type lspClient struct {
	lang    string
	workDir string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	writeMu sync.Mutex

	nextID    int
	idMu      sync.Mutex
	pending   map[int]chan json.RawMessage
	pendingMu sync.Mutex

	diagnostics map[string][]diagEntry // uri → diagnostics
	diagVersion int
	diagMu      sync.Mutex

	openFiles   map[string]int // uri → version
	openFilesMu sync.Mutex

	ready bool
	done  chan struct{}
}

func newLSPClient(lang, command, workDir string) (*lspClient, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty LSP command for %s", lang)
	}

	if _, err := exec.LookPath(parts[0]); err != nil {
		return nil, fmt.Errorf("%s: %s not found on PATH", lang, parts[0])
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workDir
	cmd.Stderr = nil

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", parts[0], err)
	}

	c := &lspClient{
		lang:        lang,
		workDir:     workDir,
		cmd:         cmd,
		stdin:       stdin,
		stdout:      stdout,
		pending:     make(map[int]chan json.RawMessage),
		diagnostics: make(map[string][]diagEntry),
		openFiles:   make(map[string]int),
		done:        make(chan struct{}),
	}

	go c.readLoop()
	return c, nil
}

// readLoop reads Content-Length framed JSON-RPC messages from stdout.
func (c *lspClient) readLoop() {
	defer close(c.done)
	reader := bufio.NewReader(c.stdout)
	for {
		contentLen := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				n, _ := strconv.Atoi(strings.TrimSpace(line[len("Content-Length:"):]))
				contentLen = n
			}
		}
		if contentLen == 0 {
			continue
		}

		body := make([]byte, contentLen)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}
		c.handleMessage(body)
	}
}

func (c *lspClient) handleMessage(body []byte) {
	var msg map[string]any
	if json.Unmarshal(body, &msg) != nil {
		return
	}

	id, hasID := msg["id"]
	method, _ := msg["method"].(string)

	// Server request (has both id and method) → acknowledge with minimal response.
	if hasID && method != "" {
		result := any(nil)
		if method == "workspace/configuration" {
			result = []map[string]any{{}}
		}
		c.sendJSON(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
		return
	}

	// Response to one of our requests.
	if hasID && method == "" {
		idFloat, _ := id.(float64)
		idInt := int(idFloat)
		c.pendingMu.Lock()
		ch, ok := c.pending[idInt]
		if ok {
			delete(c.pending, idInt)
		}
		c.pendingMu.Unlock()
		if ok {
			raw, _ := json.Marshal(msg["result"])
			ch <- raw
		}
		return
	}

	// Notification from server.
	if method == "textDocument/publishDiagnostics" {
		raw, _ := json.Marshal(msg["params"])
		c.handlePublishDiagnostics(raw)
	}
}

func (c *lspClient) handlePublishDiagnostics(raw []byte) {
	var params struct {
		URI         string `json:"uri"`
		Diagnostics []struct {
			Range struct {
				Start struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"start"`
				End struct {
					Line      int `json:"line"`
					Character int `json:"character"`
				} `json:"end"`
			} `json:"range"`
			Severity int    `json:"severity"`
			Source   string `json:"source"`
			Message  string `json:"message"`
		} `json:"diagnostics"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return
	}

	entries := make([]diagEntry, len(params.Diagnostics))
	for i, d := range params.Diagnostics {
		entries[i] = diagEntry{
			File:     uriToPath(params.URI),
			Line:     d.Range.Start.Line,
			Char:     d.Range.Start.Character,
			EndLine:  d.Range.End.Line,
			EndChar:  d.Range.End.Character,
			Severity: d.Severity,
			Source:   d.Source,
			Message:  d.Message,
		}
	}

	c.diagMu.Lock()
	c.diagnostics[params.URI] = entries
	c.diagVersion++
	c.diagMu.Unlock()
}

// ── Transport helpers ───────────────────────────────────────────

func (c *lspClient) sendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	_, err = c.stdin.Write(data)
	return err
}

func (c *lspClient) request(method string, params any) (json.RawMessage, error) {
	c.idMu.Lock()
	id := c.nextID
	c.nextID++
	c.idMu.Unlock()

	ch := make(chan json.RawMessage, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	err := c.sendJSON(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case result := <-ch:
		return result, nil
	case <-time.After(30 * time.Second):
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("%s: %q timed out", c.lang, method)
	case <-c.done:
		return nil, fmt.Errorf("%s: server exited", c.lang)
	}
}

func (c *lspClient) notify(method string, params any) error {
	return c.sendJSON(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

// ── LSP protocol methods ───────────────────────────────────────

func (c *lspClient) initialize() error {
	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   fileURI(c.workDir),
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"publishDiagnostics": map[string]any{},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"synchronization": map[string]any{
					"didSave": true,
				},
			},
		},
	}
	if _, err := c.request("initialize", params); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	if err := c.notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notify: %w", err)
	}
	c.ready = true
	return nil
}

func (c *lspClient) openFile(absPath, langID, content string) {
	uri := fileURI(absPath)
	c.openFilesMu.Lock()
	if _, ok := c.openFiles[uri]; ok {
		c.openFilesMu.Unlock()
		return
	}
	c.openFiles[uri] = 1
	c.openFilesMu.Unlock()

	c.notify("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":        uri,
			"languageId": langID,
			"version":    1,
			"text":       content,
		},
	})
}

func (c *lspClient) changeFile(absPath, content string) {
	uri := fileURI(absPath)
	c.openFilesMu.Lock()
	ver := c.openFiles[uri] + 1
	c.openFiles[uri] = ver
	c.openFilesMu.Unlock()

	c.notify("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": ver,
		},
		"contentChanges": []map[string]any{
			{"text": content},
		},
	})
}

// waitForDiagnostics polls until the server publishes new diagnostics or
// the timeout elapses.
func (c *lspClient) waitForDiagnostics(timeout time.Duration) {
	c.diagMu.Lock()
	startVersion := c.diagVersion
	c.diagMu.Unlock()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		c.diagMu.Lock()
		changed := c.diagVersion != startVersion
		c.diagMu.Unlock()
		if changed {
			time.Sleep(100 * time.Millisecond) // allow batched notifications
			return
		}
	}
}

func (c *lspClient) getDiagnostics(absPath string) []diagEntry {
	uri := fileURI(absPath)
	c.diagMu.Lock()
	defer c.diagMu.Unlock()
	src := c.diagnostics[uri]
	out := make([]diagEntry, len(src))
	copy(out, src)
	return out
}

func (c *lspClient) hover(absPath string, line, char int) (string, error) {
	if !c.ready {
		return "", fmt.Errorf("server not ready")
	}
	result, err := c.request("textDocument/hover", map[string]any{
		"textDocument": map[string]any{"uri": fileURI(absPath)},
		"position":     map[string]any{"line": line, "character": char},
	})
	if err != nil {
		return "", err
	}
	if string(result) == "null" {
		return "No hover information available.", nil
	}
	var hover map[string]any
	if json.Unmarshal(result, &hover) != nil {
		return "No hover information available.", nil
	}
	return parseHoverContents(hover["contents"]), nil
}

func (c *lspClient) shutdown() {
	if !c.ready {
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
			c.cmd.Wait()
		}
		return
	}

	// Graceful: send shutdown request then exit notification.
	shutdownDone := make(chan struct{})
	go func() {
		c.request("shutdown", nil)
		c.notify("exit", nil)
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
	}

	c.stdin.Close()
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	c.cmd.Wait()
}

// ── Hover content parsing ───────────────────────────────────────

func parseHoverContents(v any) string {
	if v == nil {
		return "No hover information available."
	}
	// MarkupContent: {"kind": "markdown", "value": "..."}
	if m, ok := v.(map[string]any); ok {
		if val, ok := m["value"].(string); ok {
			return val
		}
	}
	// Plain string
	if s, ok := v.(string); ok {
		return s
	}
	// Array of MarkedString
	if arr, ok := v.([]any); ok {
		var parts []string
		for _, item := range arr {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			} else if m, ok := item.(map[string]any); ok {
				if val, ok := m["value"].(string); ok {
					parts = append(parts, val)
				}
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n\n")
		}
	}
	return "No hover information available."
}

// ── LSP Manager ─────────────────────────────────────────────────

// lspManager coordinates per-language LSP clients with lazy startup.
type lspManager struct {
	clients  map[string]*lspClient
	commands map[string]string // language → server command
	workDir  string
	mu       sync.Mutex
}

func newLSPManager(workDir string, ctx ext.Context) *lspManager {
	m := &lspManager{
		clients: make(map[string]*lspClient),
		commands: map[string]string{
			"go":         "gopls",
			"typescript": "typescript-language-server --stdio",
			"python":     "pylsp",
			"rust":       "rust-analyzer",
		},
		workDir: workDir,
	}

	// Override defaults from extension options.
	if v := ctx.GetOption("lsp-go"); v != "" {
		m.commands["go"] = v
	}
	if v := ctx.GetOption("lsp-ts"); v != "" {
		m.commands["typescript"] = v
		m.commands["typescriptreact"] = v
		m.commands["javascript"] = v
		m.commands["javascriptreact"] = v
	}
	if v := ctx.GetOption("lsp-python"); v != "" {
		m.commands["python"] = v
	}
	if v := ctx.GetOption("lsp-rust"); v != "" {
		m.commands["rust"] = v
	}

	// Share the TS server command for JS variants if not explicitly set.
	if ts, ok := m.commands["typescript"]; ok {
		if _, ok := m.commands["javascript"]; !ok {
			m.commands["javascript"] = ts
		}
		if _, ok := m.commands["typescriptreact"]; !ok {
			m.commands["typescriptreact"] = ts
		}
		if _, ok := m.commands["javascriptreact"]; !ok {
			m.commands["javascriptreact"] = ts
		}
	}

	return m
}

// clientFor returns (or lazily starts) the LSP client for a file's language.
// Returns nil if unsupported or if the server fails to start.
func (m *lspManager) clientFor(absPath string) *lspClient {
	lang := detectLanguage(absPath)
	if lang == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.clients[lang]; ok {
		return c
	}

	cmd, ok := m.commands[lang]
	if !ok || cmd == "" {
		return nil
	}

	c, err := newLSPClient(lang, cmd, m.workDir)
	if err != nil {
		// Server binary not found or failed to start — skip silently.
		return nil
	}
	if err := c.initialize(); err != nil {
		c.shutdown()
		return nil
	}

	m.clients[lang] = c
	return c
}

// notifyChange opens the file (if needed), sends a didChange notification,
// waits for the server to publish diagnostics, and returns them.
func (m *lspManager) notifyChange(absPath string) []diagEntry {
	client := m.clientFor(absPath)
	if client == nil {
		return nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil
	}

	lang := detectLanguage(absPath)
	client.openFile(absPath, lang, string(content))
	client.changeFile(absPath, string(content))
	client.waitForDiagnostics(5 * time.Second)
	return client.getDiagnostics(absPath)
}

// cachedDiagnostics returns whatever diagnostics are currently cached
// without triggering a refresh. Used for pre-edit snapshots.
func (m *lspManager) cachedDiagnostics(absPath string) []diagEntry {
	client := m.clientFor(absPath)
	if client == nil {
		return nil
	}
	return client.getDiagnostics(absPath)
}

func (m *lspManager) hoverAt(absPath string, line, char int) (string, error) {
	client := m.clientFor(absPath)
	if client == nil {
		return "", fmt.Errorf("no LSP server for %s", filepath.Ext(absPath))
	}

	// Ensure the file is open before requesting hover.
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}
	lang := detectLanguage(absPath)
	client.openFile(absPath, lang, string(content))

	return client.hover(absPath, line, char)
}

func (m *lspManager) shutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for lang, c := range m.clients {
		c.shutdown()
		delete(m.clients, lang)
	}
}

func (m *lspManager) status() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.clients) == 0 {
		return "No active LSP servers."
	}
	var lines []string
	for lang, c := range m.clients {
		state := "ready"
		if !c.ready {
			state = "starting"
		}
		c.diagMu.Lock()
		total := 0
		for _, diags := range c.diagnostics {
			total += len(diags)
		}
		c.diagMu.Unlock()
		lines = append(lines, fmt.Sprintf("  %s: %s (%d diagnostics)", lang, state, total))
	}
	return "Active LSP servers:\n" + strings.Join(lines, "\n")
}

// ── Helpers ─────────────────────────────────────────────────────

func detectLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	default:
		return ""
	}
}

func fileURI(absPath string) string {
	p, err := filepath.Abs(absPath)
	if err != nil {
		p = absPath
	}
	return "file://" + p
}

func uriToPath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

func resolveEditPath(input string, cwd string) string {
	var args struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(input), &args)
	if args.Path == "" {
		return ""
	}
	if filepath.IsAbs(args.Path) {
		return args.Path
	}
	return filepath.Join(cwd, args.Path)
}

func isEditTool(name string) bool {
	return strings.EqualFold(name, "edit")
}

func formatDiagnostics(diags []diagEntry, filePath string) string {
	if len(diags) == 0 {
		return ""
	}
	errors, warnings, infos := countSeverities(diags)
	var lines []string
	for _, d := range diags {
		src := ""
		if d.Source != "" {
			src = "[" + d.Source + "] "
		}
		lines = append(lines, fmt.Sprintf("  %s %s:%d:%d %s%s",
			d.icon(), filepath.Base(d.File), d.Line+1, d.Char+1, src, d.Message))
	}
	summary := formatSummary(errors, warnings, infos)
	return fmt.Sprintf("<lsp_diagnostics file=%q>\n%s\n  Summary: %s\n</lsp_diagnostics>",
		filepath.Base(filePath), strings.Join(lines, "\n"), summary)
}

func formatDiagnosticsPlain(diags []diagEntry) string {
	var lines []string
	for _, d := range diags {
		src := ""
		if d.Source != "" {
			src = "[" + d.Source + "] "
		}
		lines = append(lines, fmt.Sprintf("%s %s:%d:%d %s%s",
			d.icon(), filepath.Base(d.File), d.Line+1, d.Char+1, src, d.Message))
	}
	errors, warnings, infos := countSeverities(diags)
	lines = append(lines, "")
	lines = append(lines, formatSummary(errors, warnings, infos))
	return strings.Join(lines, "\n")
}

func countSeverities(diags []diagEntry) (errors, warnings, infos int) {
	for _, d := range diags {
		switch d.Severity {
		case 1:
			errors++
		case 2:
			warnings++
		default:
			infos++
		}
	}
	return
}

func formatSummary(errors, warnings, infos int) string {
	var parts []string
	if errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error(s)", errors))
	}
	if warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning(s)", warnings))
	}
	if infos > 0 {
		parts = append(parts, fmt.Sprintf("%d info/hint(s)", infos))
	}
	if len(parts) == 0 {
		return "no issues"
	}
	return strings.Join(parts, ", ")
}

func diagBorderColor(diags []diagEntry) string {
	for _, d := range diags {
		if d.Severity == 1 {
			return "#f38ba8" // red
		}
	}
	for _, d := range diags {
		if d.Severity == 2 {
			return "#f9e2af" // yellow
		}
	}
	return "#a6e3a1" // green
}

func diffDiagnostics(before, after []diagEntry) []diagEntry {
	existing := make(map[string]bool)
	for _, d := range before {
		existing[d.key()] = true
	}
	var fresh []diagEntry
	for _, d := range after {
		if !existing[d.key()] {
			fresh = append(fresh, d)
		}
	}
	return fresh
}

// ── Widget helpers ──────────────────────────────────────────────

func updateWidget(ctx ext.Context, diags []diagEntry, filePath string) {
	if len(diags) == 0 {
		ctx.RemoveWidget("lsp-diag:status")
		return
	}
	errors, warnings, _ := countSeverities(diags)
	text := fmt.Sprintf("LSP  %s: %d error(s), %d warning(s)",
		filepath.Base(filePath), errors, warnings)
	ctx.SetWidget(ext.WidgetConfig{
		ID:        "lsp-diag:status",
		Placement: ext.WidgetAbove,
		Content:   ext.WidgetContent{Text: text},
		Style:     ext.WidgetStyle{BorderColor: diagBorderColor(diags)},
	})
}

// ── Extension Init ──────────────────────────────────────────────

func Init(api ext.API) {
	preEditCache = make(map[string][]diagEntry)

	// ── Options ─────────────────────────────────────────────

	api.RegisterOption(ext.OptionDef{
		Name:        "lsp-go",
		Description: "Command to start the Go language server",
		Default:     "gopls",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "lsp-ts",
		Description: "Command to start the TypeScript/JavaScript language server",
		Default:     "typescript-language-server --stdio",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "lsp-python",
		Description: "Command to start the Python language server",
		Default:     "pylsp",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "lsp-rust",
		Description: "Command to start the Rust language server",
		Default:     "rust-analyzer",
	})

	// ── Session lifecycle ───────────────────────────────────

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		manager = newLSPManager(ctx.CWD, ctx)
	})

	api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, ctx ext.Context) {
		if manager != nil {
			manager.shutdownAll()
			manager = nil
		}
		ctx.RemoveWidget("lsp-diag:status")
	})

	// ── Edit tool interception ──────────────────────────────

	// Pre-edit: snapshot current diagnostics so we can diff after.
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if !isEditTool(tc.ToolName) || manager == nil {
			return nil
		}
		absPath := resolveEditPath(tc.Input, ctx.CWD)
		if absPath == "" || detectLanguage(absPath) == "" {
			return nil
		}

		// Warm up the LSP server and capture pre-edit diagnostics.
		diags := manager.cachedDiagnostics(absPath)
		if diags == nil {
			diags = manager.notifyChange(absPath)
		}
		cacheMu.Lock()
		preEditCache[absPath] = diags
		cacheMu.Unlock()

		return nil
	})

	// Post-edit: refresh diagnostics, diff, append to result, update widget.
	api.OnToolResult(func(tr ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
		if !isEditTool(tr.ToolName) || tr.IsError || manager == nil {
			return nil
		}
		absPath := resolveEditPath(tr.Input, ctx.CWD)
		if absPath == "" || detectLanguage(absPath) == "" {
			return nil
		}

		// Notify LSP of the change and get fresh diagnostics.
		postDiags := manager.notifyChange(absPath)

		cacheMu.Lock()
		preDiags := preEditCache[absPath]
		delete(preEditCache, absPath)
		cacheMu.Unlock()

		// Build enhanced result.
		enhanced := tr.Content

		if len(postDiags) > 0 {
			enhanced += "\n\n" + formatDiagnostics(postDiags, absPath)

			// Highlight newly introduced errors.
			newDiags := diffDiagnostics(preDiags, postDiags)
			newErrors := 0
			for _, d := range newDiags {
				if d.Severity == 1 {
					newErrors++
				}
			}
			if newErrors > 0 {
				enhanced += fmt.Sprintf(
					"\n\nThis edit introduced %d new error(s). Review the diagnostics above and fix them.",
					newErrors)
			}
		} else if len(preDiags) > 0 {
			// Errors were resolved by this edit.
			enhanced += "\n\nAll previous LSP diagnostics have been resolved."
		}

		updateWidget(ctx, postDiags, absPath)

		if enhanced == tr.Content {
			return nil
		}
		return &ext.ToolResultResult{Content: &enhanced}
	})

	// ── LLM-callable tools ──────────────────────────────────

	api.RegisterTool(ext.ToolDef{
		Name:        "lsp_diagnostics",
		Description: "Get LSP diagnostics (errors, warnings) for a file. Use after editing to verify changes are correct.",
		Parameters:  `{"type":"object","properties":{"file":{"type":"string","description":"File path to check for diagnostics"}},"required":["file"]}`,
		Execute: func(input string) (string, error) {
			if manager == nil {
				return "LSP not initialized.", nil
			}
			var args struct {
				File string `json:"file"`
			}
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			absPath := args.File
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(manager.workDir, absPath)
			}
			diags := manager.notifyChange(absPath)
			if len(diags) == 0 {
				return fmt.Sprintf("No diagnostics for %s.", args.File), nil
			}
			return formatDiagnostics(diags, absPath), nil
		},
	})

	api.RegisterTool(ext.ToolDef{
		Name:        "lsp_hover",
		Description: "Get type information and documentation for a symbol at a specific file position.",
		Parameters: `{"type":"object","properties":{` +
			`"file":{"type":"string","description":"File path"},` +
			`"line":{"type":"integer","description":"Line number (1-indexed)"},` +
			`"character":{"type":"integer","description":"Character offset (1-indexed)"}` +
			`},"required":["file","line","character"]}`,
		Execute: func(input string) (string, error) {
			if manager == nil {
				return "LSP not initialized.", nil
			}
			var args struct {
				File      string `json:"file"`
				Line      int    `json:"line"`
				Character int    `json:"character"`
			}
			if err := json.Unmarshal([]byte(input), &args); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			absPath := args.File
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(manager.workDir, absPath)
			}
			// Convert 1-indexed (user-facing) to 0-indexed (LSP protocol).
			result, err := manager.hoverAt(absPath, args.Line-1, args.Character-1)
			if err != nil {
				return fmt.Sprintf("Hover failed: %s", err), nil
			}
			return result, nil
		},
	})

	// ── Slash commands ──────────────────────────────────────

	api.RegisterCommand(ext.CommandDef{
		Name:        "lsp",
		Description: "Show active LSP server status",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if manager == nil {
				ctx.PrintInfo("LSP manager not initialized.")
				return "", nil
			}
			ctx.PrintInfo(manager.status())
			return "", nil
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "lsp-check",
		Description: "Run LSP diagnostics on a file: /lsp-check <path>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if manager == nil {
				ctx.PrintError("LSP manager not initialized.")
				return "", nil
			}
			path := strings.TrimSpace(args)
			if path == "" {
				ctx.PrintError("Usage: /lsp-check <file-path>")
				return "", nil
			}
			absPath := path
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(ctx.CWD, absPath)
			}
			if _, err := os.Stat(absPath); err != nil {
				ctx.PrintError(fmt.Sprintf("File not found: %s", path))
				return "", nil
			}

			diags := manager.notifyChange(absPath)
			if len(diags) == 0 {
				ctx.PrintInfo(fmt.Sprintf("No diagnostics for %s", path))
			} else {
				ctx.PrintBlock(ext.PrintBlockOpts{
					Text:        formatDiagnosticsPlain(diags),
					BorderColor: diagBorderColor(diags),
					Subtitle:    "lsp-diagnostics",
				})
			}
			updateWidget(ctx, diags, absPath)
			return "", nil
		},
	})
}
