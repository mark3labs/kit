package skills

import (
	"strings"
	"testing"
)

func TestPromptBuilder_BaseOnly(t *testing.T) {
	result := NewPromptBuilder("You are a helpful assistant.").Build()
	if result != "You are a helpful assistant." {
		t.Errorf("Build = %q, want base prompt only", result)
	}
}

func TestPromptBuilder_EmptyBase(t *testing.T) {
	result := NewPromptBuilder("").Build()
	if result != "" {
		t.Errorf("Build = %q, want empty string", result)
	}
}

func TestPromptBuilder_WithSection(t *testing.T) {
	result := NewPromptBuilder("Base prompt.").
		WithSection("Context", "Some context here.").
		Build()
	if !strings.Contains(result, "Base prompt.") {
		t.Error("missing base prompt")
	}
	if !strings.Contains(result, "# Context") {
		t.Error("missing section header")
	}
	if !strings.Contains(result, "Some context here.") {
		t.Error("missing section content")
	}
}

func TestPromptBuilder_WithSkills(t *testing.T) {
	skills := []*Skill{
		{Name: "coding", Description: "Write code", Content: "Use TDD."},
	}
	result := NewPromptBuilder("You are an agent.").
		WithSkills(skills).
		Build()
	if !strings.Contains(result, "You are an agent.") {
		t.Error("missing base prompt")
	}
	if !strings.Contains(result, "## coding") {
		t.Error("missing skill header")
	}
	if !strings.Contains(result, "Use TDD.") {
		t.Error("missing skill content")
	}
}

func TestPromptBuilder_WithSkills_Empty(t *testing.T) {
	result := NewPromptBuilder("Base.").
		WithSkills(nil).
		Build()
	// No skills section should be added.
	if strings.Contains(result, "Skills") {
		t.Error("should not add skills section when skills are empty")
	}
	if result != "Base." {
		t.Errorf("Build = %q, want just base", result)
	}
}

func TestPromptBuilder_MultipleSections(t *testing.T) {
	result := NewPromptBuilder("Base.").
		WithSection("Rules", "Follow rules.").
		WithSection("Examples", "Example 1.").
		Build()
	if !strings.Contains(result, "# Rules") {
		t.Error("missing Rules section")
	}
	if !strings.Contains(result, "# Examples") {
		t.Error("missing Examples section")
	}
}

func TestPromptBuilder_EmptySectionSkipped(t *testing.T) {
	result := NewPromptBuilder("Base.").
		WithSection("Empty", "").
		WithSection("Present", "Content.").
		Build()
	if strings.Contains(result, "# Empty") {
		t.Error("empty section should be skipped")
	}
	if !strings.Contains(result, "# Present") {
		t.Error("non-empty section should be present")
	}
}

func TestPromptBuilder_Chaining(t *testing.T) {
	// Verify that methods return the builder for chaining.
	pb := NewPromptBuilder("Base.")
	result := pb.WithSection("A", "a").WithSection("B", "b").Build()
	if !strings.Contains(result, "# A") || !strings.Contains(result, "# B") {
		t.Error("chaining should work")
	}
}

func TestPromptBuilder_EmptyBaseWithSection(t *testing.T) {
	result := NewPromptBuilder("").
		WithSection("Only", "Just a section.").
		Build()
	if !strings.Contains(result, "# Only") {
		t.Error("section should be present even with empty base")
	}
	if !strings.Contains(result, "Just a section.") {
		t.Error("section content should be present")
	}
}
