//go:build ignore

// file-explorer.go — a toggleable file explorer panel for Kit.
//
// Press Ctrl+Alt+E (or run /explorer) to open a file tree of the working
// directory. The tree is drawn as a left-aligned panel above the input, and
// while it has focus an editor interceptor takes the navigation keys.
//
// Opening a file uses the default application configured on the machine:
//
//   - Linux/BSD: the xdg-mime default for the file's MIME type. When that
//     desktop entry has Terminal=true (vim, helix, less, ...) Kit releases the
//     terminal and runs it in place, the same way Ctrl+X e runs $EDITOR. GUI
//     apps are started with xdg-open and Kit keeps running. With no default,
//     or no display (e.g. over SSH), the file opens in $VISUAL/$EDITOR.
//   - macOS: `open`.
//
// Keys while the explorer has focus:
//
//	up/k, down/j    move            home/g, end/G   first / last entry
//	right/l         expand dir      left/h          collapse / go to parent
//	enter           open file or toggle dir
//	e               open in $VISUAL/$EDITOR (terminal, like Ctrl+X e)
//	a               add @path to the prompt
//	backspace       move the tree root up one directory
//	.               show/hide dotfiles      r   reload from disk
//	esc             close the explorer (Ctrl+Alt+E also toggles it)
//
// Usage: kit -e examples/extensions/file-explorer.go
//
// NOTE ON YAEGI: every helper is declared ABOVE the code that references it,
// and tagless switches never use comma case lists. See AGENTS.md.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"kit/ext"
)

const widgetID = "file-explorer:tree"

// ---------------------------------------------------------------------------
// State (package-level vars survive across callbacks)
// ---------------------------------------------------------------------------

type node struct {
	path     string
	name     string
	isDir    bool
	depth    int
	expanded bool
	loaded   bool
	children []*node
}

var (
	mu         sync.Mutex
	root       *node
	visible    []*node
	cursor     int
	offset     int
	open       bool
	showHidden bool
	lastErr    string

	// Captured from the last shortcut/command so the editor interceptor (which
	// receives no Context) can reach the Kit API.
	kctx    ext.Context
	haveCtx bool

	borderOn = "\033[38;5;75m"
	dirColor = "\033[38;5;75m"
)

// ---------------------------------------------------------------------------
// ANSI helpers
// ---------------------------------------------------------------------------

const ansiReset = "\033[0m"

func fgHex(hex string) string {
	var r, g, b int
	if len(hex) != 7 || hex[0] != '#' {
		return ""
	}
	if _, err := fmt.Sscanf(hex[1:], "%02x%02x%02x", &r, &g, &b); err != nil {
		return ""
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", r, g, b)
}

func applyTheme(t ext.ThemeColors) {
	if c := fgHex(t.Primary); c != "" {
		borderOn = c
		dirColor = c
	}
}

// fit truncates or pads s to exactly n columns. Rune-based, which is close
// enough for file names in an example.
func fit(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) > n {
		if n == 1 {
			return "…"
		}
		return string(r[:n-1]) + "…"
	}
	return s + strings.Repeat(" ", n-len(r))
}

// ---------------------------------------------------------------------------
// Tree model
// ---------------------------------------------------------------------------

