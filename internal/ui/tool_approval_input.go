package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ToolApprovalInput struct {
	textarea textarea.Model
	toolName string
	toolArgs string
	width    int
	selected bool // true when "yes" is highlighted and false when "no" is
	approved bool
	done     bool
}

func NewToolApprovalInput(toolName, toolArgs string, width int) *ToolApprovalInput {
	ta := textarea.New()
	ta.Placeholder = ""
	ta.ShowLineNumbers = false
	ta.CharLimit = 1000
	ta.SetWidth(width - 8) // Account for container padding, border and internal padding
	ta.SetHeight(4)        // Default to 3 lines like huh
	ta.Focus()

	// Style the textarea to match huh theme
	styles := ta.Styles()
	styles.Focused.Base = lipgloss.NewStyle()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styles.Focused.Prompt = lipgloss.NewStyle()
	styles.Focused.CursorLine = lipgloss.NewStyle()
	ta.SetStyles(styles)

	return &ToolApprovalInput{
		textarea: ta,
		toolName: toolName,
		toolArgs: toolArgs,
		width:    width,
		selected: true,
	}
}

func (t *ToolApprovalInput) Init() tea.Cmd {
	return textarea.Blink
}

func (t *ToolApprovalInput) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			t.approved = true
			t.done = true
			return t, tea.Quit
		case "n", "N":
			t.approved = false
			t.done = true
			return t, tea.Quit
		case "left":
			t.selected = true
			return t, nil
		case "right":
			t.selected = false
			return t, nil
		case "enter":
			t.approved = t.selected
			t.done = true
			return t, tea.Quit
		case "esc", "ctrl+c":
			t.approved = false
			t.done = true
			return t, tea.Quit
		}
	}
	return t, nil
}

func (t *ToolApprovalInput) View() tea.View {
	if t.done {
		return tea.NewView("we are done")
	}
	// Add left padding to entire component (2 spaces like other UI elements)
	containerStyle := lipgloss.NewStyle().PaddingLeft(2)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		MarginBottom(1)

	// Input box with huh-like styling
	inputBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(lipgloss.Color("39")).
		PaddingLeft(1).
		Width(t.width - 2) // Account for container padding

	// Style for the currently selected/highlighted option
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")). // Bright green
		Bold(true).
		Underline(true)

	// Style for the unselected/unhighlighted option
	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Dark gray

	// Build the view
	var view strings.Builder
	view.WriteString(titleStyle.Render("Allow tool execution"))
	view.WriteString("\n")
	details := fmt.Sprintf("Tool: %s\nArguments: %s\n\n", t.toolName, t.toolArgs)
	view.WriteString(details)
	view.WriteString("Allow tool execution: ")

	var yesText, noText string
	if t.selected {
		yesText = selectedStyle.Render("[y]es")
		noText = unselectedStyle.Render("[n]o")
	} else {
		yesText = unselectedStyle.Render("[y]es")
		noText = selectedStyle.Render("[n]o")
	}
	view.WriteString(yesText + "/" + noText + "\n")

	return tea.NewView(containerStyle.Render(inputBoxStyle.Render(view.String())))
}
