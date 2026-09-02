//go:build unix

package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// The clipboard file lives in the daemon's runtime directory, which is
// normally on a different filesystem from the system temp directory.
// Staging the image there and renaming it into place failed with EXDEV,
// and the paste was dropped with only a daemon log line to show for it.
//
// Unix-only: proving two paths are on different filesystems needs stat(2),
// and the two directories used here are Linux conventions.
func TestPublishClipboardImageAcrossFilesystems(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs two filesystems that are known to differ")
	}
	const other = "/dev/shm"
	dest := filepath.Join(t.TempDir(), "clip")
	if sameFilesystem(t, filepath.Dir(dest), other) {
		t.Skipf("%s and the temp dir are one filesystem; nothing to cross", other)
	}

	// The old code staged in the system temp directory. Pointing that at a
	// different filesystem reproduces the failure a real daemon hits.
	t.Setenv("TMPDIR", other)

	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1, 2, 3}
	if err := publishClipboardImage(dest, "run-1", png); err != nil {
		t.Fatalf("publish across filesystems: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read published image: %v", err)
	}
	if !bytes.Equal(got, png) {
		t.Fatalf("published %d bytes, want %d", len(got), len(png))
	}

	// A rewrite replaces the image rather than appending to it.
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0x01}
	if err := publishClipboardImage(dest, "run-1", jpeg); err != nil {
		t.Fatalf("republish: %v", err)
	}
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, jpeg) {
		t.Fatalf("republish left %v, want %v", got, jpeg)
	}

	// No staging file is left beside the published one.
	entries, err := os.ReadDir(filepath.Dir(dest))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "stage") {
			t.Fatalf("staging file left behind: %s", e.Name())
		}
	}
}

// sameFilesystem reports whether two directories live on one device.
func sameFilesystem(t *testing.T, a, b string) bool {
	t.Helper()
	var sa, sb syscall.Stat_t
	if err := syscall.Stat(a, &sa); err != nil {
		t.Skipf("stat %s: %v", a, err)
	}
	if err := syscall.Stat(b, &sb); err != nil {
		t.Skipf("stat %s: %v", b, err)
	}
	return sa.Dev == sb.Dev
}
