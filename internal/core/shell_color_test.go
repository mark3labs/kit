package core

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// The two consumers of shell tool output need opposite treatments, and this is
// the property that keeps them separate: the transcript gets the colour, the
// model gets plain text.
//
// It is the whole reason the shell forces colour. Without it the program emits
// nothing, and there is nothing for the two paths to disagree about.
func TestShellToolColourReachesStreamButNotModel(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	var mu sync.Mutex
	var streamed strings.Builder
	ctx := ContextWithToolOutputCallback(context.Background(), func(_, _, chunk string, _ bool) {
		mu.Lock()
		streamed.WriteString(chunk)
		mu.Unlock()
	})

	resp, err := executeShell(ctx, shellCall(`printf '\033[31mRED\033[0m plain\n'`, 0), "", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}

	mu.Lock()
	stream := streamed.String()
	mu.Unlock()

	if !strings.ContainsRune(stream, 0x1b) {
		t.Errorf("the transcript lost the colour: %q", stream)
	}
	if strings.ContainsRune(resp.Content, 0x1b) {
		t.Errorf("the model-facing response kept escape sequences: %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "RED plain") {
		t.Errorf("the model-facing response lost text: %q", resp.Content)
	}
}

// Colour is only forced when the user has not asked for none. The program here
// emits colour unconditionally, so what the test actually pins down is that kit
// did not put a conflicting FORCE_COLOR next to the user's NO_COLOR.
func TestShellToolRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	resp, err := executeShell(context.Background(),
		shellCall(`printf 'COLORTERM=[%s] CLICOLOR_FORCE=[%s] FORCE_COLOR=[%s]\n' "$COLORTERM" "$CLICOLOR_FORCE" "$FORCE_COLOR"`, 0),
		"", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}

	for _, name := range []string{"COLORTERM", "CLICOLOR_FORCE", "FORCE_COLOR"} {
		if !strings.Contains(resp.Content, name+"=[]") {
			t.Errorf("%s was forced despite NO_COLOR: %q", name, resp.Content)
		}
	}
}

// The non-interactive environment has to reach the child: a command that opens
// an editor or a pager has no terminal to do it with and would hang until the
// timeout.
func TestShellToolForcesNonInteractiveEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "")

	resp, err := executeShell(context.Background(),
		shellCall(`echo "EDITOR=$EDITOR PAGER=$PAGER GIT_PAGER=$GIT_PAGER TERM=$TERM"`, 0),
		"", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}

	for _, want := range []string{"EDITOR=false", "PAGER=cat", "GIT_PAGER=cat", "TERM=xterm-256color"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("missing %q in %q", want, resp.Content)
		}
	}
}

// A user value for one of the forced variables must be replaced, not appended
// after: the child would otherwise see two values for one name and the winner
// would depend on its own lookup.
func TestShellToolReplacesUserEditor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("EDITOR", "nvim")

	resp, err := executeShell(context.Background(),
		shellCall(`echo "EDITOR=$EDITOR"`, 0), "", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}
	if strings.Contains(resp.Content, "nvim") {
		t.Errorf("the user's EDITOR reached a non-interactive child: %q", resp.Content)
	}
}
