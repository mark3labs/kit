package skilltool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/skills"
)

func writeSkillFile(t *testing.T, dir, name string) *skills.Skill {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.py"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: " + name + "\ndescription: A test skill\n---\nDo the thing."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return &skills.Skill{Name: name, Description: "A test skill", Path: path}
}

func TestNew_NilWhenNoSkills(t *testing.T) {
	if tool := New(nil, func() []*skills.Skill { return nil }); tool != nil {
		t.Error("expected nil tool when no skill names provided")
	}
}

func TestActivateSkill_LoadsAndDedups(t *testing.T) {
	dir := t.TempDir()
	s := writeSkillFile(t, dir, "extract")
	provider := func() []*skills.Skill { return []*skills.Skill{s} }

	tool := New([]string{"extract"}, provider)
	if tool == nil {
		t.Fatal("expected a tool")
	}

	// First activation loads content + resources.
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"name":"extract"}`})
	if err != nil {
		t.Fatal(err)
	}
	out := responseText(resp)
	if !strings.Contains(out, "Do the thing.") {
		t.Errorf("expected skill body, got: %q", out)
	}
	if !strings.Contains(out, "<skill_content name=\"extract\"") {
		t.Errorf("expected skill_content wrapper, got: %q", out)
	}
	if !strings.Contains(out, "scripts/run.py") {
		t.Errorf("expected enumerated resources, got: %q", out)
	}

	// Second activation is deduplicated.
	resp2, err := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"name":"extract"}`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(responseText(resp2), "already loaded") {
		t.Errorf("expected dedup message, got: %q", responseText(resp2))
	}
}

func TestActivateSkill_UnknownSkill(t *testing.T) {
	provider := func() []*skills.Skill { return nil }
	tool := New([]string{"extract"}, provider)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"name":"nope"}`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(responseText(resp), "unknown skill") {
		t.Errorf("expected unknown-skill error, got: %q", responseText(resp))
	}
}

// responseText extracts the text from a tool response.
func responseText(resp fantasy.ToolResponse) string {
	return resp.Content
}
