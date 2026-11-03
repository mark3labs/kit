package test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

// preferredList is the option value for the preferred-models tests. The
// model ID of the first model is also the name of the second model.
const preferredList = "openrouter/anthropic/claude-opus-5, anthropic/claude-opus-5 ollama/qwen3"

// loadPreferred loads the preferred-models example and returns its command
// /preferred.
func loadPreferred(t *testing.T) (*Harness, extensions.CommandDef) {
	t.Helper()
	harness := New(t)
	harness.LoadFile("../../../examples/extensions/preferred-models.go")
	for _, cmd := range harness.RegisteredCommands() {
		if cmd.Name == "preferred" && cmd.Execute != nil && cmd.Complete != nil {
			return harness, cmd
		}
	}
	t.Fatal("the example does not register /preferred with Execute and Complete")
	return nil, extensions.CommandDef{}
}

// preferredKit gives /preferred a context as Kit does. active is the model
// in use. GetCurrentProvider and GetCurrentModelID report it, and SetModel
// switches it. ctx.Model is only the model ID until the first switch, and
// provider/model after it. switched records the models given to SetModel.
type preferredKit struct {
	ctx      extensions.Context
	active   string
	switched []string
}

// newPreferredKit returns a preferredKit with active in use. model is the
// value of ctx.Model.
func newPreferredKit(harness *Harness, active, model string) *preferredKit {
	k := &preferredKit{ctx: harness.Context().ToContext(), active: active}
	k.ctx.Model = model
	k.ctx.GetOption = func(name string) string {
		if name == "preferred-models" {
			return preferredList
		}
		return ""
	}
	k.ctx.GetCurrentProvider = func() string {
		provider, _, _ := strings.Cut(k.active, "/")
		return provider
	}
	k.ctx.GetCurrentModelID = func() string {
		_, id, _ := strings.Cut(k.active, "/")
		return id
	}
	k.ctx.SetModel = func(model string) error {
		k.switched = append(k.switched, model)
		k.active = model
		k.ctx.Model = model
		return nil
	}
	return k
}

// picker opens the picker with /preferred, selects the option pick (-1 for
// Esc), and returns the options and the output.
func (k *preferredKit) picker(t *testing.T, cmd extensions.CommandDef, pick int) ([]string, string) {
	t.Helper()
	var shown []string
	k.ctx.PromptSelect = func(cfg extensions.PromptSelectConfig) extensions.PromptSelectResult {
		shown = cfg.Options
		if pick < 0 {
			return extensions.PromptSelectResult{Cancelled: true}
		}
		return extensions.PromptSelectResult{Value: cfg.Options[pick], Index: pick}
	}
	out, err := cmd.Execute("", k.ctx)
	if err != nil {
		t.Fatalf("/preferred: %v", err)
	}
	return shown, out
}

var (
	// markFirst and markSecond are the options of the picker with the
	// first or the second model in use.
	markFirst = []string{
		"1  openrouter/anthropic/claude-opus-5  (current)",
		"2  anthropic/claude-opus-5",
		"3  ollama/qwen3",
	}
	markSecond = []string{
		"1  openrouter/anthropic/claude-opus-5",
		"2  anthropic/claude-opus-5  (current)",
		"3  ollama/qwen3",
	}
)

