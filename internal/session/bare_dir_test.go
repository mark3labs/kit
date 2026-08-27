package session

import (
	"path/filepath"
	"testing"
)

// TestDefaultSessionDir_BareBucketCannotCollide is a regression test for the
// bare session bucket sharing a directory with a real working directory
// (CodeRabbit review on #109).
//
// encodeCwdForDir is lossy: it strips a leading separator, so "/__bare__" and
// "__bare__" both encode to "__bare__". Routing bare sessions through the
// encoder therefore let a project at /__bare__ land in the bare bucket, where
// `--continue` could resume the wrong conversation.
func TestDefaultSessionDir_BareBucketCannotCollide(t *testing.T) {
	bare := DefaultSessionDir(BareSessionKey)

	// The premise the bug rested on: these two cwds are indistinguishable
	// after encoding. If this ever stops being true the encoder changed and
	// this test should be revisited rather than deleted.
	if encodeCwdForDir("/"+BareSessionKey) != encodeCwdForDir(BareSessionKey) {
		t.Fatalf("premise changed: %q and %q no longer encode alike",
			"/"+BareSessionKey, BareSessionKey)
	}

	// Despite that, the directories must differ.
	collider := DefaultSessionDir("/" + BareSessionKey)
	if bare == collider {
		t.Errorf("a project at /%s shares the bare bucket %q", BareSessionKey, bare)
	}

	// Bare must live outside the cwd-keyed namespace entirely, which is what
	// makes the separation structural rather than a special case per input.
	if filepath.Base(filepath.Dir(bare)) == "sessions" {
		t.Errorf("bare bucket %q is inside the cwd-keyed sessions/ namespace", bare)
	}
	if filepath.Base(filepath.Dir(collider)) != "sessions" {
		t.Errorf("ordinary cwd %q should live under sessions/", collider)
	}
}

// TestDefaultSessionDir_OrdinaryCwdsUnchanged guards the existing on-disk
// convention: real working directories must keep resolving to
// ~/.kit/sessions/<encoded-cwd>, so the bare special case does not orphan
// anyone's existing sessions.
func TestDefaultSessionDir_OrdinaryCwdsUnchanged(t *testing.T) {
	for _, cwd := range []string{"/home/u/proj", "/tmp", `C:\work\repo`} {
		got := DefaultSessionDir(cwd)
		want := filepath.Base(got)
		if want != encodeCwdForDir(cwd) {
			t.Errorf("DefaultSessionDir(%q) basename = %q, want %q", cwd, want, encodeCwdForDir(cwd))
		}
		if filepath.Base(filepath.Dir(got)) != "sessions" {
			t.Errorf("DefaultSessionDir(%q) = %q, want it under sessions/", cwd, got)
		}
	}
}

// TestFindSessionPathByID_FindsBareSessions is a regression test for bare
// sessions becoming unreachable by ID (CodeRabbit review on #109).
//
// Moving the bare bucket out of ~/.kit/sessions fixed the cwd collision but
// put it beyond the reach of FindSessionPathByID, which scans only the cwd
// directory and the sessions/ subtree. A subagent started in bare mode could
// then not be resumed by SessionID.
func TestFindSessionPathByID_FindsBareSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows

	tm, err := CreateTreeSession(BareSessionKey)
	if err != nil {
		t.Fatalf("CreateTreeSession: %v", err)
	}
	id := tm.GetSessionID()
	_ = tm.Close()

	// Look up from an unrelated working directory, as Kit.Subagent does:
	// it passes the parent's real cwd, not the bare sentinel.
	got, err := FindSessionPathByID(filepath.Join(home, "some", "project"), id)
	if err != nil {
		t.Fatalf("bare session %q not found by ID: %v", id, err)
	}
	if filepath.Base(filepath.Dir(got)) != bareSessionDirName {
		t.Errorf("resolved %q, want a file in %s/", got, bareSessionDirName)
	}
}

// TestFindSessionPathByID_MissingIDStillErrors guards the negative path: the
// added bare-bucket fallback must not turn "not found" into a false match.
func TestFindSessionPathByID_MissingIDStillErrors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if _, err := FindSessionPathByID(home, "nonexistent-id"); err == nil {
		t.Error("expected an error for an unknown session ID")
	}
}
