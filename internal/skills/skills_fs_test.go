package skills

import (
	"testing"
	"testing/fstest"
)

func TestLoadSkillsFromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"top.md":        {Data: []byte("---\nname: top-skill\ndescription: a top level skill\n---\nbody here")},
		"notes.txt":     {Data: []byte("plain text skill")},
		"deep/SKILL.md": {Data: []byte("---\nname: deep-skill\n---\ndeep body")},
		"deep/other.md": {Data: []byte("ignored non-SKILL nested md")},
		"ignore.json":   {Data: []byte("{}")},
	}

	got, err := LoadSkillsFromFS(fsys, ".")
	if err != nil {
		t.Fatalf("LoadSkillsFromFS error: %v", err)
	}

	byName := map[string]*Skill{}
	for _, s := range got {
		byName[s.Name] = s
	}

	if _, ok := byName["top-skill"]; !ok {
		t.Errorf("top-skill not loaded; got %v", names(got))
	}
	if _, ok := byName["notes"]; !ok {
		t.Errorf("notes (txt) not loaded; got %v", names(got))
	}
	if _, ok := byName["deep-skill"]; !ok {
		t.Errorf("deep SKILL.md not loaded; got %v", names(got))
	}
	if _, ok := byName["other"]; ok {
		t.Errorf("nested non-SKILL .md should be ignored; got %v", names(got))
	}
	if len(got) != 3 {
		t.Errorf("expected 3 skills, got %d: %v", len(got), names(got))
	}

	// Content/description parsed from frontmatter.
	if s := byName["top-skill"]; s != nil {
		if s.Description != "a top level skill" {
			t.Errorf("description = %q", s.Description)
		}
		if s.Content != "body here" {
			t.Errorf("content = %q", s.Content)
		}
		if s.Path != "top.md" {
			t.Errorf("path = %q, want top.md", s.Path)
		}
	}
}

func TestLoadSkillsFromFSNil(t *testing.T) {
	got, err := LoadSkillsFromFS(nil, ".")
	if err != nil || got != nil {
		t.Fatalf("nil fs should yield (nil, nil), got (%v, %v)", got, err)
	}
}

func names(skills []*Skill) []string {
	out := make([]string, 0, len(skills))
	for _, s := range skills {
		out = append(out, s.Name)
	}
	return out
}