// TestPreferredModelsExample runs /preferred from the start. The configured
// model is openrouter/anthropic/claude-opus-5, so ctx.Model starts as
// anthropic/claude-opus-5, the name of the second model.
func TestPreferredModelsExample(t *testing.T) {
	harness, cmd := loadPreferred(t)
	k := newPreferredKit(harness, "openrouter/anthropic/claude-opus-5", "anthropic/claude-opus-5")

	if shown, out := k.picker(t, cmd, -1); out != "" || !reflect.DeepEqual(shown, markFirst) {
		t.Fatalf("picker at the start = %q, output %q; want %q and no output", shown, out, markFirst)
	}
	for _, c := range []struct{ args, want string }{
		{"2", "Switched to anthropic/claude-opus-5"},
		{"1", "Switched to openrouter/anthropic/claude-opus-5"},
		{"2", "Switched to anthropic/claude-opus-5"},
		{"2", "Already using anthropic/claude-opus-5"},
	} {
		if out, err := cmd.Execute(c.args, k.ctx); out != c.want || err != nil {
			t.Fatalf("/preferred %s = %q, %v; want %q", c.args, out, err, c.want)
		}
	}
	if shown, _ := k.picker(t, cmd, -1); !reflect.DeepEqual(shown, markSecond) {
		t.Fatalf("picker after a switch = %q, want %q", shown, markSecond)
	}
	if _, out := k.picker(t, cmd, 2); out != "Switched to ollama/qwen3" {
		t.Fatalf("picker, third option = %q, want the switch", out)
	}

	// A part of a name selects the first model that contains it. The match
	// ignores case.
	for _, c := range []struct{ args, want string }{
		{"OPENROUTER", "Switched to openrouter/anthropic/claude-opus-5"},
		{"openrouter", "Already using openrouter/anthropic/claude-opus-5"},
	} {
		if out, err := cmd.Execute(c.args, k.ctx); out != c.want || err != nil {
			t.Errorf("/preferred %s = %q, %v; want %q", c.args, out, err, c.want)
		}
	}
	for _, args := range []string{"0", "4", "gpt"} {
		if _, err := cmd.Execute(args, k.ctx); err == nil || !strings.Contains(err.Error(), "no preferred model matches") {
			t.Errorf("/preferred %s gave error %v, want no match", args, err)
		}
	}
	want := []string{
		"anthropic/claude-opus-5",
		"openrouter/anthropic/claude-opus-5",
		"anthropic/claude-opus-5",
		"ollama/qwen3",
		"openrouter/anthropic/claude-opus-5",
	}
	if !reflect.DeepEqual(k.switched, want) {
		t.Errorf("SetModel calls = %q, want %q", k.switched, want)
	}

	if got := cmd.Complete("anthropic/", k.ctx); !reflect.DeepEqual(got, []string{"anthropic/claude-opus-5"}) {
		t.Errorf("Complete(anthropic/) = %q, want [anthropic/claude-opus-5]", got)
	}
	if got := cmd.Complete("", k.ctx); len(got) != 3 {
		t.Errorf("Complete() = %q, want all three models", got)
	}
}

// TestPreferredModelsExampleAfterReload loads the example after a switch to
// anthropic/claude-opus-5, as /reload-ext does. ctx.Model is then the same
// string as at the start with openrouter/anthropic/claude-opus-5.
func TestPreferredModelsExampleAfterReload(t *testing.T) {
	harness, cmd := loadPreferred(t)
	k := newPreferredKit(harness, "anthropic/claude-opus-5", "anthropic/claude-opus-5")

	if shown, _ := k.picker(t, cmd, -1); !reflect.DeepEqual(shown, markSecond) {
		t.Fatalf("picker after the reload = %q, want %q", shown, markSecond)
	}
	if out, err := cmd.Execute("1", k.ctx); out != "Switched to openrouter/anthropic/claude-opus-5" || err != nil {
		t.Fatalf("/preferred 1 after the reload = %q, %v; want the switch", out, err)
	}
}

// TestPreferredModelsExampleIgnoresInvalidPickerIndex checks that an index
// outside the list switches nothing.
func TestPreferredModelsExampleIgnoresInvalidPickerIndex(t *testing.T) {
	harness, cmd := loadPreferred(t)
	k := newPreferredKit(harness, "ollama/qwen3", "ollama/qwen3")
	for _, pick := range []int{-2, 3, 5} {
		k.ctx.PromptSelect = func(extensions.PromptSelectConfig) extensions.PromptSelectResult {
			return extensions.PromptSelectResult{Index: pick}
		}
		if out, err := cmd.Execute("", k.ctx); out != "" || err != nil {
			t.Errorf("picker index %d = %q, %v; want no output", pick, out, err)
		}
	}
	if len(k.switched) != 0 {
		t.Errorf("SetModel calls = %q, want none", k.switched)
	}
}

// TestPreferredModelsExampleWithoutList checks that /preferred explains the
// option when it is not set, and opens no picker.
func TestPreferredModelsExampleWithoutList(t *testing.T) {
	harness, cmd := loadPreferred(t)
	ctx := harness.Context().ToContext()
	ctx.PromptSelect = func(extensions.PromptSelectConfig) extensions.PromptSelectResult {
		t.Error("the picker opened without a list")
		return extensions.PromptSelectResult{Cancelled: true}
	}
	out, err := cmd.Execute("", ctx)
	if err != nil || !strings.Contains(out, "No preferred models") || !strings.Contains(out, "preferred-models:") {
		t.Errorf("/preferred without the option = %q, %v; want a hint with the option", out, err)
	}
}
