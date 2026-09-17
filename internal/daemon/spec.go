package daemon

import (
	"os"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/clipboard"
	"github.com/mark3labs/kit/internal/ui/termgfx"
)

// Building and sanitising a SessionSpec.
//
// A session child is spawned by the daemon, which shares nothing with the
// terminal the user typed in: not the working directory, not the argument
// list, and not the environment. The spec carries all three across the
// socket, and this file decides what is safe to carry and what the daemon
// keeps for itself.

// SessionFlag is the hidden flag every session child is spawned with. It
// is the marker the crash sweep matches on (see isSessionChild), so it is
// present whether or not a spec supplied any other argument, and it tells
// the child it is being hosted rather than run from a shell.
//
// Exported because the flag has to be registered on the kit command as
// well as passed here, and the two must not be able to drift apart.
const SessionFlag = "--daemon-session"

// SessionFlagName is SessionFlag without the leading dashes, which is how
// a flag is named when it is registered.
const SessionFlagName = "daemon-session"

// SessionSpecForCommand describes the invocation the caller wants a
// daemon-hosted session to reproduce: the directory it was run in, the
// arguments it was given, and the part of its environment a session
// legitimately needs.
//
// args is the argument list WITHOUT the program name, exactly as
// os.Args[1:] reports it.
func SessionSpecForCommand(cwd string, args []string) SessionSpec {
	return SessionSpec{
		Cwd:  cwd,
		Args: slices.Clone(args),
		Env:  forwardableEnv(os.Environ()),
	}
}

// SessionSpecForPicker describes a client that wants to choose a
// directory rather than name one: `kit attach`, which asks for a session
// on this machine without saying where.
//
// The directory still travels, because it decides where the picker opens.
// Rooting it at the client's own directory rather than at the daemon
// user's home is the difference between one keystroke and navigating back
// to the project the user was already in.
func SessionSpecForPicker(cwd string) SessionSpec {
	return SessionSpec{
		Cwd:  cwd,
		Pick: true,
		Env:  forwardableEnv(os.Environ()),
	}
}

// reservedSpecEnv reports whether a variable belongs to the daemon rather
// than to the client.
//
// These name the session's own scratch files, its owner marker, and the
// terminal description the daemon assembles from the TERMINAL frame. A
// client that set them would be describing a session it does not own — at
// worst pointing the crash sweep's ownership proof at another daemon's
// runtime directory — so they are dropped on the way out AND ignored on
// the way in.
func reservedSpecEnv(key string) bool {
	switch key {
	case RemoteSessionEnv,
		RemoteBackgroundEnv,
		sessionOwnerEnv,
		sessionCwdEnv,
		clipboard.RemoteClipboardEnv,
		termgfx.RemoteMultiplexerEnv,
		"TERM",
		"COLORTERM":
		return true
	}
	// The daemon's identity is derived from its own cache and runtime
	// directories. A client that moved them would make its sessions look
	// like another daemon's to the sweep.
	if key == "XDG_CACHE_HOME" || key == "XDG_RUNTIME_DIR" {
		return true
	}
	// Pane variables describe the process that reads them; childEnv
	// already drops the daemon's, and the client's belong to a terminal
	// on the other side of a PTY.
	return slices.Contains(multiplexerEnv, key)
}

// forwardEnvExact are the variables a session needs to behave like a
// shell-launched kit: where to find programs, who the user is, how to
// spell text, and how to reach the network.
var forwardEnvExact = []string{
	"PATH", "HOME", "SHELL", "USER", "LOGNAME",
	"EDITOR", "VISUAL", "PAGER", "LANG", "LC_ALL", "TZ",
	"SSH_AUTH_SOCK",
	"XDG_CONFIG_HOME", "XDG_DATA_HOME",
	"NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
	"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY",
	"http_proxy", "https_proxy", "no_proxy",
	"GOPATH", "GOROOT", "GOFLAGS", "GOPRIVATE",
}

// forwardEnvPrefixes cover the provider credentials and locale settings a
// session needs. They mirror the prefixes the systemd unit snapshots (see
// envFileVars), because the two solve the same problem from opposite
// ends: a daemon that never saw the user's shell cannot authenticate.
var forwardEnvPrefixes = []string{
	"LC_", "KIT_",
	"PROVIDER_", "OPENCODE_", "ANTHROPIC_", "OPENAI_", "GEMINI_",
	"GOOGLE_", "AZURE_", "GROQ_", "MISTRAL_", "DEEPSEEK_",
	"OPENROUTER_", "XAI_", "AWS_", "VERTEX_", "OLLAMA_", "GITHUB_",
}

