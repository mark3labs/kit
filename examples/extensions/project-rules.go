//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kit/ext"
)

// Init loads project-specific rules from .kit/rules/ into the system prompt.
// Each .md file in the rules directory is injected as additional context,
// giving projects a way to customise LLM behaviour without editing the
// main system prompt. Inspired by Pi's claude-rules.ts.
//
// Place rule files in:
//
//	.kit/rules/code-style.md
//	.kit/rules/testing.md
//	.kit/rules/security.md
//
// Usage: kit -e examples/extensions/project-rules.go
func Init(api ext.API) {
	var rules string

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		rulesDir := filepath.Join(ctx.CWD, ".kit", "rules")
		entries, err := os.ReadDir(rulesDir)
		if err != nil {
			return // no rules directory, nothing to do
		}

		var parts []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".txt") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(rulesDir, name))
			if err != nil {
				continue
			}
			content := strings.TrimSpace(string(data))
			if content != "" {
				parts = append(parts, "## "+strings.TrimSuffix(name, filepath.Ext(name))+"\n\n"+content)
			}
		}

		if len(parts) == 0 {
			return
		}

		rules = "# Project Rules\n\n" + strings.Join(parts, "\n\n---\n\n")
		ctx.PrintInfo(fmt.Sprintf("[project-rules] Loaded %d rule file(s) from .kit/rules/", len(parts)))
	})

	api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
		if rules == "" {
			return nil
		}
		return &ext.BeforeAgentStartResult{
			SystemPrompt: &rules,
		}
	})
}
