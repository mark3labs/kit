//go:build ignore

// popup-terminal.go — a persistent, toggleable terminal for Kit.
//
// Runs the user's shell inside Kit by releasing the terminal with
// ctx.SuspendTUI(). When a terminal multiplexer is available (tmux, abduco or
// dtach) the shell is hosted by the multiplexer, so it keeps running in the
// background while you are back in Kit: cwd, environment, shell history and
// any running jobs all survive. Re-running /term reattaches to the same shell.
//
// The multiplexer session is named after the Kit session ID, so each Kit
// session gets its own shell and two Kit windows never collide. On shutdown
// the session is killed (see the terminal-kill-on-exit option).
//
// Commands:
//
//	/term          — attach to the shell, creating it on first use
//	/term status   — report the backend, session name and liveness
//	/term kill     — kill the background shell (tmux only)
//
// Shortcut:
//
//	ctrl+alt+t     — same as /term
//
// Options (env KIT_OPT_<NAME> or config options.<name>):
//
//	terminal-multiplexer   auto | tmux | abduco | dtach | none   (default auto)
//	terminal-shell         override $SHELL
//	terminal-kill-on-exit  true | false                          (default true)
//	terminal-dry-run       true | false                          (default false)
//
// terminal-dry-run prints the command that would be run instead of running
// it. Use it to debug backend detection without losing the TUI.
//
// NOTE ON YAEGI: every helper is declared ABOVE its first use, and every
// struct function field is an anonymous closure. A bare reference to a
// function declared later in the file silently returns zero values.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	ext "kit/ext"
)

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

var (
	mu sync.Mutex

	// attached records that we handed the terminal over at least once this
	// Kit session. It gates the shutdown cleanup so we never kill a session
	// we did not open.
	attached bool

	// lastBackend and lastName remember what we attached to, so shutdown
	// cleanup does not have to re-resolve options against a Context that
	// may no longer be fully wired.
	lastBackend string
	lastName    string
)

const statusKey = "popup-terminal"

// ---------------------------------------------------------------------------
// Helpers (declared before use — see the Yaegi note above)
// ---------------------------------------------------------------------------

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// optOr reads an option and falls back to def when it is unset. Kit resolves
// registered defaults itself, but test harnesses and headless contexts return
// an empty string, so never trust a bare GetOption.
func optOr(ctx ext.Context, name string, def string) string {
	if ctx.GetOption == nil {
		return def
	}
	v := strings.TrimSpace(ctx.GetOption(name))
	if v == "" {
		return def
	}
	return v
}

func isTrue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// sanitizeName makes an arbitrary string safe as a tmux session name and as a
// filename. tmux treats "." and ":" as window and pane separators, so they
// must not survive.
// YAEGI: the conditions below use "||" inside one case expression rather
// than a comma-separated case list. In a tagless switch Yaegi evaluates only
// the FIRST expression of "case a, b, c:" and silently ignores the rest.
func sanitizeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// sessionNameFor derives the multiplexer session name from the Kit session ID.
// Without a session ID (kit --no-session) it falls back to the process ID so
// concurrent Kit instances still get separate shells.
func sessionNameFor(sessionID string) string {
	clean := sanitizeName(sessionID)
	if clean == "" {
		return fmt.Sprintf("kit-p%d", os.Getpid())
	}
	return "kit-" + clean
}

func shellFor(ctx ext.Context) string {
	return firstNonEmpty(
		optOr(ctx, "terminal-shell", ""),
		os.Getenv("SHELL"),
		"/bin/sh",
	)
}

func havePath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// detectBackend resolves the multiplexer preference into a concrete backend.
// "auto" probes for each supported multiplexer in order of preference and
// degrades to "none" (a plain, non-persistent shell) when none is installed.
func detectBackend(pref string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(pref)) {
	case "", "auto":
		for _, b := range []string{"tmux", "abduco", "dtach"} {
			if havePath(b) {
				return b, nil
			}
		}
		return "none", nil
	case "tmux", "abduco", "dtach":
		b := strings.ToLower(strings.TrimSpace(pref))
		if !havePath(b) {
			return "", fmt.Errorf("terminal-multiplexer is %q but %s is not on PATH", b, b)
		}
		return b, nil
	case "none":
		return "none", nil
	default:
		return "", fmt.Errorf("unknown terminal-multiplexer %q (want auto, tmux, abduco, dtach or none)", pref)
	}
}

func sockPath(name string) string {
	return filepath.Join(os.TempDir(), name+".dtach")
}

// persistent reports whether a backend keeps the shell alive after detaching.
func persistent(backend string) bool {
	return backend == "tmux" || backend == "abduco" || backend == "dtach"
}

