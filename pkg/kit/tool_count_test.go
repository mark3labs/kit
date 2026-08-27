package kit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/kit/pkg/kit"
)

// writeSkill creates a minimal skill directory and returns its path.
func writeSkill(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	body := "---\nname: " + name + "\ndescription: A skill used to test tool counting.\n---\n\nBody.\n"
	path := filepath.Join(dir, name+".md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// newEchoTool returns a trivial tool used to populate the extra-tool bucket.
func newEchoTool(name string) kit.Tool {
	type input struct {
		Text string `json:"text"`
	}
	return kit.NewTool(name, "Echo the input back",
		func(ctx context.Context, in input) (kit.ToolOutput, error) {
			return kit.TextResult(in.Text), nil
		},
	)
}

// TestExtensionToolCount_ExcludesSkillTool is a regression test for the
// startup banner reporting "extensions 1 tools" when no extension is loaded.
//
// Loading any skill registers the built-in activate_skill tool into the
// agent's extra-tool bucket. GetExtensionToolCount used to return the size of
// that whole bucket, so a skill was reported as an extension.
func TestExtensionToolCount_ExcludesSkillTool(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")

	host, err := kit.New(context.Background(), &kit.Options{
		Model:            "openai/gpt-4o-mini",
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
		NoContextFiles:   true,
		Skills:           []string{writeSkill(t, "counted-skill")},
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = host.Close() }()

	// Guard the premise: the skill really did load, so a zero count below
	// cannot pass for the wrong reason.
	if got := len(host.GetSkills()); got != 1 {
		t.Fatalf("premise failed: want 1 skill loaded, got %d", got)
	}
	if got := host.GetExtensionToolCount(); got != 0 {
		t.Errorf("no extensions loaded: want extension tool count 0, got %d", got)
	}
}

// TestExtensionToolCount_ExcludesSDKExtraTools covers the same bucket-conflation
// bug from the SDK side: tools supplied via Options.ExtraTools or AddTools are
// caller-provided, not extension-provided, and must not inflate the count.
func TestExtensionToolCount_ExcludesSDKExtraTools(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")

	host, err := kit.New(context.Background(), &kit.Options{
		Model:            "openai/gpt-4o-mini",
		Quiet:            true,
		NoSession:        true,
		NoExtensions:     true,
		DisableCoreTools: true,
		SkipConfig:       true,
		NoSkills:         true,
		NoContextFiles:   true,
	})
	if err != nil {
		t.Fatalf("kit.New: %v", err)
	}
	defer func() { _ = host.Close() }()

	if got := host.GetExtensionToolCount(); got != 0 {
		t.Fatalf("baseline: want 0, got %d", got)
	}

	host.AddTools(newEchoTool("sdk_tool_a"), newEchoTool("sdk_tool_b"))

	// Premise guard: the tools really were added to the agent.
	if got := len(host.GetExtraTools()); got != 2 {
		t.Fatalf("premise failed: want 2 extra tools, got %d", got)
	}
	if got := host.GetExtensionToolCount(); got != 0 {
		t.Errorf("SDK-supplied tools counted as extensions: want 0, got %d", got)
	}
}
