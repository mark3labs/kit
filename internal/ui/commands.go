package ui

import (
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/models"
)

// SlashCommand represents a user-invokable slash command with its metadata.
// Commands can have multiple aliases and are organized by category for better
// discoverability and help display.
type SlashCommand struct {
	Name        string
	Description string
	Aliases     []string
	Category    string                       // e.g., "Navigation", "System", "Info"
	Complete    func(prefix string) []string // optional argument tab-completion
}

// SlashCommands provides the global registry of all available slash commands
// in the application. Commands are organized by category (Info, System) and
// include their primary names, descriptions, and alternative aliases.
var SlashCommands = []SlashCommand{
	{
		Name:        "/help",
		Description: "Show available commands and usage information",
		Category:    "Info",
		Aliases:     []string{"/h", "/?"},
	},
	{
		Name:        "/tools",
		Description: "List all available MCP tools",
		Category:    "Info",
		Aliases:     []string{"/t"},
	},
	{
		Name:        "/servers",
		Description: "Show connected MCP servers",
		Category:    "Info",
		Aliases:     []string{"/s"},
	},

	{
		Name:        "/clear",
		Description: "Clear conversation and start fresh",
		Category:    "System",
		Aliases:     []string{"/c", "/cls"},
	},
	{
		Name:        "/usage",
		Description: "Show token usage statistics",
		Category:    "Info",
		Aliases:     []string{"/u"},
	},
	{
		Name:        "/reset-usage",
		Description: "Reset usage statistics",
		Category:    "System",
		Aliases:     []string{"/ru"},
	},
	{
		Name:        "/clear-queue",
		Description: "Clear all queued messages",
		Category:    "System",
		Aliases:     []string{"/cq"},
	},
	{
		Name:        "/compact",
		Description: "Summarise older messages to free context space",
		Category:    "System",
		Aliases:     []string{"/co"},
	},
	{
		Name:        "/model",
		Description: "Switch to a different model",
		Category:    "System",
		Aliases:     []string{"/m"},
	},
	{
		Name:        "/thinking",
		Description: "Set thinking/reasoning level (off, minimal, low, medium, high)",
		Category:    "System",
		Aliases:     []string{"/think"},
		Complete: func(prefix string) []string {
			levels := models.ThinkingLevels()
			var matches []string
			for _, l := range levels {
				s := string(l)
				if prefix == "" || strings.HasPrefix(s, strings.ToLower(prefix)) {
					matches = append(matches, s)
				}
			}
			return matches
		},
	},
	{
		Name:        "/theme",
		Description: "Switch color theme (e.g. /theme catppuccin)",
		Category:    "System",
		Complete: func(prefix string) []string {
			names := ListThemes()
			if prefix == "" {
				return names
			}
			var matches []string
			for _, n := range names {
				if strings.HasPrefix(n, strings.ToLower(prefix)) {
					matches = append(matches, n)
				}
			}
			return matches
		},
	},
	{
		Name:        "/quit",
		Description: "Exit the application",
		Category:    "System",
		Aliases:     []string{"/q", "/exit"},
	},

	// Navigation commands (tree sessions)
	{
		Name:        "/tree",
		Description: "Navigate session tree (switch branches)",
		Category:    "Navigation",
	},
	{
		Name:        "/fork",
		Description: "Branch from an earlier message",
		Category:    "Navigation",
	},
	{
		Name:        "/new",
		Description: "Start a new session",
		Category:    "Navigation",
		Aliases:     []string{"/n"},
	},
	{
		Name:        "/name",
		Description: "Set a display name for this session",
		Category:    "Navigation",
	},
	{
		Name:        "/resume",
		Description: "Open session picker to switch sessions",
		Category:    "Navigation",
		Aliases:     []string{"/r"},
	},
	{
		Name:        "/export",
		Description: "Export session (JSONL by default, or /export path.jsonl)",
		Category:    "System",
	},
	{
		Name:        "/import",
		Description: "Import a session from a JSONL file (/import path.jsonl)",
		Category:    "System",
	},
	{
		Name:        "/session",
		Description: "Show session info and statistics",
		Category:    "Info",
	},
}

// GetCommandByName looks up a slash command by its primary name or any of its
// aliases. Returns a pointer to the matching SlashCommand, or nil if no command
// matches the provided name.
func GetCommandByName(name string) *SlashCommand {
	for i := range SlashCommands {
		cmd := &SlashCommands[i]
		if cmd.Name == name {
			return cmd
		}
		if slices.Contains(cmd.Aliases, name) {
			return cmd
		}
	}
	return nil
}

// GetAllCommandNames returns a complete list of all command names and their aliases.
// This is useful for command completion, validation, and help display. The returned
// slice contains both primary command names and all alternative aliases.
func GetAllCommandNames() []string {
	var names []string
	for _, cmd := range SlashCommands {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return names
}

// ExtensionCommand is a slash command registered by an extension. Unlike
// built-in SlashCommands whose execution is hardcoded in handleSlashCommand,
// extension commands carry their own Execute callback.
type ExtensionCommand struct {
	Name        string
	Description string
	Execute     func(args string) (string, error)
	Complete    func(prefix string) []string // optional argument tab-completion
}

// FindExtensionCommand looks up an extension command by name from the given
// slice. Returns a pointer to the matching command, or nil if not found.
func FindExtensionCommand(name string, cmds []ExtensionCommand) *ExtensionCommand {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}
