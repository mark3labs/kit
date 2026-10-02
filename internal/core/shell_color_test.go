package core

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

// A program that emits colour unconditionally must still reach the transcript
// coloured while the model sees plain text. The shell tool does not force
// colour any more (see shellEnv), so this is about the split rather than about
// the environment.
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

// probeColorEnv is the command used to report which colour variables a child
// actually sees.
const probeColorEnv = `printf 'C=[%s] F=[%s] CF=[%s]\n' "$COLORTERM" "$FORCE_COLOR" "$CLICOLOR_FORCE"`

// clearColorEnv empties the three variables that drive colour, so a test starts
// from a known baseline.
//
// A user who runs Kit from a terminal that sets COLORTERM hands that value to
// every child through the inherited environment, and shellEnv passes the
// environment through on purpose. So "the variable is set" cannot distinguish
// "Kit forced it" from "the user's shell already had it"; only the change from
// a known-empty baseline can.
func clearColorEnv(t *testing.T) {
	t.Helper()
	t.Setenv("COLORTERM", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
}

// probeColorFields is how many fields probeColorEnv prints: one per colour
// variable.
const probeColorFields = 3

// colorProbe reports which of the three colour variables a child saw set.
type colorProbe struct {
	set   []string
	unset []string
}

// probeColor parses the output of probeColorEnv, which has one "NAME=[value]"
// field per colour variable.
//
// The fields are split rather than searched for: "F=[]" occurs inside "CF=[]",
// so a substring search reports every field as unset whenever the last one is
// empty. Only the name half of each field is recorded, so the caller compares
// like with like.
func probeColor(output string) colorProbe {
	var p colorProbe
	for field := range strings.FieldsSeq(output) {
		name, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		// A value is present unless the brackets are empty.
		if strings.Trim(value, "[]") == "" {
			p.unset = append(p.unset, name)
			continue
		}
		p.set = append(p.set, name)
	}
	return p
}

// wantsColor reports whether all three variables were set, and names the ones
// that were not.
//
// It has to be all-or-nothing in both directions, and it has to require all
// three fields to be present at all. A weaker "at least one was set" check
// would pass a command that forced only COLORTERM, and a weaker "at least one was
// empty" check would pass one that forced only FORCE_COLOR — and a single
// forced variable is enough to make git or ripgrep emit colour into a file or a
// pipe. Requiring all three also stops an empty or truncated response from
// passing vacuously: with no fields at all both lists are empty, and checking
// only one of them cannot tell that from the expected state.
func wantsColor(output string) (bool, []string) {
	p := probeColor(output)
	return len(p.set) == probeColorFields && len(p.unset) == 0, p.unset
}

// wantsNoColor reports whether every variable was left alone, and names the ones
// that were not. It requires all three fields for the same reason.
func wantsNoColor(output string) (bool, []string) {
	p := probeColor(output)
	return len(p.unset) == probeColorFields && len(p.set) == 0, p.set
}

func TestProbeColorParsesFieldsWholesale(t *testing.T) {
	tests := []struct {
		name      string
		out       string
		wantSet   []string
		wantUnset []string
	}{
		{
			name:      "none set",
			out:       "C=[] F=[] CF=[]\n",
			wantUnset: []string{"C", "F", "CF"},
		},
		{
			name:    "all set",
			out:     "C=[truecolor] F=[1] CF=[1]\n",
			wantSet: []string{"C", "F", "CF"},
		},
		{
			// The case a substring search gets wrong: "F=[]" occurs inside
			// "CF=[]", so checking for the shorter field name would report all
			// three as unset.
			name:      "only the middle set",
			out:       "C=[] F=[1] CF=[]\n",
			wantSet:   []string{"F"},
			wantUnset: []string{"C", "CF"},
		},
		{
			name:      "only the last set",
			out:       "C=[] F=[] CF=[1]\n",
			wantSet:   []string{"CF"},
			wantUnset: []string{"C", "F"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeColor(tt.out)
			if !slices.Equal(got.set, tt.wantSet) {
				t.Errorf("set = %v, want %v", got.set, tt.wantSet)
			}
			if !slices.Equal(got.unset, tt.wantUnset) {
				t.Errorf("unset = %v, want %v", got.unset, tt.wantUnset)
			}
		})
	}
}

// An empty or truncated response must fail both helpers. Otherwise a shell
// that printed nothing would satisfy "the tool did not force colour" without a
// single field having been inspected.
func TestColorAssertionsRejectIncompleteOutput(t *testing.T) {
	incomplete := []string{
		"",
		"\n",
		"C=[truecolor]\n",
		"C=[] F=[]\n",
		"C=[truecolor] F=[1] CF=[]\n", // still a valid report
	}
	for _, out := range incomplete {
		if ok, _ := wantsColor(out); ok {
			t.Errorf("wantsColor accepted %q as fully coloured", out)
		}
		if ok, _ := wantsNoColor(out); ok {
			t.Errorf("wantsNoColor accepted %q as fully plain", out)
		}
	}
}

// The shell tool must not force colour. These variables are not scoped to the
// child: they tell every process in the tree that a terminal is attached, so
// forcing them for a model-issued command writes escape bytes into files and
// into the output of any pipe inside the command line. Nothing downstream can
// repair that — the tool result is stripped on the way out, but a file on disk
// and an intermediate pipe are not.
func TestShellToolDoesNotForceColour(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	clearColorEnv(t)

	resp, err := executeShell(context.Background(),
		shellCall(probeColorEnv, 0), "", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}
	if forced, set := wantsNoColor(resp.Content); !forced {
		t.Errorf("the shell tool forced %v: %q", set, resp.Content)
	}
}

// A redirect inside a model-issued command must receive plain bytes. This is the
// failure the tool path exists to avoid: `some-tool > file` would otherwise
// leave escape sequences in a file the model reads back later.
func TestShellToolRedirectReceivesPlainBytes(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	clearColorEnv(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")
	// A program that colours whenever it is told to, written so that the only
	// thing deciding its behaviour is the environment kit handed it.
	script := filepath.Join(dir, "colour.sh")
	body := `#!/bin/sh
if [ -n "$FORCE_COLOR" ] || [ -n "$CLICOLOR_FORCE" ]; then
  printf '\033[31mdanger\033[0m\n'
else
  printf 'danger\n'
fi
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}

	if _, err := executeShell(context.Background(),
		shellCall(script+" > "+target, 0), dir, nil, defaultShellTimeout, maxShellTimeout); err != nil {
		t.Fatalf("executeShell: %v", err)
	}

	written, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read redirect target: %v", err)
	}
	if strings.ContainsRune(string(written), 0x1b) {
		t.Errorf("a redirect received escape sequences: %q", written)
	}
	if !strings.Contains(string(written), "danger") {
		t.Errorf("the redirect lost its content: %q", written)
	}
}

// The bang path is the one that wants colour: a person is reading the output.
func TestBangCommandForcesColour(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	clearColorEnv(t)

	res, err := RunShellCommand(context.Background(), ShellRunOptions{Command: probeColorEnv})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if forced, missing := wantsColor(res.Output); !forced {
		t.Errorf("the bang path did not force %v: %q", missing, res.Output)
	}
}

// NO_COLOR is honoured by not forcing colour, and not by putting a conflicting
// FORCE_COLOR next to the user's NO_COLOR. The program here reports what it
// sees either way, so this pins down the environment and nothing else.
func TestBangCommandRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	clearColorEnv(t)

	res, err := RunShellCommand(context.Background(), ShellRunOptions{Command: probeColorEnv})
	if err != nil {
		t.Fatalf("RunShellCommand: %v", err)
	}
	if forced, set := wantsNoColor(res.Output); !forced {
		t.Errorf("%v was forced despite NO_COLOR: %q", set, res.Output)
	}
}

// A user's own colour variables pass through untouched. Kit declines to force
// colour on the tool path but must not strip what the user set either, or a
// deliberate COLORTERM would silently stop working.
func TestShellToolPassesThroughUserColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("COLORTERM", "truecolor")
	t.Setenv("FORCE_COLOR", "3")
	t.Setenv("CLICOLOR_FORCE", "2")

	resp, err := executeShell(context.Background(),
		shellCall(probeColorEnv, 0), "", nil, defaultShellTimeout, maxShellTimeout)
	if err != nil {
		t.Fatalf("executeShell: %v", err)
	}
	for _, want := range []string{"C=[truecolor]", "F=[3]", "CF=[2]"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("the user's value did not survive: want %q in %q", want, resp.Content)
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
