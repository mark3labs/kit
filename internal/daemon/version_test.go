package daemon

import (
	"strings"
	"testing"
)

// The protocol constant is the promise this package makes to every kit
// binary a user might have installed: same number, same conversation,
// whatever release either side came from. These tests are what stops that
// promise being broken by accident.

// TestProtocolVersionIsStable is a deliberate speed bump.
//
// Bumping ProtocolVersion strands every daemon a user currently has
// running: their sessions become unreachable from the new client and the
// only fix is to stop the daemon, which is exactly the work this whole
// design exists to avoid. Almost everything that feels like it needs a
// bump is additive and belongs in a feature bit instead.
//
// If you are changing this test, make sure you have first ruled out a
// feature bit — and then bump remoteALPN with it.
func TestProtocolVersionIsStable(t *testing.T) {
	if ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion = %d, want 1.\n"+
			"Bumping it makes every running daemon unreachable to this client. "+
			"An ADDITIVE change (a new frame, a new field, a new capability) "+
			"belongs in a Feature bit, which costs nobody anything.",
			ProtocolVersion)
	}
}

// TestRemoteALPNMatchesProtocolVersion pins the two places the version is
// written down to each other.
//
// The ALPN is compared byte for byte by peers compiled years apart, so it
// cannot be built at run time — which means it can silently fall behind a
// version bump. A daemon advertising kit/remote/1 while claiming to speak
// protocol 2 would accept connections and then refuse every one of them
// in the handshake.
func TestRemoteALPNMatchesProtocolVersion(t *testing.T) {
	want := "kit/remote/1"
	if ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion is %d: update remoteALPN and this test together", ProtocolVersion)
	}
	if remoteALPN != want {
		t.Fatalf("remoteALPN = %q, want %q", remoteALPN, want)
	}
	if remoteProtocolVersion != ProtocolVersion {
		t.Fatalf("remoteProtocolVersion = %d, ProtocolVersion = %d: "+
			"the local socket and the iroh transport must agree on what compatible means",
			remoteProtocolVersion, ProtocolVersion)
	}
}

// TestHelloCompatibility walks every answer Compatible has to give.
func TestHelloCompatibility(t *testing.T) {
	cases := []struct {
		name  string
		hello Hello
		ok    bool
	}{
		{
			"this build talks to itself",
			localHello(RoleDaemon),
			true,
		},
		{
			// The case that matters most: a daemon from an older release
			// that knows none of today's features. Same protocol, so it
			// is served, and the missing features simply go unused.
			"an older release with no features",
			Hello{Protocol: ProtocolName, Version: ProtocolVersion, Build: "v0.1.0"},
			true,
		},
		{
			// And the mirror image: a peer from the future advertising
			// bits this build has never heard of. Unknown features are
			// not a reason to refuse anything.
			"a newer release with unknown features",
			Hello{Protocol: ProtocolName, Version: ProtocolVersion, Features: Feature(1) << 40},
			true,
		},
		{
			// Silence. Every kit released before the hello existed looks
			// like this, and all of them must keep working.
			"a peer that sent no hello at all",
			legacyHello(RoleDaemon),
			true,
		},
		{
			"a different protocol version",
			Hello{Protocol: ProtocolName, Version: ProtocolVersion + 1},
			false,
		},
		{
			"a different protocol entirely",
			Hello{Protocol: "something/else", Version: ProtocolVersion},
			false,
		},
		{
			// A zero value is not a peer that said nothing (that is
			// legacyHello); it is a payload that parsed to nothing, and
			// version 0 is not a version we speak.
			"an empty hello",
			Hello{},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.hello.Compatible()
			if tc.ok && err != nil {
				t.Fatalf("Compatible() = %v, want nil", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("Compatible() = nil, want an error")
			}
		})
	}
}

// TestHelloMismatchNamesTheFix checks that a rejection tells the user
// what to do about it. A bare "protocol mismatch" leaves them guessing
// which of the two kits is the old one.
func TestHelloMismatchNamesTheFix(t *testing.T) {
	daemonSide := Hello{
		Protocol: ProtocolName,
		Version:  ProtocolVersion + 1,
		Build:    "v9.9.9",
		Role:     RoleDaemon,
	}
	err := daemonSide.Compatible()
	if err == nil {
		t.Fatal("a version mismatch must be an error")
	}
	msg := err.Error()
	for _, want := range []string{"v9.9.9", "kit daemon restart"} {
		if !strings.Contains(msg, want) {
			t.Errorf("mismatch message does not mention %q: %s", want, msg)
		}
	}
}

