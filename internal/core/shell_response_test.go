package core

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The tool result goes to the model, and the shell now forces colour. Those
// bytes must not reach the context window.
func TestBuildShellResponseStripsANSI(t *testing.T) {
	resp, err := buildShellResponse("\x1b[31mRED\x1b[0m plain\n", "", 0)
	if err != nil {
		t.Fatalf("buildShellResponse: %v", err)
	}
	if strings.ContainsRune(resp.Content, 0x1b) {
		t.Errorf("escape sequences reached the tool result: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "RED plain") {
		t.Errorf("text lost: %q", resp.Content)
	}
}

func TestBuildShellResponseStripsStderrANSI(t *testing.T) {
	resp, err := buildShellResponse("", "\x1b[31mboom\x1b[0m\n", 1)
	if err != nil {
		t.Fatalf("buildShellResponse: %v", err)
	}
	if strings.ContainsRune(resp.Content, 0x1b) {
		t.Errorf("escape sequences reached the stderr section: %q", resp.Content)
	}
	for _, want := range []string{"STDERR:", "boom", "Exit code: 1"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("missing %q in %q", want, resp.Content)
		}
	}
}

// Truncation happens on the stripped text, so a result that is mostly escape
// sequences is not mistaken for a small one.
func TestBuildShellResponseTruncationSeesStrippedLength(t *testing.T) {
	decorated := "\x1b[1m\x1b[31m\x1b[0m" + strings.Repeat("x", 100) + "\x1b[0m\n"
	resp, err := buildShellResponse(decorated, "", 0)
	if err != nil {
		t.Fatalf("buildShellResponse: %v", err)
	}
	if strings.ContainsRune(resp.Content, 0x1b) {
		t.Errorf("escape sequences survived: %q", resp.Content)
	}
	if got := ansi.Strip(resp.Content); !strings.Contains(got, "xxx") {
		t.Errorf("content lost: %q", got)
	}
}
