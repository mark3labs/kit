package ui

import (
	"testing"

	"github.com/mark3labs/kit/internal/ui/commands"
)

// TestAppendSkillCommands verifies skills are exposed as bare /<name>
// autocomplete entries tagged with the Skills category, and that a skill
// whose name collides with an existing slash command is skipped (dispatch
// would never reach it).
func TestAppendSkillCommands(t *testing.T) {
	base := []commands.SlashCommand{
		{Name: "/help", Aliases: []string{"/h"}, Category: "Info"},
		{Name: "/review", Category: "Prompts"},
	}
	items := []SkillItem{
		{Name: "pdf", Description: "PDF tools", Source: "user"},
		{Name: "review", Description: "shadowed by prompt template", Source: "user"},
		{Name: "h", Description: "shadowed by alias", Source: "user"},
		{Name: "deploy", Description: "Ship it", Source: "project"},
	}

	got := appendSkillCommands(base, items)

	var skillNames []string
	for _, sc := range got {
		if sc.Category == "Skills" {
			skillNames = append(skillNames, sc.Name)
			if !sc.HasArgs {
				t.Errorf("%s: skills accept trailing args, HasArgs must be true", sc.Name)
			}
		}
	}
	if len(skillNames) != 2 || skillNames[0] != "/pdf" || skillNames[1] != "/deploy" {
		t.Fatalf("skill entries = %v; want [/pdf /deploy]", skillNames)
	}

	// Project skills are tagged in the description; user skills are not.
	for _, sc := range got {
		switch sc.Name {
		case "/pdf":
			if sc.Description != "PDF tools" {
				t.Errorf("/pdf description = %q", sc.Description)
			}
		case "/deploy":
			if sc.Description != "(project) Ship it" {
				t.Errorf("/deploy description = %q", sc.Description)
			}
		}
	}
}

func TestCommandBadge(t *testing.T) {
	cases := map[string]string{
		"Skills":      "skill",
		"Prompts":     "prompt",
		"Extensions":  "ext",
		"MCP Prompts": "mcp",
		"Info":        "",
		"System":      "",
		"":            "",
	}
	for cat, want := range cases {
		if got := commandBadge(cat); got != want {
			t.Errorf("commandBadge(%q) = %q; want %q", cat, got, want)
		}
	}
}
