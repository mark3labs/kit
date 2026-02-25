package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ApprovalComponent is the tool approval dialog for the parent AppModel.
// It displays tool name and arguments, lets the user approve or deny the call,
// and returns an approvalResultMsg tea.Cmd instead of tea.Quit — lifecycle is
// entirely managed by the parent.
//
// Key bindings:
//   - y / Y      → approve immediately
//   - n / N      → deny immediately
//   - left        → select "yes"
//   - right       → select "no"
//   - enter       → confirm current selection
//   - esc / ctrl+c → deny (same as "no")
type ApprovalComponent struct {
	toolName string
	toolArgs string
	width    int
	selected bool // true = "yes" highlighted, false = "no" highlighted
}

// NewApprovalComponent creates a new ApprovalComponent for the given tool call.
// width is the terminal width passed down from the parent model.
// By default the "yes" option is highlighted.
func NewApprovalComponent(toolName, toolArgs string, width int) *ApprovalComponent {
	return &ApprovalComponent{
		toolName: toolName,
		toolArgs: toolArgs,
		width:    width,
		selected: true, // default to "yes"
	}
}

// Init implements tea.Model. No startup commands needed.
func (a *ApprovalComponent) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model. Handles keyboard input and returns an
// approvalResultMsg tea.Cmd when the user makes a decision.
// It does NOT return tea.Quit — the parent owns the program lifecycle.
func (a *ApprovalComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "y", "Y":
			return a, approvalResult(true)
		case "n", "N":
			return a, approvalResult(false)
		case "left":
			a.selected = true
			return a, nil
		case "right":
			a.selected = false
			return a, nil
		case "enter":
			return a, approvalResult(a.selected)
		case "esc", "ctrl+c":
			return a, approvalResult(false)
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
	}
	return a, nil
}

// View implements tea.Model. Renders the approval dialog with tool info and
// yes/no selection.
func (a *ApprovalComponent) View() tea.View {
	// Add left padding to entire component (2 spaces like other UI elements).
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
		Width(a.width - 2) // Account for container padding

	// Style for the currently selected option
	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")). // Bright green
		Bold(true).
		Underline(true)

	// Style for the unselected option
	unselectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")) // Dark gray

	var view strings.Builder
	view.WriteString(titleStyle.Render("Allow tool execution"))
	view.WriteString("\n")
	view.WriteString(fmt.Sprintf("Tool: %s\nArguments: %s\n\n", a.toolName, a.toolArgs))
	view.WriteString("Allow tool execution: ")

	var yesText, noText string
	if a.selected {
		yesText = selectedStyle.Render("[y]es")
		noText = unselectedStyle.Render("[n]o")
	} else {
		yesText = unselectedStyle.Render("[y]es")
		noText = selectedStyle.Render("[n]o")
	}
	view.WriteString(yesText + "/" + noText + "\n")

	return tea.NewView(containerStyle.Render(inputBoxStyle.Render(view.String())))
}

// approvalResult returns a tea.Cmd that emits an approvalResultMsg with the
// given decision. The parent AppModel receives this and sends the result on
// the stored approvalChan.
func approvalResult(approved bool) tea.Cmd {
	return func() tea.Msg {
		return approvalResultMsg{Approved: approved}
	}
}
