//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kit/ext"
)

const (
	diagnosticsTimeout = 20 * time.Second
	maxOutputBytes     = 12_000
)

type toolPathInput struct {
	Path string `json:"path"`
}

type lintResult struct {
	Output string
	Err    error
}

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("go-edit-lint extension loaded - will run gopls and golangci-lint on Go file edits")
	})

	api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
		if e.IsError || !isEditOrWrite(e.ToolName) {
			return nil
		}

		absPath, ok := resolveGoFilePath(e.Input, ctx.CWD)
		if !ok {
			return nil
		}

		report := runGoDiagnostics(ctx.CWD, absPath)
		
		// Check if there are issues and add explicit prompt for the LLM to react
		goplsIssues, lintIssues := countIssues(report)
		hasIssues := goplsIssues > 0 || lintIssues > 0
		
		var enhanced string
		if hasIssues {
			enhanced = e.Content + "\n\n" + report + "\n\n⚠️ DIAGNOSTICS FOUND: Please review the issues above and fix them before proceeding."
		} else {
			enhanced = e.Content + "\n\n" + report
		}

		// Show TUI message block for diagnostics visibility (only if there are issues)
		if hasIssues {
			var msgLines []string
			msgLines = append(msgLines, fmt.Sprintf("File: %s", filepath.Base(absPath)))
			if goplsIssues > 0 {
				msgLines = append(msgLines, fmt.Sprintf("gopls: %d issue(s)", goplsIssues))
			}
			if lintIssues > 0 {
				msgLines = append(msgLines, fmt.Sprintf("golangci-lint: %d issue(s)", lintIssues))
			}
			msgLines = append(msgLines, "", "⚠️ Please fix these issues before proceeding.")

			borderColor := "#f9e2af" // yellow
			if goplsIssues > 0 && lintIssues > 0 {
				borderColor = "#f38ba8" // red
			}

			ctx.PrintBlock(ext.PrintBlockOpts{
				Text:        strings.Join(msgLines, "\n"),
				BorderColor: borderColor,
				Subtitle:    "go-edit-lint",
			})
		}

		return &ext.ToolResultResult{Content: &enhanced}
	})
}

func isEditOrWrite(toolName string) bool {
	return strings.EqualFold(toolName, "edit") || strings.EqualFold(toolName, "write")
}

func resolveGoFilePath(inputJSON, cwd string) (string, bool) {
	var args toolPathInput
	if err := json.Unmarshal([]byte(inputJSON), &args); err != nil || args.Path == "" {
		return "", false
	}

	absPath := args.Path
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, absPath)
	}

	if strings.ToLower(filepath.Ext(absPath)) != ".go" {
		return "", false
	}

	return absPath, true
}

func runGoDiagnostics(cwd, absPath string) string {
	target := absPath
	if rel, err := filepath.Rel(cwd, absPath); err == nil && !strings.HasPrefix(rel, "..") {
		target = rel
	}

	gopls := runGopls(cwd, absPath)
	lint := runGolangCILint(cwd, target)

	return fmt.Sprintf(
		"<go_diagnostics file=%q>\n[gopls]\n%s\n\n[golangci-lint]\n%s\n</go_diagnostics>",
		filepath.Base(absPath),
		formatToolResult(gopls, "No diagnostics."),
		formatToolResult(lint, "No lint issues."),
	)
}

func runGopls(cwd, absPath string) lintResult {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gopls", "check", absPath)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return lintResult{Err: fmt.Errorf("timed out after %s", diagnosticsTimeout)}
	}

	if err != nil {
		return lintResult{Output: truncate(string(out), maxOutputBytes), Err: fmt.Errorf("failed to run gopls check: %w", err)}
	}

	return lintResult{Output: truncate(string(out), maxOutputBytes)}
}

func runGolangCILint(cwd, target string) lintResult {
	ctx, cancel := context.WithTimeout(context.Background(), diagnosticsTimeout)
	defer cancel()

	args := []string{
		"run",
		target,
		"--show-stats=false",
		"--output.text.path", "stdout",
		"--output.text.colors=false",
		"--output.text.print-issued-lines=false",
	}
	cmd := exec.CommandContext(ctx, "golangci-lint", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return lintResult{Err: fmt.Errorf("timed out after %s", diagnosticsTimeout)}
	}

	trimmed := truncate(string(out), maxOutputBytes)
	if err == nil {
		return lintResult{Output: trimmed}
	}

	exitErr, ok := err.(*exec.ExitError)
	if ok && exitErr.ExitCode() == 1 {
		return lintResult{Output: trimmed}
	}

	return lintResult{Output: trimmed, Err: fmt.Errorf("failed to run golangci-lint: %w", err)}
}

func formatToolResult(res lintResult, emptyFallback string) string {
	var lines []string
	if res.Err != nil {
		lines = append(lines, "ERROR: "+res.Err.Error())
	}
	out := strings.TrimSpace(res.Output)
	if out == "" {
		if res.Err == nil {
			lines = append(lines, emptyFallback)
		}
	} else {
		lines = append(lines, out)
	}
	if len(lines) == 0 {
		return emptyFallback
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... output truncated ..."
}

func countIssues(report string) (goplsCount, lintCount int) {
	// Extract gopls section
	goplsStart := strings.Index(report, "[gopls]")
	lintStart := strings.Index(report, "[golangci-lint]")
	endTag := strings.Index(report, "</go_diagnostics>")

	if goplsStart != -1 && lintStart != -1 {
		goplsSection := report[goplsStart:lintStart]
		// Count non-empty lines excluding the header and "No diagnostics." message
		for _, line := range strings.Split(goplsSection, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "[gopls]" && line != "No diagnostics." {
				goplsCount++
			}
		}
	}

	if lintStart != -1 && endTag != -1 {
		lintSection := report[lintStart:endTag]
		// Count non-empty lines excluding the header and "No lint issues." message
		for _, line := range strings.Split(lintSection, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "[golangci-lint]" && line != "No lint issues." {
				lintCount++
			}
		}
	}

	return goplsCount, lintCount
}