func loadChildren(n *node) {
	n.loaded = true
	n.children = nil
	entries, err := os.ReadDir(n.path)
	if err != nil {
		lastErr = err.Error()
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}
		p := filepath.Join(n.path, name)
		isDir := e.IsDir()
		// Follow symlinks so a linked directory can be expanded.
		if e.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(p); err == nil {
				isDir = info.IsDir()
			}
		}
		n.children = append(n.children, &node{path: p, name: name, isDir: isDir, depth: n.depth + 1})
	}
	sort.Slice(n.children, func(i, j int) bool {
		a, b := n.children[i], n.children[j]
		if a.isDir != b.isDir {
			return a.isDir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
}

func appendVisible(n *node) {
	for _, c := range n.children {
		visible = append(visible, c)
		if c.isDir && c.expanded {
			if !c.loaded {
				loadChildren(c)
			}
			appendVisible(c)
		}
	}
}

func rebuild() {
	visible = nil
	if root == nil {
		return
	}
	if !root.loaded {
		loadChildren(root)
	}
	appendVisible(root)
	if cursor >= len(visible) {
		cursor = len(visible) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
}

// collectExpanded records expanded dirs so a reload keeps the tree shape.
func collectExpanded(n *node, out map[string]bool) {
	for _, c := range n.children {
		if c.isDir && c.expanded {
			out[c.path] = true
			collectExpanded(c, out)
		}
	}
}

func restoreExpanded(n *node, want map[string]bool) {
	for _, c := range n.children {
		if c.isDir && want[c.path] {
			c.expanded = true
			loadChildren(c)
			restoreExpanded(c, want)
		}
	}
}

func setRoot(dir string) {
	root = &node{path: dir, name: filepath.Base(dir), isDir: true, depth: -1, expanded: true}
	cursor, offset = 0, 0
	rebuild()
}

func reload() {
	if root == nil {
		return
	}
	var sel string
	if cursor < len(visible) {
		sel = visible[cursor].path
	}
	want := map[string]bool{}
	collectExpanded(root, want)
	loadChildren(root)
	restoreExpanded(root, want)
	rebuild()
	for i, n := range visible {
		if n.path == sel {
			cursor = i
			break
		}
	}
}

func parentIndex(i int) int {
	d := visible[i].depth
	for j := i - 1; j >= 0; j-- {
		if visible[j].depth < d {
			return j
		}
	}
	return -1
}

func selected() *node {
	if cursor < 0 || cursor >= len(visible) {
		return nil
	}
	return visible[cursor]
}

// ---------------------------------------------------------------------------
// Opening files
// ---------------------------------------------------------------------------

// editorArgv returns $VISUAL or $EDITOR split into argv, like Ctrl+X e.
func editorArgv(path string) []string {
	app := os.Getenv("VISUAL")
	if app == "" {
		app = os.Getenv("EDITOR")
	}
	f := strings.Fields(app)
	if len(f) == 0 {
		return nil
	}
	return append(f, path)
}

// runInTerminal hands the terminal to argv and restores the TUI afterwards.
// Must run off the TUI event loop (we always call it from a goroutine).
func runInTerminal(ctx ext.Context, argv []string) {
	var runErr error
	err := ctx.SuspendTUI(func() {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr = cmd.Run()
	})
	if err == nil {
		err = runErr
	}
	if err != nil {
		ctx.PrintError(fmt.Sprintf("explorer: %s: %v", argv[0], err))
	}
}

// startDetached launches a GUI program and returns at once.
func startDetached(ctx ext.Context, argv []string) {
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		ctx.PrintError(fmt.Sprintf("explorer: %s: %v", argv[0], err))
		return
	}
	go func() { _ = cmd.Wait() }()
}

func openInEditor(ctx ext.Context, path string) {
	argv := editorArgv(path)
	if argv == nil {
		ctx.PrintError("explorer: set $VISUAL or $EDITOR to open files in a terminal editor")
		return
	}
	runInTerminal(ctx, argv)
}

// splitExec tokenizes a desktop-entry Exec value (double quotes, backslash
// escapes) and substitutes the field codes with path.
func splitExec(execLine, path string) []string {
	var args []string
	var cur strings.Builder
	inQuote, have := false, false
	r := []rune(execLine)
	for i := 0; i < len(r); i++ {
		c := r[i]
		if c == '\\' && i+1 < len(r) {
			i++
			cur.WriteRune(r[i])
			have = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			have = true
			continue
		}
		if (c == ' ' || c == '\t') && !inQuote {
			if have {
				args = append(args, cur.String())
				cur.Reset()
				have = false
			}
			continue
		}
		cur.WriteRune(c)
		have = true
	}
	if have {
		args = append(args, cur.String())
	}

	var out []string
	gotPath := false
	for _, a := range args {
		switch a {
		case "%f", "%F", "%u", "%U":
			out = append(out, path)
			gotPath = true
			continue
		case "%i", "%c", "%k":
			continue
		}
		a = strings.ReplaceAll(a, "%%", "\x00")
		for _, code := range []string{"%f", "%F", "%u", "%U"} {
			if strings.Contains(a, code) {
				a = strings.ReplaceAll(a, code, path)
				gotPath = true
			}
		}
		for _, code := range []string{"%i", "%c", "%k", "%d", "%D", "%n", "%N", "%v", "%m"} {
			a = strings.ReplaceAll(a, code, "")
		}
		out = append(out, strings.ReplaceAll(a, "\x00", "%"))
	}
	if !gotPath {
		out = append(out, path)
	}
	return out
}

// desktopEntry finds and parses an XDG .desktop file by id. ok is false when
// the file cannot be found.
func desktopEntry(id string) (execLine string, terminal bool, ok bool) {
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	dirs := append([]string{dataHome}, filepath.SplitList(dataDirs)...)
	for _, d := range dirs {
		f, err := os.Open(filepath.Join(d, "applications", id))
		if err != nil {
			continue
		}
		inMain := false
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if strings.HasPrefix(line, "[") {
				inMain = line == "[Desktop Entry]"
				continue
			}
			if !inMain {
				continue
			}
			if strings.HasPrefix(line, "Exec=") {
				execLine = strings.TrimPrefix(line, "Exec=")
			} else if strings.HasPrefix(line, "Terminal=") {
				terminal = strings.TrimPrefix(line, "Terminal=") == "true"
			}
		}
		_ = f.Close()
		return execLine, terminal, execLine != ""
	}
	return "", false, false
}

func xdgQuery(args ...string) string {
	out, err := exec.Command("xdg-mime", append([]string{"query"}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// openDefault opens path with the machine's default application. Terminal
// apps take over the terminal like Ctrl+X e; GUI apps start detached.
func openDefault(ctx ext.Context, path string) {
	if runtime.GOOS == "darwin" {
		startDetached(ctx, []string{"open", path})
		return
	}
	if runtime.GOOS == "windows" {
		startDetached(ctx, []string{"cmd", "/c", "start", "", path})
		return
	}

	hasDisplay := os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
	if _, err := exec.LookPath("xdg-mime"); err == nil {
		mime := xdgQuery("filetype", path)
		if id := xdgQuery("default", mime); mime != "" && id != "" {
			if execLine, term, ok := desktopEntry(id); ok {
				argv := splitExec(execLine, path)
				if term {
					runInTerminal(ctx, argv)
					return
				}
				if hasDisplay {
					startDetached(ctx, argv)
					return
				}
			}
		}
	}

	// No usable default (or no display for a GUI app): use the terminal
	// editor when one is set, otherwise let xdg-open try its best.
	if editorArgv(path) != nil {
		openInEditor(ctx, path)
		return
	}
	if _, err := exec.LookPath("xdg-open"); err == nil && hasDisplay {
		startDetached(ctx, []string{"xdg-open", path})
		return
	}
	ctx.PrintError("explorer: no default application found; set $VISUAL or $EDITOR")
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

// renderPanel draws the file tree within the allocated content dimensions.
func renderPanel(width, height int) string {
	mu.Lock()
	defer mu.Unlock()
	if !open || root == nil {
		return ""
	}

	w := width * 35 / 100
	if w < 30 {
		w = 30
	}
	if w > 56 {
		w = 56
	}
	if w > width {
		w = width
	}
	inner := w - 2

	bc := borderOn

	rows := height - 2 // top and bottom borders
	if rows < 0 {
		rows = 0
	}
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+rows {
		offset = cursor - rows + 1
	}
	if offset < 0 {
		offset = 0
	}

	var b strings.Builder
	// Show the tail of a long root path: the deepest dirs matter most.
	rp := []rune(root.path)
	if maxLen := inner - 3; len(rp) > maxLen && maxLen > 1 {
		rp = append([]rune("…"), rp[len(rp)-maxLen+1:]...)
	}
	title := " " + string(rp) + " "
	pad := inner - 1 - len([]rune(title))
	if pad < 0 {
		pad = 0
	}
	b.WriteString(bc + "╭─" + ansiReset + "\033[1m" + title + ansiReset + bc + strings.Repeat("─", pad) + "╮" + ansiReset + "\n")

	for i := 0; i < rows; i++ {
		idx := offset + i
		line := ""
		var n *node
		if idx < len(visible) {
			n = visible[idx]
			icon := "  "
			if n.isDir {
				icon = "▸ "
				if n.expanded {
					icon = "▾ "
				}
			}
			name := n.name
			if n.isDir {
				name += "/"
			}
			line = strings.Repeat("  ", n.depth) + icon + name
		} else if idx == 0 {
			line = "(empty)"
		}
		cell := fit(" "+line, inner)
		if n != nil && idx == cursor {
			cell = "\033[7m" + cell + ansiReset
		} else if n != nil && n.isDir {
			cell = dirColor + cell + ansiReset
		}
		b.WriteString(bc + "│" + ansiReset + cell + bc + "│" + ansiReset + "\n")
	}

	hint := " ⏎ open · e edit · a @ · esc "
	if lastErr != "" {
		hint = " " + lastErr + " "
	}
	hint = fit(hint, inner-1)
	b.WriteString(bc + "╰─" + "\033[2m" + hint + ansiReset + bc + "╯" + ansiReset)
	return b.String()
}

// ---------------------------------------------------------------------------
// Focus / visibility
// ---------------------------------------------------------------------------

// widgetConfig reserves the transcript rows for the visible file tree.
func widgetConfig() ext.WidgetConfig {
	return ext.WidgetConfig{
		ID:        widgetID,
		Placement: ext.WidgetAbove,
		Priority:  -100,
		Style:     ext.WidgetStyle{NoBorder: true},
		Content: ext.WidgetContent{
			FillHeight:  true,
			RenderSized: func(width, height int) string { return renderPanel(width, height) },
		},
	}
}

func closePanel(ctx ext.Context) {
	ctx.ResetEditor()
	ctx.RemoveWidget(widgetID)
}

// handleKey runs synchronously on the TUI event loop: it must not block.
// Anything that suspends the TUI runs in a goroutine.
func handleKey(key, text string) ext.EditorKeyAction {
	consumed := ext.EditorKeyAction{Type: ext.EditorKeyConsumed}

	mu.Lock()
	if !open {
		mu.Unlock()
		return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
	}
	lastErr = ""
	ctx, ok := kctx, haveCtx
	n := selected()
	var openPath, editPath, attachPath string
	closing := false

	switch key {
	case "up", "k":
		if cursor > 0 {
			cursor--
		}
	case "down", "j":
		if cursor < len(visible)-1 {
			cursor++
		}
	case "home", "g":
		cursor = 0
	case "end", "G", "shift+g":
		if len(visible) > 0 {
			cursor = len(visible) - 1
		}
	case "right", "l":
		if n != nil && n.isDir && !n.expanded {
			n.expanded = true
			rebuild()
		}
	case "left", "h":
		if n != nil && n.isDir && n.expanded {
			n.expanded = false
			rebuild()
		} else if n != nil {
			if p := parentIndex(cursor); p >= 0 {
				cursor = p
			}
		}
	case "enter":
		if n != nil && n.isDir {
			n.expanded = !n.expanded
			rebuild()
		} else if n != nil {
			openPath = n.path
		}
	case "e":
		if n != nil && !n.isDir {
			editPath = n.path
		}
	case "a":
		if n != nil {
			attachPath = n.path
		}
	case "backspace":
		if root != nil {
			if parent := filepath.Dir(root.path); parent != root.path {
				prev := root.path
				setRoot(parent)
				for i, v := range visible {
					if v.path == prev {
						cursor = i
						break
					}
				}
			}
		}
	case ".":
		showHidden = !showHidden
		reload()
	case "r":
		reload()
	case "esc":
		open = false
		closing = true
	default:
		mu.Unlock()
		// Let modifier combos (ctrl+..., alt+...) and other special keys
		// reach Kit; swallow plain typing so it does not leak into the prompt.
		if len([]rune(key)) > 1 && key != "space" {
			return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
		}
		return consumed
	}
	mu.Unlock()

	if !ok {
		return consumed
	}
	if closing {
		closePanel(ctx)
		return consumed
	}
	if attachPath != "" {
		rel := attachPath
		if cwd, err := os.Getwd(); err == nil {
			if r, err := filepath.Rel(cwd, attachPath); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		if strings.ContainsAny(rel, " \t") {
			rel = `"` + rel + `"`
		}
		sep := ""
		if text != "" && !strings.HasSuffix(text, " ") {
			sep = " "
		}
		// SetEditorText posts a message to the TUI; doing that synchronously
		// from inside the key handler (which runs in Update) deadlocks.
		newText := text + sep + "@" + rel + " "
		go ctx.SetEditorText(newText)
	}
	if openPath != "" {
		go func() {
			openDefault(ctx, openPath)
			mu.Lock()
			reload()
			mu.Unlock()
			ctx.SetWidget(widgetConfig())
		}()
	}
	if editPath != "" {
		go func() {
			openInEditor(ctx, editPath)
			mu.Lock()
			reload()
			mu.Unlock()
			ctx.SetWidget(widgetConfig())
		}()
	}
	return consumed
}

// focusPanel is below handleKey: the interceptor closure calls handleKey by
// name, so handleKey must be declared first (Yaegi forward-reference bug).
func focusPanel(ctx ext.Context) {
	ctx.SetEditor(ext.EditorConfig{
		HandleKey: func(key string, text string) ext.EditorKeyAction { return handleKey(key, text) },
	})
	ctx.SetWidget(widgetConfig())
}

// toggle opens the explorer when it is closed and closes it when it is open.
// With dir set it always opens, rooted at dir.
func toggle(ctx ext.Context, dir string) {
	mu.Lock()
	kctx, haveCtx = ctx, true
	if ctx.GetTheme != nil {
		applyTheme(ctx.GetTheme())
	}

	if dir != "" {
		abs, err := filepath.Abs(dir)
		if err == nil {
			setRoot(abs)
		}
		open = true
		mu.Unlock()
		focusPanel(ctx)
		return
	}

	if open {
		open = false
		mu.Unlock()
		closePanel(ctx)
		return
	}
	if root == nil {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		setRoot(cwd)
	} else {
		reload()
	}
	open = true
	mu.Unlock()
	focusPanel(ctx)
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	api.RegisterShortcut(ext.ShortcutDef{
		Key:         "ctrl+alt+e",
		Description: "Toggle the file explorer",
	}, func(ctx ext.Context) { toggle(ctx, "") })

	api.RegisterCommand(ext.CommandDef{
		Name:        "explorer",
		Description: "Toggle the file explorer (optional: /explorer <dir>)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			dir := strings.TrimSpace(args)
			if dir != "" {
				if info, err := os.Stat(dir); err != nil || !info.IsDir() {
					return "", fmt.Errorf("not a directory: %s", dir)
				}
			}
			toggle(ctx, dir)
			return "", nil
		},
	})
}
