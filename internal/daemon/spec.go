package daemon

import (
	"cmp"
	"fmt"
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
// socket, and this file decides what the daemon keeps for itself.

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
		Cwd:     cwd,
		Args:    slices.Clone(args),
		Env:     clientEnv(os.Environ()),
		FullEnv: true,
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
		Cwd:     cwd,
		Pick:    true,
		Env:     clientEnv(os.Environ()),
		FullEnv: true,
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

// shellLocalEnv are variables that describe the client's shell process or
// its tty, not the environment the user configured. A session child is a
// new process on a new PTY, so it gets its own values or none.
var shellLocalEnv = []string{
	"PWD", "OLDPWD", "SHLVL", "_", "SSH_TTY",
}

// clientEnv selects the variables a spec carries: the client's whole
// environment, less the variables the daemon owns and the ones that
// describe only the client's shell.
//
// The whole environment, not an allowlist. A spec is accepted only on
// the local socket, which admits only the daemon's own user (see
// connSet.setSpec), so the session child runs with the same user and the
// same trust as a kit started in the shell. An allowlist gave no security
// and dropped everything it did not name: tool API keys, direnv and nix
// dev shell variables, toolchain settings.
//
// Empty values are kept: FOO= is a value the user set.
func clientEnv(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, kv := range environ {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			continue
		}
		if reservedSpecEnv(key) || slices.Contains(shellLocalEnv, key) {
			continue
		}
		out[key] = value
	}
	return out
}