// attachArgv builds the command that attaches to the session, creating it if
// it does not exist. Every backend here has an "attach or create" flag, which
// is what makes the toggle idempotent.
func attachArgv(backend string, name string, shell string, cwd string) []string {
	switch backend {
	case "tmux":
		argv := []string{"tmux", "new-session", "-A", "-s", name}
		if cwd != "" {
			argv = append(argv, "-c", cwd)
		}
		return append(argv, shell)
	case "abduco":
		return []string{"abduco", "-A", name, shell}
	case "dtach":
		// -z disables dtach's own suspend key so ctrl+z reaches the shell.
		return []string{"dtach", "-A", sockPath(name), "-z", shell}
	default:
		return []string{shell}
	}
}

// sessionAlive reports whether a detached background session exists.
func sessionAlive(backend string, name string) bool {
	switch backend {
	case "tmux":
		// "=" forces an exact match; without it tmux does prefix matching
		// and "kit-abc" would match "kit-abcdef".
		return exec.Command("tmux", "has-session", "-t", "="+name).Run() == nil
	case "abduco":
		out, err := exec.Command("abduco").CombinedOutput()
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasSuffix(strings.TrimSpace(line), name) {
				return true
			}
		}
		return false
	case "dtach":
		_, err := os.Stat(sockPath(name))
		return err == nil
	default:
		return false
	}
}

