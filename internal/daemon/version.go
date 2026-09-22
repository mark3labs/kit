package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// The daemon's compatibility contract.
//
// A kit client and a kit daemon are two independent installations of the
// same binary, and there is no rule that says they were released
// together: a package upgrade replaces the binary on disk while the
// daemon started from the old one keeps running, a nix devshell puts a
// different build first on PATH, and a long-lived daemon on a server
// outlives many client releases. All of those must keep working.
//
// So compatibility is decided by ONE number that rarely changes, not by
// the release version:
//
//   - ProtocolVersion is bumped only for a BREAKING change to the wire
//     format — a changed frame header, a changed payload layout, a
//     retired frame that peers still depend on. Two peers that agree on
//     it can always talk.
//   - ProtocolFeatures is a bitmap of ADDITIVE capabilities. A new frame
//     type, a new field, a new session capability: each gets a bit, and
//     the peer that lacks it simply does not get that behaviour. Nothing
//     about it makes the connection unusable, so it must never be
//     confused with a version bump.
//
// The build version travels too, but purely as information: it is what
// `kit daemon status` reports and what a message can quote when a user
// wonders why a brand new feature is missing from a daemon they started
// three weeks ago. It is never a reason to refuse a connection.

const (
	// ProtocolName identifies this protocol on the wire. It exists so a
	// peer speaking something else entirely is rejected with a clear
	// message rather than by a payload that fails to parse.
	ProtocolName = "kit/daemon"

	// ProtocolVersion is the breaking-change counter.
	//
	// Bump this ONLY when an old peer cannot usefully talk to a new one.
	// Everything that can be expressed as a feature bit must be, because
	// every bump here strands every daemon a user has running.
	//
	// v1 is the format the daemon has spoken since the transport moved
	// in-process: a 7-byte frame header (type, u32 wire session, u16
	// length) and the frame numbers in protocol.go.
	ProtocolVersion uint16 = 1
)

// Feature is one additive capability bit. Peers exchange the set they
// support in the hello; behaviour behind a bit the other side lacks is
// simply not used.
type Feature uint64

const (
	// FeatureTerminalInfo: the peer understands FrameTerminal, so a
	// session's child can be told what terminal it renders in.
	FeatureTerminalInfo Feature = 1 << 0
	// FeatureSessionSpec: the peer understands FrameSessionSpec, so a
	// local session starts in the caller's directory with the caller's
	// arguments instead of behind the directory picker.
	FeatureSessionSpec Feature = 1 << 1
	// FeatureTermModes: the daemon replays a session's terminal modes to
	// a client that attaches mid-session.
	FeatureTermModes Feature = 1 << 2
	// FeatureClipboard: the peer understands FrameClipboard image
	// transfer.
	FeatureClipboard Feature = 1 << 3
	// FeatureSessionRename: the peer understands FrameSessionRename.
	FeatureSessionRename Feature = 1 << 4
	// FeatureSessionRedraw: the peer understands FrameSessionRedraw.
	FeatureSessionRedraw Feature = 1 << 5
	// FeatureReattach: this daemon hosts its sessions in separate
	// supervisor processes, so they survive it being stopped, restarted
	// or killed, and it adopts them again when it comes back.
	//
	// A client uses the bit to decide what to say when a connection
	// drops: with it, "the daemon went away, your session is still
	// running, reconnecting"; without it, the session died with the
	// daemon and there is nothing to reconnect to.
	FeatureReattach Feature = 1 << 6
	// FeatureScrollback: an attaching client is sent the session's recent
	// output, so the screen is restored without waiting for the child to
	// repaint.
	FeatureScrollback Feature = 1 << 7
)

// ProtocolFeatures is everything this build supports.
const ProtocolFeatures = FeatureTerminalInfo |
	FeatureSessionSpec |
	FeatureTermModes |
	FeatureClipboard |
	FeatureSessionRename |
	FeatureSessionRedraw |
	FeatureReattach |
	FeatureScrollback

// Has reports whether every bit in want is present.
func (f Feature) Has(want Feature) bool { return f&want == want }

// featureNames labels the bits for logs and `kit daemon status`. A bit
// with no name here is reported by number, which is what a peer from the
// future looks like.
var featureNames = []struct {
	bit  Feature
	name string
}{
	{FeatureTerminalInfo, "terminal-info"},
	{FeatureSessionSpec, "session-spec"},
	{FeatureTermModes, "term-modes"},
	{FeatureClipboard, "clipboard"},
	{FeatureSessionRename, "rename"},
	{FeatureSessionRedraw, "redraw"},
	{FeatureReattach, "reattach"},
	{FeatureScrollback, "scrollback"},
}

