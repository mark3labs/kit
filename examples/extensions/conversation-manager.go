//go:build ignore

// conversation-manager.go - Advanced conversation tree navigation and management.
// This extension demonstrates:
// - Tree navigation (GetTreeNode, GetCurrentBranch, NavigateTo)
// - Branch summarization and collapsing
// - Interactive tree exploration
//
// Commands:
//   /tree              - Show conversation tree structure
//   /branch            - Show current branch path
//   /goto <entry-id>   - Navigate to a specific entry
//   /summarize <n>     - Summarize last N messages
//   /fresh-context     - Collapse branch and start fresh
//   /loop <n> <prompt> - Execute prompt N times with fresh context each iteration

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"kit/ext"
)

var (
	loopActive    bool
	loopCount     int
	loopCurrent   int
	loopPrompt    string
	loopStartNode string
)

func Init(api ext.API) {
	// /tree - Show tree structure
	api.RegisterCommand(ext.CommandDef{
		Name:        "tree",
		Description: "Show conversation tree structure",
		Execute: func(args string, ctx ext.Context) (string, error) {
			showTree(ctx)
			return "", nil
		},
	})

	// /branch - Show current branch
	api.RegisterCommand(ext.CommandDef{
		Name:        "branch",
		Description: "Show current conversation branch",
		Execute: func(args string, ctx ext.Context) (string, error) {
			showBranch(ctx)
			return "", nil
		},
	})

	// /goto - Navigate to entry
	api.RegisterCommand(ext.CommandDef{
		Name:        "goto",
		Description: "Navigate to a specific entry ID (usage: /goto <entry-id>)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if args == "" {
				ctx.PrintError("Usage: /goto <entry-id>")
				return "", nil
			}
			result := ctx.NavigateTo(args)
			if !result.Success {
				ctx.PrintError(fmt.Sprintf("Navigation failed: %s", result.Error))
				return "", nil
			}
			ctx.PrintInfo(fmt.Sprintf("Navigated to entry: %s", args))

			// Show the node we navigated to
			node := ctx.GetTreeNode(args)
			if node != nil {
				ctx.PrintInfo(fmt.Sprintf("Entry type: %s, Role: %s", node.Type, node.Role))
			}
			return "", nil
		},
	})

	// /summarize - Summarize recent messages
	api.RegisterCommand(ext.CommandDef{
		Name:        "summarize",
		Description: "Summarize last N messages (usage: /summarize [n=5])",
		Execute: func(args string, ctx ext.Context) (string, error) {
			n := 5
			if args != "" {
				if parsed, err := strconv.Atoi(args); err == nil && parsed > 0 {
					n = parsed
				}
			}

			branch := ctx.GetCurrentBranch()
			if len(branch) < 2 {
				ctx.PrintError("Not enough messages to summarize")
				return "", nil
			}

			// Find range to summarize
			startIdx := len(branch) - n - 1
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx := len(branch) - 1

			fromID := branch[startIdx].ID
			toID := branch[endIdx].ID

			ctx.PrintInfo(fmt.Sprintf("Summarizing messages %d to %d...", startIdx, endIdx))
			summary := ctx.SummarizeBranch(fromID, toID)

			if summary == "" {
				ctx.PrintError("Failed to generate summary")
				return "", nil
			}

			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        summary,
				BorderColor: "#89b4fa",
				Subtitle:    "conversation-manager · Summary",
			})
			return "", nil
		},
	})

	// /fresh-context - Collapse and restart
	api.RegisterCommand(ext.CommandDef{
		Name:        "fresh-context",
		Description: "Collapse conversation to summary and start fresh",
		Execute: func(args string, ctx ext.Context) (string, error) {
			branch := ctx.GetCurrentBranch()
			if len(branch) < 3 {
				ctx.PrintError("Not enough context to collapse")
				return "", nil
			}

			// Keep first message (system), summarize rest
			fromID := branch[1].ID
			toID := branch[len(branch)-1].ID

			ctx.PrintInfo("Generating summary for context collapse...")
			summary := ctx.SummarizeBranch(fromID, toID)

			if summary == "" {
				ctx.PrintError("Failed to generate summary")
				return "", nil
			}

			// Collapse the branch
			result := ctx.CollapseBranch(fromID, toID, summary)
			if !result.Success {
				ctx.PrintError(fmt.Sprintf("Collapse failed: %s", result.Error))
				return "", nil
			}

			ctx.PrintInfo("Context collapsed. Starting fresh with summary.")
			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        summary,
				BorderColor: "#a6e3a1",
				Subtitle:    "conversation-manager · Collapsed Context",
			})

			// Set a widget showing we're in fresh mode
			ctx.SetWidget(ext.WidgetConfig{
				ID:        "fresh-context",
				Placement: ext.WidgetAbove,
				Content:   ext.WidgetContent{Text: "🌱 Fresh Context Mode - Previous conversation collapsed"},
				Style:     ext.WidgetStyle{BorderColor: "#a6e3a1"},
			})

			return "", nil
		},
	})

	// /loop - Execute with fresh context each iteration
	api.RegisterCommand(ext.CommandDef{
		Name:        "loop",
		Description: "Execute prompt N times with fresh context (usage: /loop 5 analyze this code)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if loopActive {
				ctx.PrintError("Loop already in progress. Wait for completion.")
				return "", nil
			}

			// Parse arguments
			parts := strings.SplitN(args, " ", 2)
			if len(parts) < 2 {
				ctx.PrintError("Usage: /loop <count> <prompt>")
				return "", nil
			}

			count, err := strconv.Atoi(parts[0])
			if err != nil || count <= 0 || count > 10 {
				ctx.PrintError("Invalid count (must be 1-10)")
				return "", nil
			}

			loopCount = count
			loopCurrent = 0
			loopPrompt = parts[1]
			loopActive = true

			// Store current branch position
			branch := ctx.GetCurrentBranch()
			if len(branch) > 0 {
				loopStartNode = branch[len(branch)-1].ID
			}

			ctx.PrintInfo(fmt.Sprintf("Starting loop: %d iterations", loopCount))
			ctx.SetWidget(ext.WidgetConfig{
				ID:        "loop-progress",
				Placement: ext.WidgetAbove,
				Content:   ext.WidgetContent{Text: fmt.Sprintf("🔄 Loop: 0/%d - %s", loopCount, loopPrompt)},
				Style:     ext.WidgetStyle{BorderColor: "#fab387"},
			})

			// Start first iteration
			executeLoopIteration(ctx)
			return "", nil
		},
	})

	// OnAgentEnd handles loop continuation
	api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
		if !loopActive {
			return
		}

		loopCurrent++

		if loopCurrent >= loopCount {
			// Loop complete
			loopActive = false
			ctx.RemoveWidget("loop-progress")
			ctx.PrintInfo(fmt.Sprintf("✅ Loop complete: %d/%d iterations", loopCurrent, loopCount))

			// Show final summary
			branch := ctx.GetCurrentBranch()
			if len(branch) > 0 && loopStartNode != "" {
				summary := ctx.SummarizeBranch(loopStartNode, branch[len(branch)-1].ID)
				if summary != "" {
					ctx.PrintBlock(ext.PrintBlockOpts{
						Text:        summary,
						BorderColor: "#a6e3a1",
						Subtitle:    "conversation-manager · Loop Summary",
					})
				}
			}
			return
		}

		// Update progress
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "loop-progress",
			Placement: ext.WidgetAbove,
			Content:   ext.WidgetContent{Text: fmt.Sprintf("🔄 Loop: %d/%d - %s", loopCurrent, loopCount, loopPrompt)},
			Style:     ext.WidgetStyle{BorderColor: "#fab387"},
		})

		// Collapse previous iteration for fresh context
		branch := ctx.GetCurrentBranch()
		if len(branch) >= 2 {
			// Find the user messages (look for the one before the last assistant message)
			// We want to collapse from the user message that started this iteration
			// to the last assistant response
			var collapseStartIdx = -1
			for i := len(branch) - 1; i >= 0; i-- {
				if branch[i].Role == "assistant" {
					// Found the last assistant message, now find the user message before it
					for j := i - 1; j >= 0; j-- {
						if branch[j].Role == "user" {
							collapseStartIdx = j
							break
						}
					}
					break
				}
			}

			if collapseStartIdx >= 0 {
				fromID := branch[collapseStartIdx].ID
				toID := branch[len(branch)-1].ID

				ctx.PrintInfo(fmt.Sprintf("Collapsing iteration %d for fresh context...", loopCurrent))
				summary := ctx.SummarizeBranch(fromID, toID)
				if summary != "" {
					result := ctx.CollapseBranch(fromID, toID, summary)
					if result.Success {
						ctx.PrintInfo("Context collapsed successfully")
					} else {
						ctx.PrintError(fmt.Sprintf("Collapse failed: %s", result.Error))
					}
				}
			}
		}

		// Small delay to let UI update
		time.Sleep(500 * time.Millisecond)

		// Trigger next iteration
		executeLoopIteration(ctx)
	})
}