// killSession tears down the background shell. Only tmux exposes a way to do
// this without attaching first.
func killSession(backend string, name string) error {
	switch backend {
	case "tmux":
		if !sessionAlive("tmux", name) {
			return fmt.Errorf("no tmux session named %s", name)
		}
		out, err := exec.Command("tmux", "kill-session", "-t", "="+name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tmux kill-session: %v: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	case "abduco", "dtach":
		return fmt.Errorf("%s cannot kill a detached session from outside; attach with /term and exit the shell", backend)
	default:
		return fmt.Errorf("no background session to kill (multiplexer is disabled)")
	}
}

func detachHint(backend string) string {
	switch backend {
	case "tmux":
		return "detach with ctrl+b d (or type exit to end the shell)"
	case "abduco", "dtach":
		return "detach with ctrl+\\ (or type exit to end the shell)"
	default:
		return "type exit to return to Kit"
	}
}

// childEnv strips $TMUX when launching tmux so an outer tmux does not refuse
// to nest. Kit itself may be running inside tmux; the popup then becomes a
// nested session, which works.
func childEnv(backend string) []string {
	env := os.Environ()
	if backend != "tmux" {
		return env
	}
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, "TMUX=") {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func quoteArgv(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		if a == "" || strings.ContainsAny(a, " \t\"'\\") {
			parts = append(parts, fmt.Sprintf("%q", a))
			continue
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// refreshStatus shows a status bar marker while a background shell is alive,
// so the user can tell the terminal is still there after detaching.
func refreshStatus(ctx ext.Context, backend string, name string) {
	if ctx.SetStatus == nil || ctx.RemoveStatus == nil {
		return
	}
	if persistent(backend) && sessionAlive(backend, name) {
		ctx.SetStatus(statusKey, "term", 20)
		return
	}
	ctx.RemoveStatus(statusKey)
}

// resolve gathers everything a command needs, so each command body stays
// short and the failure modes are reported in one place.
func resolve(ctx ext.Context) (string, string, string, error) {
	backend, err := detectBackend(optOr(ctx, "terminal-multiplexer", "auto"))
	if err != nil {
		return "", "", "", err
	}
	return backend, sessionNameFor(ctx.SessionID), shellFor(ctx), nil
}

// runStatus implements /term status.
func runStatus(ctx ext.Context) (string, error) {
	backend, name, shell, err := resolve(ctx)
	if err != nil {
		return "", err
	}

	state := "not started"
	if persistent(backend) {
		if sessionAlive(backend, name) {
			state = "running (detached)"
		} else {
			state = "not started"
		}
	} else {
		state = "no multiplexer — the shell does not persist between attaches"
	}

	lines := []string{
		"backend:  " + backend,
		"session:  " + name,
		"shell:    " + shell,
		"state:    " + state,
	}
	if backend == "none" {
		lines = append(lines, "", "Install tmux, abduco or dtach for a shell that survives detaching.")
	}
	ctx.PrintInfo(strings.Join(lines, "\n"))
	refreshStatus(ctx, backend, name)
	return "", nil
}

// runKill implements /term kill.
func runKill(ctx ext.Context) (string, error) {
	backend, name, _, err := resolve(ctx)
	if err != nil {
		return "", err
	}
	if err := killSession(backend, name); err != nil {
		return "", err
	}
	mu.Lock()
	attached = false
	mu.Unlock()
	refreshStatus(ctx, backend, name)
	return "Killed " + name + ".", nil
}

// runAttach implements /term. It is the whole feature: release the TUI, hand
// the real terminal to the shell, and take it back when the user detaches.
func runAttach(ctx ext.Context) (string, error) {
	backend, name, shell, err := resolve(ctx)
	if err != nil {
		return "", err
	}

	cwd := ctx.CWD
	argv := attachArgv(backend, name, shell, cwd)
	hint := detachHint(backend)

	if isTrue(optOr(ctx, "terminal-dry-run", "false")) {
		ctx.PrintInfo(strings.Join([]string{
			"dry run — nothing was executed",
			"",
			"backend:  " + backend,
			"session:  " + name,
			"cwd:      " + cwd,
			"command:  " + quoteArgv(argv),
		}, "\n"))
		return "", nil
	}

	if !ctx.Interactive {
		return "", fmt.Errorf("/term needs the interactive TUI")
	}
	if ctx.SuspendTUI == nil {
		return "", fmt.Errorf("terminal suspension is unavailable in this mode")
	}

	resuming := persistent(backend) && sessionAlive(backend, name)

	var runErr error
	err = ctx.SuspendTUI(func() {
		// Printed into the handed-over terminal, not into Kit's scrollback,
		// so the user sees it exactly when it is useful.
		if resuming {
			fmt.Fprintf(os.Stdout, "\r\n[kit] reattaching to %s — %s\r\n", name, hint)
		} else {
			fmt.Fprintf(os.Stdout, "\r\n[kit] starting %s — %s\r\n", name, hint)
		}

		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = cwd
		cmd.Env = childEnv(backend)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		runErr = cmd.Run()
	})
	if err != nil {
		return "", fmt.Errorf("could not suspend the TUI: %w", err)
	}

	mu.Lock()
	attached = true
	lastBackend = backend
	lastName = name
	mu.Unlock()

	refreshStatus(ctx, backend, name)

	if runErr != nil {
		// A shell that exits non-zero is normal, so this is informational.
		ctx.Print(fmt.Sprintf("Terminal exited: %v", runErr))
	}

	if persistent(backend) && sessionAlive(backend, name) {
		return "Detached from " + name + " — it keeps running. /term to reattach.", nil
	}
	return "Terminal session ended.", nil
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "terminal-multiplexer",
		Description: "Multiplexer hosting the popup shell: auto, tmux, abduco, dtach or none",
		Default:     "auto",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "terminal-shell",
		Description: "Shell to run in the popup terminal (defaults to $SHELL)",
		Default:     "",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "terminal-kill-on-exit",
		Description: "Kill the background shell when the Kit session ends",
		Default:     "true",
	})
	api.RegisterOption(ext.OptionDef{
		Name:        "terminal-dry-run",
		Description: "Print the terminal command instead of running it",
		Default:     "false",
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "term",
		Description: "Open a persistent terminal (subcommands: status, kill)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			switch strings.ToLower(strings.TrimSpace(args)) {
			case "":
				return runAttach(ctx)
			case "status", "info":
				return runStatus(ctx)
			case "kill", "stop":
				return runKill(ctx)
			default:
				return "", fmt.Errorf("usage: /term [status|kill]")
			}
		},
		Complete: func(prefix string, ctx ext.Context) []string {
			var out []string
			for _, s := range []string{"status", "kill"} {
				if strings.HasPrefix(s, prefix) {
					out = append(out, s)
				}
			}
			return out
		},
	})

	api.RegisterShortcut(ext.ShortcutDef{
		Key:         "ctrl+alt+t",
		Description: "Open the popup terminal",
	}, func(ctx ext.Context) {
		if _, err := runAttach(ctx); err != nil {
			ctx.PrintError(err.Error())
		}
	})

	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		backend, name, _, err := resolve(ctx)
		if err != nil {
			return
		}
		// A session left over from a previous Kit run with the same session
		// ID (a resumed session) is reattachable, so surface it.
		refreshStatus(ctx, backend, name)
	})

	api.OnSessionShutdown(func(e ext.SessionShutdownEvent, ctx ext.Context) {
		mu.Lock()
		didAttach := attached
		backend := lastBackend
		name := lastName
		mu.Unlock()

		if !didAttach || !persistent(backend) {
			return
		}
		if !isTrue(optOr(ctx, "terminal-kill-on-exit", "true")) {
			return
		}
		// Best effort: the shell is scoped to this Kit session, so it should
		// not outlive it. Errors here are not worth reporting during exit.
		_ = killSession(backend, name)
	})
}
