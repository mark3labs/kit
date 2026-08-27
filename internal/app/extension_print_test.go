package app

import (
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdStreams runs fn with os.Stdout and os.Stderr redirected to pipes
// and returns what was written to each.
func captureStdStreams(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	origOut, origErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()

	fn()

	_ = outW.Close()
	_ = errW.Close()
	outBytes, _ := io.ReadAll(outR)
	errBytes, _ := io.ReadAll(errR)
	return string(outBytes), string(errBytes)
}

// TestPrintFromExtensionQuietUsesStderr guards the --json / --quiet contract:
// stdout carries only the response payload, so an extension calling ctx.Print
// must not corrupt output that is piped into jq or another JSON consumer.
func TestPrintFromExtensionQuietUsesStderr(t *testing.T) {
	tests := []struct {
		name       string
		quiet      bool
		level      string
		wantStdout bool
	}{
		{name: "plain print in quiet mode", quiet: true, level: "", wantStdout: false},
		{name: "plain print in normal mode", quiet: false, level: "", wantStdout: true},
		{name: "info always on stderr", quiet: false, level: "info", wantStdout: false},
		{name: "error always on stderr", quiet: false, level: "error", wantStdout: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New(Options{Quiet: tt.quiet}, nil)
			defer a.Close()

			const msg = "extension-output-marker"
			stdout, stderr := captureStdStreams(t, func() {
				a.PrintFromExtension(tt.level, msg)
			})

			if tt.wantStdout {
				if !strings.Contains(stdout, msg) {
					t.Errorf("stdout = %q, want it to contain %q", stdout, msg)
				}
				return
			}
			if strings.Contains(stdout, msg) {
				t.Errorf("stdout = %q, want no extension output on stdout", stdout)
			}
			if !strings.Contains(stderr, msg) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, msg)
			}
		})
	}
}