// specBase builds the environment a session child starts from, before
// childEnv applies the terminal description and the per-session files.
//
// A spec with FullEnv REPLACES the daemon's environment: the child gets
// the client's variables, plus the daemon's values for the keys the
// daemon owns (its cache and runtime directories, which the crash sweep
// uses to identify its sessions). The daemon's pane variables are not
// kept, because they describe the daemon's terminal, not the client's.
//
// A spec without FullEnv comes from an older client, which sent only an
// allowlist; its Env is layered on the daemon's environment as before.
//
// Either way a reserved key from the client is ignored: the client is the
// untrusted side.
func specBase(environ []string, spec *SessionSpec) []string {
	if spec != nil && spec.FullEnv {
		return fullSpecBase(environ, spec.Env)
	}
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

// fullSpecBase is specBase for a spec that carries the client's complete
// environment.
func fullSpecBase(environ []string, env map[string]string) []string {
	out := make([]string, 0, len(env)+8)
	for _, kv := range environ {
		key, _, ok := strings.Cut(kv, "=")
		if !ok || !reservedSpecEnv(key) || slices.Contains(multiplexerEnv, key) {
			continue
		}
		out = append(out, kv)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		if key == "" || strings.Contains(key, "=") || reservedSpecEnv(key) {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys) // a stable order makes the child's environment reproducible
	for _, key := range keys {
		out = append(out, key+"="+env[key])
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
	if spec.Cwd == "" && len(spec.Env) == 0 && !spec.FullEnv {
		return nil
	}
	return &SessionSpec{Cwd: spec.Cwd, Env: spec.Env, FullEnv: spec.FullEnv, Pick: spec.Pick}
}

// specFrames encodes a spec as the frames to send to a daemon.
//
// A spec that fits in one frame goes as one FrameSessionSpec, which every
// daemon understands. A larger one goes as FrameSessionSpecPart frames
// when the daemon can join them (parts), so the environment arrives
// complete. For an older daemon the environment is trimmed to fit: see
// trimSpec. dropped names the variables trimming removed, so the caller
// can tell the user.
//
// ok is false only when the spec cannot be sent in any form.
func specFrames(spec SessionSpec, parts bool) (frames []Frame, dropped []string, ok bool) {
	payload, err := EncodeSessionSpec(spec)
	if err != nil {
		return nil, nil, false
	}
	if len(payload) <= maxPayload {
		return []Frame{{Type: FrameSessionSpec, Payload: payload}}, nil, true
	}
	if parts && len(payload) <= maxSpecSize {
		return specPartFrames(payload), nil, true
	}
	trimmed, dropped, ok := trimSpec(spec)
	if !ok {
		return nil, dropped, false
	}
	payload, err = EncodeSessionSpec(trimmed)
	if err != nil {
		return nil, dropped, false
	}
	return []Frame{{Type: FrameSessionSpec, Payload: payload}}, dropped, true
}

// specPartFrames splits an encoded spec into FrameSessionSpecPart frames.
func specPartFrames(payload []byte) []Frame {
	const step = maxPayload - 1 // one byte for the marker
	var frames []Frame
	for len(payload) > 0 {
		n := min(step, len(payload))
		marker := specPartMore
		if n == len(payload) {
			marker = specPartLast
		}
		part := make([]byte, 0, n+1)
		part = append(part, marker)
		part = append(part, payload[:n]...)
		frames = append(frames, Frame{Type: FrameSessionSpecPart, Payload: part})
		payload = payload[n:]
	}
	return frames
}

// specJoiner joins FrameSessionSpecPart payloads on the daemon side. One
// joiner belongs to one connection.
type specJoiner struct {
	buf      []byte
	overflow bool
}

// add takes one part. It returns the joined payload when the part is the
// last one, and an error when the parts are malformed or too large. After
// the last part, or after an error, the joiner is empty again.
func (j *specJoiner) add(part []byte) (payload []byte, done bool, err error) {
	if len(part) == 0 {
		j.reset()
		return nil, false, fmt.Errorf("daemon: empty session spec part")
	}
	marker, data := part[0], part[1:]
	if marker != specPartMore && marker != specPartLast {
		j.reset()
		return nil, false, fmt.Errorf("daemon: bad session spec part marker %#x", marker)
	}
	if !j.overflow {
		if len(j.buf)+len(data) > maxSpecSize {
			// Keep reading to the last part, so the parts that follow are
			// not taken as the start of a new spec.
			j.buf, j.overflow = nil, true
		} else {
			j.buf = append(j.buf, data...)
		}
	}
	if marker == specPartMore {
		return nil, false, nil
	}
	payload, overflow := j.buf, j.overflow
	j.reset()
	if overflow {
		return nil, false, fmt.Errorf("daemon: session spec exceeds %d bytes", maxSpecSize)
	}
	return payload, true, nil
}

func (j *specJoiner) reset() { j.buf, j.overflow = nil, false }

// trimSpec makes a spec fit in one frame for a daemon that cannot join
// parts, and reports the variables it removed.
//
// The largest variables go first: they free the most space for the least
// loss, and the large ones (PATH-like lists, dev shell build flags) are
// the ones the daemon's own environment most likely also has. The
// directory and the arguments are kept to the last, because a session in
// the wrong directory is a silent error.
func trimSpec(spec SessionSpec) (SessionSpec, []string, bool) {
	keys := make([]string, 0, len(spec.Env))
	for key := range spec.Env {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if c := cmp.Compare(len(b)+len(spec.Env[b]), len(a)+len(spec.Env[a])); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})

	env := make(map[string]string, len(spec.Env))
	for key, value := range spec.Env {
		env[key] = value
	}
	spec.Env = env

	// A trimmed environment is no longer complete, so it is layered on
	// the daemon's as before rather than replacing it: a dropped PATH
	// must fall back on the daemon's PATH, not on none.
	spec.FullEnv = false

	var dropped []string
	for {
		if payload, err := EncodeSessionSpec(spec); err == nil && len(payload) <= maxPayload {
			slices.Sort(dropped)
			return spec, dropped, true
		}
		if len(keys) == 0 {
			return SessionSpec{}, dropped, false
		}
		delete(env, keys[0])
		dropped = append(dropped, keys[0])
		keys = keys[1:]
	}
}
