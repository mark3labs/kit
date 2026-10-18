package kit

import (
	"strings"
	"testing"
)

// TestGetSystemPrompt checks that GetSystemPrompt reports the prompt the
// agent sends to the model. That is the prompt that New composed, and after
// a runtime change the recomposed prompt. The configuration store keeps the
// prompt from New, so a getter that read the store would miss the runtime
// change.
func TestGetSystemPrompt(t *testing.T) {
	k := newBareTestKit(t, false)

	got := k.GetSystemPrompt()
	if want := k.v.GetString("system-prompt"); got != want {
		t.Errorf("GetSystemPrompt() differs from the prompt composed by New\n got: %q\nwant: %q", got, want)
	}
	if !strings.Contains(got, "Always speak in Latin.") {
		t.Errorf("GetSystemPrompt() does not contain the AGENTS.md content:\n%s", got)
	}

	const path = "/runtime/AGENTS.md"
	const marker = "RUNTIME-CONTEXT-MARKER"
	if err := k.AddContextFile(&ContextFile{Path: path, Content: marker}); err != nil {
		t.Fatalf("AddContextFile: %v", err)
	}
	if !strings.Contains(k.GetSystemPrompt(), marker) {
		t.Error("GetSystemPrompt() does not report a context file added at runtime")
	}

	if !k.RemoveContextFile(path) {
		t.Fatalf("RemoveContextFile(%q) = false; want true", path)
	}
	if strings.Contains(k.GetSystemPrompt(), marker) {
		t.Error("GetSystemPrompt() still reports a context file removed at runtime")
	}
}
