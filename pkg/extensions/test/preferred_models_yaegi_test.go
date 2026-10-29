package test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/kit/pkg/extensions"
)

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

// TestPreferredModelsExample runs /preferred as Kit does. At startup,
// Context.Model is only the model ID, and the configured model is in use.
// After a model switch, Context.Model is provider/model, but
// GetModelCapabilities("") still describes the configured model.
func TestPreferredModelsExample(t *testing.T) {
	harness, cmd := loadPreferred(t)

	var switched []string // the models given to SetModel
	var shown []string    // the options of the last picker
	pick := -1            // the option that the picker selects, -1 for Esc
	ctx := harness.Context().ToContext()
	ctx.Model = "claude-opus-5-5"
	ctx.GetOption = func(name string) string {
		if name == "preferred-models" {
			return "anthropic/claude-opus-5-5, amazon-bedrock/eu.anthropic.claude-opus-5-5 ollama/qwen3"
		}
		return ""
	}
	ctx.GetModelCapabilities = func(model string) (extensions.ModelCapabilities, string) {
		if model != "" {
			return extensions.ModelCapabilities{}, "model not found in registry"
		}
		return extensions.ModelCapabilities{Provider: "anthropic", ModelID: "claude-opus-5-5"}, ""
	}
	ctx.SetModel = func(model string) error {
		switched = append(switched, model)
		ctx.Model = model
		return nil
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

	// The picker marks the configured model. Esc changes nothing.
	if out, err := run(""); out != "" || err != nil {
		t.Fatalf("/preferred with Esc = %q, %v; want no output", out, err)
	}
	want := []string{
		"1  anthropic/claude-opus-5-5  (current)",
		"2  amazon-bedrock/eu.anthropic.claude-opus-5-5",
		"3  ollama/qwen3",
	}
	if !reflect.DeepEqual(shown, want) {
		t.Fatalf("options at startup = %q, want %q", shown, want)
	}

	pick = 2
	if out, err := run(""); out != "Switched to ollama/qwen3" || err != nil {
		t.Fatalf("/preferred, third option = %q, %v; want the switch", out, err)
	}

	// After the switch, the mark follows Context.Model.
	pick = -1
	if _, err := run(""); err != nil {
		t.Fatal(err)
	}
	want[0] = "1  anthropic/claude-opus-5-5"
	want[2] = "3  ollama/qwen3  (current)"
	if !reflect.DeepEqual(shown, want) {
		t.Fatalf("options after the switch = %q, want %q", shown, want)
	}

	// A number or a part of a name selects a model. The match ignores case.
	for _, c := range []struct{ args, want string }{
		{"1", "Switched to anthropic/claude-opus-5-5"},
		{"BEDROCK", "Switched to amazon-bedrock/eu.anthropic.claude-opus-5-5"},
		{"bedrock", "Already using amazon-bedrock/eu.anthropic.claude-opus-5-5"},
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
	wantSwitched := []string{"ollama/qwen3", "anthropic/claude-opus-5-5", "amazon-bedrock/eu.anthropic.claude-opus-5-5"}
	if !reflect.DeepEqual(switched, wantSwitched) {
		t.Errorf("SetModel calls = %q, want %q", switched, wantSwitched)
	}

	if got := cmd.Complete("oll", ctx); !reflect.DeepEqual(got, []string{"ollama/qwen3"}) {
		t.Errorf("Complete(oll) = %q, want [ollama/qwen3]", got)
	}
	if got := cmd.Complete("", ctx); len(got) != 3 {
		t.Errorf("Complete() = %q, want all three models", got)
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