// showTree displays the conversation tree structure
func showTree(ctx ext.Context) {
	branch := ctx.GetCurrentBranch()
	if len(branch) == 0 {
		ctx.PrintInfo("Tree is empty")
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Conversation Tree (%d nodes):\n\n", len(branch)))

	for i, node := range branch {
		prefix := "  "
		if i == len(branch)-1 {
			prefix = "▶ " // Current node
		} else {
			prefix = "  "
		}

		roleIcon := "💬"
		switch node.Role {
		case "user":
			roleIcon = "👤"
		case "assistant":
			roleIcon = "🤖"
		case "system":
			roleIcon = "⚙️"
		}

		content := truncate(node.Content, 50)
		if node.Type == "branch_summary" {
			roleIcon = "📋"
			content = "[Summary] " + truncate(node.Content, 40)
		}

		output.WriteString(fmt.Sprintf("%s%s %s: %s (%s...)\n", prefix, roleIcon, node.Role, node.ID[:8], content))

		// Show children count if any
		children := ctx.GetChildren(node.ID)
		if len(children) > 0 {
			output.WriteString(fmt.Sprintf("    └─ %d branch(es)\n", len(children)))
		}
	}

	ctx.PrintBlock(ext.PrintBlockOpts{
		Text:        output.String(),
		BorderColor: "#89b4fa",
		Subtitle:    "conversation-manager · Tree View",
	})
}

// showBranch displays the current branch path
func showBranch(ctx ext.Context) {
	branch := ctx.GetCurrentBranch()
	if len(branch) == 0 {
		ctx.PrintInfo("No active branch")
		return
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Current Branch (%d nodes from root to leaf):\n\n", len(branch)))

	for i, node := range branch {
		marker := "  "
		if i == len(branch)-1 {
			marker = "▶ " // Current leaf
		}

		output.WriteString(fmt.Sprintf("%s[%d] %s (%s): %s\n",
			marker, i, node.Type, node.ID[:8], truncate(node.Content, 40)))
	}

	// Show current node details
	leaf := branch[len(branch)-1]
	output.WriteString(fmt.Sprintf("\nCurrent Leaf:\n"))
	output.WriteString(fmt.Sprintf("  ID: %s\n", leaf.ID))
	output.WriteString(fmt.Sprintf("  Type: %s\n", leaf.Type))
	output.WriteString(fmt.Sprintf("  Role: %s\n", leaf.Role))
	output.WriteString(fmt.Sprintf("  Model: %s\n", leaf.Model))
	output.WriteString(fmt.Sprintf("  Children: %d\n", len(leaf.Children)))

	ctx.PrintBlock(ext.PrintBlockOpts{
		Text:        output.String(),
		BorderColor: "#cba6f7",
		Subtitle:    "conversation-manager · Branch View",
	})
}

// executeLoopIteration triggers the next loop iteration
func executeLoopIteration(ctx ext.Context) {
	iterationPrompt := fmt.Sprintf("[%d/%d] %s", loopCurrent+1, loopCount, loopPrompt)
	ctx.SendMessage(iterationPrompt)
}

// truncate helper
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
