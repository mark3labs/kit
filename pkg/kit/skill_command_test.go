package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/skills"
)

func TestParseSkillCommand(t *testing.T) {
	cases := []struct {
		in         string
		name, args string
		ok         bool
	}{
		{"/pdf", "pdf", "", true},
		{"/pdf extract tables", "pdf", "extract tables", true},
		{"/pdf   spaced  ", "pdf", "spaced", true},
		{"/", "", "", false},
		{"pdf", "", "", false},
		{"hello /pdf", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		name, args, ok := ParseSkillCommand(c.in)
		if name != c.name || args != c.args || ok != c.ok {
			t.Errorf("ParseSkillCommand(%q) = (%q, %q, %v); want (%q, %q, %v)",
				c.in, name, args, ok, c.name, c.args, c.ok)
		}
	}
}

// TestExpandSkillCommand_SlashName verifies the agentskills.io user-explicit
// activation form "/<name> [args]": the skill body is re-read from disk,
// wrapped in <skill_content>, and trailing args are appended.
func TestExpandSkillCommand_SlashName(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "pdf")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: pdf\ndescription: d\n---\n# PDF body"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "REF.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	k := &Kit{}
	k.skills = []*skills.Skill{{Name: "pdf", Description: "d", Path: path, DisableModelInvocation: true}}

	got := k.expandSkillCommand("/pdf extract the tables")
	for _, want := range []string{
		`<skill_content name="pdf" location="` + path + `">`,
		"# PDF body",
		"<file>references/REF.md</file>",
		"</skill_content>\n\nextract the tables",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expanded prompt missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "name: pdf") {
		t.Error("frontmatter must be stripped from the activated body")
	}

	// No args: nothing appended after the closing tag.
	if got := k.expandSkillCommand("/pdf"); !strings.HasSuffix(got, "</skill_content>") {
		t.Errorf("expected no trailing args, got:\n%s", got)
	}

	// Unknown names and non-slash text pass through untouched.
	for _, in := range []string{"/nope", "/skill:pdf", "plain prompt", "/"} {
		if got := k.expandSkillCommand(in); got != in {
			t.Errorf("expandSkillCommand(%q) = %q; want unchanged", in, got)
		}
	}
}
