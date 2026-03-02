//go:build ignore

package main

import (
	"os/exec"
	"strings"

	"kit/ext"
)

// Init registers before-hooks for destructive session operations:
//   - Forks: Asks for confirmation before branching to a different tree node.
//   - New sessions: Checks for uncommitted git changes and warns before
//     starting a new branch if the working tree is dirty.
//
// This demonstrates the OnBeforeFork and OnBeforeSessionSwitch events
// which allow extensions to cancel session lifecycle operations.
//
// Usage: kit -e examples/extensions/confirm-destructive.go --continue
func Init(api ext.API) {
	// Gate /new command: warn if there are uncommitted git changes.
	api.OnBeforeSessionSwitch(func(e ext.BeforeSessionSwitchEvent, ctx ext.Context) *ext.BeforeSessionSwitchResult {
		if !isGitDirty() {
			return nil // clean repo, allow switch
		}

		result := ctx.PromptConfirm(ext.PromptConfirmConfig{
			Message: "Working tree has uncommitted changes. Start new session anyway?",
		})
		if result.Cancelled || !result.Value {
			return &ext.BeforeSessionSwitchResult{
				Cancel: true,
				Reason: "Session switch cancelled: uncommitted git changes.",
			}
		}
		return nil // user approved
	})

	// Gate fork: ask for confirmation before branching.
	api.OnBeforeFork(func(e ext.BeforeForkEvent, ctx ext.Context) *ext.BeforeForkResult {
		msg := "Branch to this point in the conversation?"
		if e.IsUserMessage && e.UserText != "" {
			// Show a preview of the user message being forked to.
			preview := e.UserText
			if len(preview) > 80 {
				preview = preview[:77] + "..."
			}
			msg = "Fork and edit: " + preview + "\n\nContinue?"
		}

		result := ctx.PromptConfirm(ext.PromptConfirmConfig{
			Message: msg,
		})
		if result.Cancelled || !result.Value {
			return &ext.BeforeForkResult{
				Cancel: true,
				Reason: "Fork cancelled by user.",
			}
		}
		return nil // user approved
	})
}

// isGitDirty returns true if the git working tree has uncommitted changes.
func isGitDirty() bool {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return false // not a git repo or git not available
	}
	return len(strings.TrimSpace(string(out))) > 0
}
