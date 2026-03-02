//go:build ignore

// interactive-shell.go — TUI Suspend example extension for Kit.
//
// Demonstrates ctx.SuspendTUI() which temporarily releases the terminal
// from the TUI so interactive subprocesses can run with full terminal
// control. The TUI is automatically restored when the callback returns.
//
// Commands:
//   /edit <file>   — opens $EDITOR (or vi) to edit a file
//   /shell         — drops into an interactive shell session
//   /run <cmd>     — runs a command with full terminal I/O (no TUI capture)

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	ext "kit/ext"
)

func Init(api ext.API) {
	api.RegisterCommand(ext.CommandDef{
		Name:        "edit",
		Description: "Open $EDITOR to edit a file (TUI suspends)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			file := strings.TrimSpace(args)
			if file == "" {
				return "", fmt.Errorf("usage: /edit <file>")
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			ctx.Print(fmt.Sprintf("Opening %s in %s...", file, editor))

			err := ctx.SuspendTUI(func() {
				cmd := exec.Command(editor, file)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			})
			if err != nil {
				return "", fmt.Errorf("editor session failed: %w", err)
			}

			return fmt.Sprintf("Finished editing %s", file), nil
		},
		Complete: func(prefix string, ctx ext.Context) []string {
			// Suggest files in the current directory.
			entries, err := os.ReadDir(".")
			if err != nil {
				return nil
			}
			var results []string
			for _, e := range entries {
				name := e.Name()
				if strings.HasPrefix(name, prefix) {
					results = append(results, name)
				}
			}
			return results
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "shell",
		Description: "Drop into an interactive shell (TUI suspends)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}

			ctx.Print(fmt.Sprintf("Starting %s... (type 'exit' to return to Kit)", shell))

			err := ctx.SuspendTUI(func() {
				cmd := exec.Command(shell)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			})
			if err != nil {
				return "", fmt.Errorf("shell session failed: %w", err)
			}

			return "Shell session ended, TUI restored.", nil
		},
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "run",
		Description: "Run a command with full terminal I/O (TUI suspends)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			cmdStr := strings.TrimSpace(args)
			if cmdStr == "" {
				return "", fmt.Errorf("usage: /run <command>")
			}

			ctx.Print(fmt.Sprintf("Running: %s", cmdStr))

			err := ctx.SuspendTUI(func() {
				cmd := exec.Command("sh", "-c", cmdStr)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
			})
			if err != nil {
				return "", fmt.Errorf("command failed: %w", err)
			}

			return "Command finished, TUI restored.", nil
		},
	})
}
