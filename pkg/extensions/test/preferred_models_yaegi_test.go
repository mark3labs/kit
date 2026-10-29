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

// preferredContext returns a context for /preferred as Kit gives it. The
// configured model is openrouter/anthropic/claude-opus-5, and model is
// ctx.Model. GetModelCapabilities("") describes the configured model, also
// after a switch.
func preferredContext(harness *Harness, model string) extensions.Context {
	ctx := harness.Context().ToContext()
	ctx.Model = model
	ctx.GetOption = func(name string) string {
		if name == "preferred-models" {
			return preferredList
		}
		return ""
	}
	ctx.GetModelCapabilities = func(name string) (extensions.ModelCapabilities, string) {
		if name != "" {
			return extensions.ModelCapabilities{}, "model not found in registry"
		}
		return extensions.ModelCapabilities{Provider: "openrouter", ModelID: "anthropic/claude-opus-5"}, ""
	}
	return ctx
}

// TestPreferredModelsExample runs /preferred as Kit does. At startup,
// ctx.Model is only the model ID: anthropic/claude-opus-5, which is also the
// name of the second model. After a switch, ctx.Model is provider/model, and
// Kit sends a ModelChangeEvent.
func TestPreferredModelsExample(t *testing.T) {
	harness, cmd := loadPreferred(t)

	var switched []string // the models given to SetModel
	var shown []string    // the options of the last picker
	pick := -1            // the option that the picker selects, -1 for Esc
	ctx := preferredContext(harness, "anthropic/claude-opus-5")
	// switchModel does what Kit does for a switch by SetModel or by /model.
	switchModel := func(model, source string) error {
		previous := ctx.Model
		ctx.Model = model
		_, err := harness.Emit(extensions.ModelChangeEvent{NewModel: model, PreviousModel: previous, Source: source})
		return err
	}
	ctx.SetModel = func(model string) error {
		switched = append(switched, model)
		return switchModel(model, "extension")
	}
	ctx.PromptSelect = func(cfg extensions.PromptSelectConfig) extensions.PromptSelectResult {
		shown = cfg.Options
		if pick < 0 {
			return extensions.PromptSelectResult{Cancelled: true}
		}
		return extensions.PromptSelectResult{Value: cfg.Options[pick], Index: pick}
	}
	run := func(args string) (string, error) {
		return cmd.Execute(args, ctx)
	}
	// checkPicker opens the picker, closes it with Esc, and compares the
	// options.
	checkPicker := func(when string, want []string) {
		t.Helper()
		pick = -1
		if out, err := run(""); out != "" || err != nil {
			t.Fatalf("/preferred with Esc %s = %q, %v; want no output", when, out, err)
		}
		if !reflect.DeepEqual(shown, want) {
			t.Fatalf("options %s = %q, want %q", when, shown, want)
		}
	}
	onFirst := []string{
		"1  openrouter/anthropic/claude-opus-5  (current)",
		"2  anthropic/claude-opus-5",
		"3  ollama/qwen3",
	}
	onSecond := []string{
		"1  openrouter/anthropic/claude-opus-5",
		"2  anthropic/claude-opus-5  (current)",
		"3  ollama/qwen3",
	}

	checkPicker("at startup", onFirst)
	for _, c := range []struct{ args, want string }{
		{"2", "Switched to anthropic/claude-opus-5"},
		{"1", "Switched to openrouter/anthropic/claude-opus-5"},
		{"2", "Switched to anthropic/claude-opus-5"},
	} {
		if out, err := run(c.args); out != c.want || err != nil {
			t.Fatalf("/preferred %s = %q, %v; want %q", c.args, out, err, c.want)
		}
	}
	checkPicker("after a switch", onSecond)

	pick = 2
	if out, err := run(""); out != "Switched to ollama/qwen3" || err != nil {
		t.Fatalf("/preferred, third option = %q, %v; want the switch", out, err)
	}
	// A switch by /model moves the mark too.
	if err := switchModel("anthropic/claude-opus-5", "user"); err != nil {
		t.Fatal(err)
	}
	checkPicker("after /model", onSecond)

	// A part of a name selects the first model that contains it. The match
	// ignores case.
	for _, c := range []struct{ args, want string }{
		{"OPENROUTER", "Switched to openrouter/anthropic/claude-opus-5"},
		{"openrouter", "Already using openrouter/anthropic/claude-opus-5"},
	} {
		if out, err := run(c.args); out != c.want || err != nil {
			t.Errorf("/preferred %s = %q, %v; want %q", c.args, out, err, c.want)
		}
	}
	for _, args := range []string{"0", "4", "gpt"} {
		if _, err := run(args); err == nil || !strings.Contains(err.Error(), "no preferred model matches") {
			t.Errorf("/preferred %s gave error %v, want no match", args, err)
		}
	}
	wantSwitched := []string{
		"anthropic/claude-opus-5",
		"openrouter/anthropic/claude-opus-5",
		"anthropic/claude-opus-5",
		"ollama/qwen3",
		"openrouter/anthropic/claude-opus-5",
	}
	if !reflect.DeepEqual(switched, wantSwitched) {
		t.Errorf("SetModel calls = %q, want %q", switched, wantSwitched)
	}

	if got := cmd.Complete("anthropic/", ctx); !reflect.DeepEqual(got, []string{"anthropic/claude-opus-5"}) {
		t.Errorf("Complete(anthropic/) = %q, want [anthropic/claude-opus-5]", got)
	}
	if got := cmd.Complete("", ctx); len(got) != 3 {
		t.Errorf("Complete() = %q, want all three models", got)
	}
}

// TestPreferredModelsExampleAfterSwitch loads the example after a switch,
// as after a reload of the extensions. ctx.Model is then provider/model, and
// GetModelCapabilities("") still describes the configured model.
func TestPreferredModelsExampleAfterSwitch(t *testing.T) {
	harness, cmd := loadPreferred(t)
	ctx := preferredContext(harness, "ollama/qwen3")
	var shown []string
	ctx.PromptSelect = func(cfg extensions.PromptSelectConfig) extensions.PromptSelectResult {
		shown = cfg.Options
		return extensions.PromptSelectResult{Cancelled: true}
	}
	if _, err := cmd.Execute("", ctx); err != nil {
		t.Fatal(err)
	}
	if len(shown) != 3 || shown[2] != "3  ollama/qwen3  (current)" || strings.Contains(shown[0], "(current)") {
		t.Errorf("options = %q, want the mark on ollama/qwen3 only", shown)
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