// TestHelloRoundTrip checks the wire encoding survives a peer that does
// not know one of the fields, which is the whole reason it is JSON.
func TestHelloRoundTrip(t *testing.T) {
	want := localHello(RoleClient)
	payload, err := EncodeHello(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeHello(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.Protocol != want.Protocol || got.Version != want.Version ||
		got.Features != want.Features || got.Build != want.Build {
		t.Fatalf("round trip changed the hello: %+v -> %+v", want, got)
	}

	// A payload with an unknown field must still parse: that is a peer
	// from a later release, and refusing it would make every future
	// addition a breaking change.
	fromTheFuture := []byte(`{"protocol":"kit/daemon","version":1,"features":3,"something_new":true}`)
	future, err := DecodeHello(fromTheFuture)
	if err != nil {
		t.Fatalf("a hello with an unknown field was rejected: %v", err)
	}
	if err := future.Compatible(); err != nil {
		t.Fatalf("a hello with an unknown field was declared incompatible: %v", err)
	}
}

// TestFeatureBits checks the bitmap does what the compatibility story
// depends on: bits are independent, and Has is an all-of test.
func TestFeatureBits(t *testing.T) {
	if !ProtocolFeatures.Has(FeatureReattach) {
		t.Error("this build hosts sessions separately but does not advertise FeatureReattach")
	}
	if !ProtocolFeatures.Has(FeatureSessionSpec | FeatureTerminalInfo) {
		t.Error("Has must accept a combination of bits")
	}

	var none Feature
	if none.Has(FeatureReattach) {
		t.Error("an empty feature set must not claim a feature")
	}
	one := FeatureClipboard
	if one.Has(FeatureClipboard | FeatureReattach) {
		t.Error("Has must require EVERY requested bit, not any of them")
	}

	// Every named bit must be distinct, or two capabilities would be
	// negotiated as one.
	seen := map[Feature]string{}
	for _, fn := range featureNames {
		if prev, dup := seen[fn.bit]; dup {
			t.Errorf("feature bit %#x is used by both %q and %q", uint64(fn.bit), prev, fn.name)
		}
		seen[fn.bit] = fn.name
	}
}

// TestFeatureString keeps the log and status output readable, including
// for a peer advertising bits this build does not know.
func TestFeatureString(t *testing.T) {
	if got := Feature(0).String(); got != "none" {
		t.Errorf("empty feature set renders as %q, want \"none\"", got)
	}
	got := (FeatureReattach | FeatureClipboard).String()
	if !strings.Contains(got, "reattach") || !strings.Contains(got, "clipboard") {
		t.Errorf("feature string %q does not name its bits", got)
	}
	unknown := (Feature(1) << 63).String()
	if unknown == "none" || !strings.HasPrefix(unknown, "0x") {
		t.Errorf("an unknown bit renders as %q, want a hex value", unknown)
	}
}

// TestBuildVersionIsDisplayOnly guards the separation the whole design
// rests on: the release version must never reach a compatibility
// decision.
func TestBuildVersionIsDisplayOnly(t *testing.T) {
	a := Hello{Protocol: ProtocolName, Version: ProtocolVersion, Build: "v1.0.0"}
	b := Hello{Protocol: ProtocolName, Version: ProtocolVersion, Build: "v2.0.0"}
	if err := a.Compatible(); err != nil {
		t.Fatalf("build v1.0.0 was refused: %v", err)
	}
	if err := b.Compatible(); err != nil {
		t.Fatalf("build v2.0.0 was refused: %v", err)
	}
}

// TestSetBuildVersion checks the default and that blank input cannot
// erase a version already recorded.
func TestSetBuildVersion(t *testing.T) {
	prev := BuildVersion()
	t.Cleanup(func() { buildVersion.Store(prev) })

	SetBuildVersion("v1.2.3")
	if got := BuildVersion(); got != "v1.2.3" {
		t.Fatalf("BuildVersion() = %q, want v1.2.3", got)
	}
	SetBuildVersion("   ")
	if got := BuildVersion(); got != "v1.2.3" {
		t.Fatalf("a blank version overwrote a real one: %q", got)
	}
}

// TestSessionHostInfoCompatibility mirrors the client rule for the other
// long-lived link: a supervisor from another protocol version is refused,
// and refusing must never be read as permission to kill it.
func TestSessionHostInfoCompatibility(t *testing.T) {
	ok := SessionHostInfo{Protocol: ProtocolName, Version: ProtocolVersion}
	if err := ok.Compatible(); err != nil {
		t.Fatalf("a matching supervisor was refused: %v", err)
	}
	bad := SessionHostInfo{Protocol: ProtocolName, Version: ProtocolVersion + 1}
	if err := bad.Compatible(); err == nil {
		t.Fatal("a supervisor from another protocol version must not be adopted")
	}
}