// forwardableEnv selects the variables a spec may carry.
//
// An allowlist rather than the whole environment: everything here crosses
// into a long-lived process the user cannot see, and a wholesale copy
// would also overwrite the daemon's own settings with a snapshot that
// goes stale the moment the terminal closes.
func forwardableEnv(environ []string) map[string]string {
	out := make(map[string]string, 32)
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || value == "" {
			continue
		}
		if forwardableEnvKey(key) {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func forwardableEnvKey(key string) bool {
	if reservedSpecEnv(key) {
		return false
	}
	if slices.Contains(forwardEnvExact, key) {
		return true
	}
	for _, prefix := range forwardEnvPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// specBase layers a spec's environment onto the daemon's own, dropping
// anything the daemon owns.
//
// The result is the `base` childEnv works from, so the terminal
// description and the per-session file paths are still applied on top and
// still win.
func specBase(environ []string, spec *SessionSpec) []string {
	if spec == nil || len(spec.Env) == 0 {
		return environ
	}
	overlay := make(map[string]string, len(spec.Env))
	for key, value := range spec.Env {
		if key == "" || reservedSpecEnv(key) {
			continue
		}
		overlay[key] = value
	}
	if len(overlay) == 0 {
		return environ
	}

	out := make([]string, 0, len(environ)+len(overlay))
	for _, kv := range environ {
		key, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, replaced := overlay[key]; replaced {
				continue // the spec's value is appended below
			}
		}
		out = append(out, kv)
	}
	for key, value := range overlay {
		out = append(out, key+"="+value)
	}
	return out
}

// specCommand resolves a spec into the working directory and argument
// list a session child is started with.
//
// The two are decided together because they answer the same question.
// There are three outcomes:
//
//   - No spec, or a directory this daemon cannot use: the picker, rooted
//     in the daemon user's home. A remote peer shares no filesystem with
//     this machine, and a client whose directory has been deleted has
//     named nothing usable, so home is the only honest starting point.
//   - A spec asking for the picker: the picker, rooted where the client
//     is. `kit attach` chose the machine, not the directory.
//   - A spec naming a directory: start there, with the client's own
//     arguments and no picker at all.
//
// Note that an EMPTY argument list is not the same as no spec: a bare
// `kit` in a project directory sends a spec with a directory and nothing
// else, and showing it a picker would defeat the whole exercise.
func specCommand(spec *SessionSpec) (dir string, args []string) {
	if spec == nil {
		return homeDir(), []string{SessionFlag, pickDirFlagName}
	}
	dir, ok := usableDir(spec.Cwd)
	if !ok || spec.Pick {
		// The picker opens in the child's own working directory, which is
		// dir either way: the client's when it named a usable one, and
		// home when it did not.
		return dir, []string{SessionFlag, pickDirFlagName}
	}
	return dir, append([]string{SessionFlag}, spec.Args...)
}

// usableDir reports whether a requested directory can be used, and what
// to use instead when it cannot.
//
// A directory that is gone — the client was run somewhere since deleted,
// or on a path this daemon cannot see — falls back to home rather than
// failing the spawn: the session still starts, and the caller offers the
// picker to choose from.
func usableDir(cwd string) (string, bool) {
	if cwd == "" {
		return homeDir(), false
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return homeDir(), false
	}
	return cwd, true
}

// inheritedSpec is what a spec becomes once it has started a session: the
// directory, the environment and the choice of picker, without the
// arguments.
//
// A second session on the same connection (Ctrl-] c) belongs to the same
// terminal and the same project, so it inherits the directory the way a
// new tmux window does. It must NOT inherit the arguments: replaying
// them would answer "give me a new session" with --resume's picker, or
// re-attach a --continue'd conversation the user has already moved on
// from. Nil when there is nothing left worth keeping.
func inheritedSpec(spec *SessionSpec) *SessionSpec {
	if spec == nil {
		return nil
	}
	if spec.Cwd == "" && len(spec.Env) == 0 {
		return nil
	}
	return &SessionSpec{Cwd: spec.Cwd, Env: spec.Env, Pick: spec.Pick}
}

// specFits reports whether a spec still fits in one frame, and trims it
// until it does.
//
// The environment is the only part that can realistically grow past the
// frame's 64 KiB payload limit, and it is also the part a session can do
// without: losing it costs the provider credentials, which the daemon may
// well have of its own. Losing the directory would silently start the
// session in the wrong place, so the arguments and the directory are kept
// to the last.
func specFits(spec SessionSpec) (SessionSpec, bool) {
	if payload, err := EncodeSessionSpec(spec); err == nil && len(payload) <= maxPayload {
		return spec, true
	}
	spec.Env = nil
	if payload, err := EncodeSessionSpec(spec); err == nil && len(payload) <= maxPayload {
		return spec, true
	}
	return SessionSpec{}, false
}