// String renders a feature set as a comma-separated list.
func (f Feature) String() string {
	if f == 0 {
		return "none"
	}
	var (
		parts []string
		rest  = f
	)
	for _, fn := range featureNames {
		if f.Has(fn.bit) {
			parts = append(parts, fn.name)
			rest &^= fn.bit
		}
	}
	if rest != 0 {
		parts = append(parts, fmt.Sprintf("%#x", uint64(rest)))
	}
	return strings.Join(parts, ",")
}

// Hello is the first frame each side sends on a connection: who I am,
// which protocol I speak, and what I can do.
//
// JSON rather than a packed struct on purpose. The whole point of this
// frame is that a peer from another release can read it, which means an
// unknown field must be ignorable and a missing field must have a
// sensible zero — exactly what a JSON object gives and a fixed byte
// layout does not.
type Hello struct {
	// Protocol is ProtocolName. A different value is a different program.
	Protocol string `json:"protocol"`
	// Version is ProtocolVersion: the compatibility decision.
	Version uint16 `json:"version"`
	// Features is the sender's ProtocolFeatures bitmap.
	Features Feature `json:"features"`
	// Build is the sender's release version, for display only.
	Build string `json:"build,omitempty"`
	// Role is "client" or "daemon", so a log line says which end it came
	// from without the reader having to infer it.
	Role string `json:"role,omitempty"`
	// PID is the sender's process id, for display only.
	PID int `json:"pid,omitempty"`
}

// Hello roles.
const (
	RoleClient = "client"
	RoleDaemon = "daemon"
)

// localHello builds this build's hello for the given role.
func localHello(role string) Hello {
	return Hello{
		Protocol: ProtocolName,
		Version:  ProtocolVersion,
		Features: ProtocolFeatures,
		Build:    BuildVersion(),
		Role:     role,
		PID:      os.Getpid(),
	}
}

// EncodeHello renders a HELLO payload.
func EncodeHello(h Hello) ([]byte, error) { return json.Marshal(h) }

// DecodeHello parses a HELLO payload.
func DecodeHello(payload []byte) (Hello, error) {
	var h Hello
	if err := json.Unmarshal(payload, &h); err != nil {
		return Hello{}, fmt.Errorf("daemon: bad hello payload: %w", err)
	}
	return h, nil
}

// legacyHello describes a peer that sent no hello at all.
//
// Silence is not an error: every kit released before the hello existed
// connects this way, and it speaks protocol v1 with the frames that were
// defined at the time. Treating it as v1 with no optional features is
// exactly right — each of those frames is additive, and a peer that does
// not know one already drops it.
func legacyHello(role string) Hello {
	return Hello{Protocol: ProtocolName, Version: ProtocolVersion, Role: role}
}

// Compatible reports whether a peer's hello can be served, and why not
// when it cannot.
//
// Only two things are fatal: a different protocol entirely, and a
// different version. Features never are — that is the whole reason they
// are separate.
func (h Hello) Compatible() error {
	if h.Protocol != "" && h.Protocol != ProtocolName {
		return fmt.Errorf("daemon: the peer speaks %q, not %q", h.Protocol, ProtocolName)
	}
	if h.Version != ProtocolVersion {
		return fmt.Errorf(
			"daemon: protocol mismatch — this kit speaks protocol %d, the other end speaks %d%s",
			ProtocolVersion, h.Version, h.buildHint())
	}
	return nil
}

// buildHint names the peer's release in a mismatch message, because the
// fix is almost always to restart the older side.
func (h Hello) buildHint() string {
	if h.Build == "" {
		return ""
	}
	if h.Role == RoleDaemon {
		return fmt.Sprintf(" (the daemon is kit %s; restart it with 'kit daemon restart')", h.Build)
	}
	return fmt.Sprintf(" (the client is kit %s)", h.Build)
}

// buildVersion is the release this binary was built as. main sets it
// through SetBuildVersion; tests and `go run` leave the default.
var buildVersion atomic.Value

// SetBuildVersion records the release version for the hello and for
// `kit daemon status`. It is display-only: compatibility is decided by
// ProtocolVersion, never by this.
func SetBuildVersion(v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		return
	}
	buildVersion.Store(v)
}

// BuildVersion reports the release version recorded by SetBuildVersion.
func BuildVersion() string {
	if v, ok := buildVersion.Load().(string); ok && v != "" {
		return v
	}
	return "dev"
}
