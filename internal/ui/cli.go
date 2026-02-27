package ui

import (
	"fmt"
	"os"
	"time"

	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// CLI manages the command-line interface for KIT, providing message rendering,
// user input handling, and display management. It supports both standard and compact
// display modes, handles streaming responses, tracks token usage, and manages the
// overall conversation flow between the user and AI assistants.
type CLI struct {
	renderer     Renderer
	usageTracker *UsageTracker
	width        int
	compactMode  bool
	debug        bool
	modelName    string
}

// NewCLI creates and initializes a new CLI instance with the specified display modes.
// The debug parameter enables debug message rendering, while compact enables a more
// condensed display format. Returns an initialized CLI ready for interaction or an
// error if initialization fails.
func NewCLI(debug bool, compact bool) (*CLI, error) {
	cli := &CLI{
		compactMode: compact,
		debug:       debug,
	}
	cli.updateSize()
	if compact {
		cli.renderer = NewCompactRenderer(cli.width, debug)
	} else {
		cli.renderer = NewMessageRenderer(cli.width, debug)
	}

	return cli, nil
}

// SetUsageTracker attaches a usage tracker to the CLI for monitoring token
// consumption and costs. The tracker will be automatically updated with the
// current display width for proper rendering.
func (c *CLI) SetUsageTracker(tracker *UsageTracker) {
	c.usageTracker = tracker
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}

// GetUsageTracker returns the usage tracker attached to this CLI, or nil if no
// tracker has been configured. Callers that need a usage-tracker-agnostic handle
// can assign the returned *UsageTracker wherever an app.UsageUpdater is expected —
// *UsageTracker satisfies that interface.
func (c *CLI) GetUsageTracker() *UsageTracker {
	return c.usageTracker
}

// GetDebugLogger returns a CLIDebugLogger instance that routes debug output
// through the CLI's rendering system for consistent message formatting and display.
func (c *CLI) GetDebugLogger() *CLIDebugLogger {
	return NewCLIDebugLogger(c)
}

// SetModelName updates the current AI model name being used in the conversation.
// This name is displayed in message headers to indicate which model is responding.
func (c *CLI) SetModelName(modelName string) {
	c.modelName = modelName
}

// ShowSpinner displays an animated spinner while executing the provided action
// function. The spinner automatically stops when the action completes. Returns
// any error returned by the action function.
func (c *CLI) ShowSpinner(action func() error) error {
	spinner := NewSpinner()
	spinner.Start()

	err := action()

	spinner.Stop()

	return err
}

// DisplayUserMessage renders and displays a user's message with appropriate
// formatting based on the current display mode (standard or compact). The message
// is timestamped and styled according to the active theme.
func (c *CLI) DisplayUserMessage(message string) {
	fmt.Println(c.renderer.RenderUserMessage(message, time.Now()).Content)
}

// DisplayAssistantMessage renders and displays an AI assistant's response message
// with appropriate formatting. This method delegates to DisplayAssistantMessageWithModel
// with an empty model name for backward compatibility.
func (c *CLI) DisplayAssistantMessage(message string) error {
	return c.DisplayAssistantMessageWithModel(message, "")
}

// DisplayAssistantMessageWithModel renders and displays an AI assistant's response
// with the specified model name shown in the message header. The message is
// formatted according to the current display mode and includes timestamp information.
func (c *CLI) DisplayAssistantMessageWithModel(message, modelName string) error {
	fmt.Println(c.renderer.RenderAssistantMessage(message, time.Now(), modelName).Content)
	return nil
}

// DisplayToolCallMessage is a no-op retained for backward compatibility. Tool
// calls are now rendered as part of the unified tool block in DisplayToolMessage,
// which combines the invocation header with the execution result.
func (c *CLI) DisplayToolCallMessage(toolName, toolArgs string) {
	// No-op: unified tool blocks are rendered in DisplayToolMessage.
}

// DisplayToolMessage renders and displays the complete result of a tool execution,
// including the tool name, arguments, and result. The isError parameter determines
// whether the result should be displayed as an error or success message.
func (c *CLI) DisplayToolMessage(toolName, toolArgs, toolResult string, isError bool) {
	fmt.Println(c.renderer.RenderToolMessage(toolName, toolArgs, toolResult, isError).Content)
}

// DisplayError renders and displays an error message with distinctive formatting
// to ensure visibility. The error is timestamped and styled according to the
// current display mode's error theme.
func (c *CLI) DisplayError(err error) {
	fmt.Println(c.renderer.RenderErrorMessage(err.Error(), time.Now()).Content)
}

// DisplayInfo renders and displays an informational system message. These messages
// are typically used for status updates, notifications, or other non-error system
// communications to the user.
func (c *CLI) DisplayInfo(message string) {
	fmt.Println(c.renderer.RenderSystemMessage(message, time.Now()).Content)
}

// DisplayExtensionBlock renders a custom styled block with the given border
// color and optional subtitle. Used by extensions via ctx.PrintBlock.
func (c *CLI) DisplayExtensionBlock(text, borderColor, subtitle string) {
	theme := GetTheme()

	var borderClr = lipgloss.Color("#89b4fa")
	if borderColor != "" {
		borderClr = lipgloss.Color(borderColor)
	}

	content := text
	if subtitle != "" {
		sub := lipgloss.NewStyle().Foreground(theme.VeryMuted).Render(" " + subtitle)
		content = content + "\n" + sub
	}

	rendered := renderContentBlock(
		content,
		c.width,
		WithAlign(lipgloss.Left),
		WithBorderColor(borderClr),
		WithMarginBottom(1),
	)
	fmt.Println(rendered)
}

// DisplayCancellation displays a system message indicating that the current
// AI generation has been cancelled by the user (typically via ESC key).
func (c *CLI) DisplayCancellation() {
	fmt.Println(c.renderer.RenderSystemMessage("Generation cancelled by user (ESC pressed)", time.Now()).Content)
}

// DisplayDebugMessage renders and displays a debug message if debug mode is enabled.
// Debug messages are formatted distinctively and only shown when the CLI is
// initialized with debug=true.
func (c *CLI) DisplayDebugMessage(message string) {
	if !c.debug {
		return
	}
	fmt.Println(c.renderer.RenderDebugMessage(message, time.Now()).Content)
}

// DisplayDebugConfig renders and displays configuration settings in a formatted
// debug message. The config parameter should contain key-value pairs representing
// configuration options that will be displayed for debugging purposes.
func (c *CLI) DisplayDebugConfig(config map[string]any) {
	fmt.Println(c.renderer.RenderDebugConfigMessage(config, time.Now()).Content)
}

// UpdateUsageFromResponse records token usage using metadata from the fantasy
// response when available. Falls back to text-based estimation if the metadata is
// missing or appears unreliable. This provides more accurate usage tracking when
// providers supply token count information.
func (c *CLI) UpdateUsageFromResponse(response *fantasy.Response, inputText string) {
	if c.usageTracker == nil {
		return
	}

	usage := response.Usage
	inputTokens := int(usage.InputTokens)
	outputTokens := int(usage.OutputTokens)

	// Validate that the metadata seems reasonable
	if inputTokens > 0 && outputTokens > 0 {
		cacheReadTokens := int(usage.CacheReadTokens)
		cacheWriteTokens := int(usage.CacheCreationTokens)
		c.usageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
		// Per-response usage is a single API call, so it represents the
		// actual context window fill level.
		c.usageTracker.SetContextTokens(inputTokens + outputTokens)
	} else {
		// Fallback to estimation if no metadata is available.
		// EstimateAndUpdateUsage sets context tokens internally.
		c.usageTracker.EstimateAndUpdateUsage(inputText, response.Content.Text())
	}
}

// DisplayUsageAfterResponse renders and displays token usage information immediately
// following an AI response. This provides real-time feedback about the cost and
// token consumption of each interaction.
func (c *CLI) DisplayUsageAfterResponse() {
	if c.usageTracker == nil {
		return
	}

	usageInfo := c.usageTracker.RenderUsageInfo()
	if usageInfo != "" {
		paddedUsage := lipgloss.NewStyle().
			PaddingLeft(2).
			PaddingTop(1).
			Render(usageInfo)
		fmt.Println(paddedUsage)
	}
}

// updateSize updates the CLI size based on terminal dimensions
func (c *CLI) updateSize() {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		c.width = 80 // Fallback width
		return
	}

	// Add left and right padding (4 characters total: 2 on each side)
	paddingTotal := 4
	c.width = width - paddingTotal

	// Update renderer if it exists
	if c.renderer != nil {
		c.renderer.SetWidth(c.width)
	}
	if c.usageTracker != nil {
		c.usageTracker.SetWidth(c.width)
	}
}
