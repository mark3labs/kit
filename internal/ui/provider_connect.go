package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/auth"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/ui/style"
)

// ProviderConnectEntry describes one provider in the /connect picker.
type ProviderConnectEntry struct {
	ID        string   // provider ID as used in model strings
	Name      string   // human-friendly name
	EnvVars   []string // environment variables the provider reads
	Connected bool     // a key or token is already available
	OAuthOnly bool     // provider has no API-key auth (GitHub Copilot)
}

// ProviderKeySubmittedMsg is sent when the user enters an API key for a
// provider and presses Enter.
type ProviderKeySubmittedMsg struct {
	ProviderID   string
	ProviderName string
	Key          string
}

// ProviderConnectOAuthMsg is sent when the user picks a provider that only
// supports OAuth, so the app can print the matching login command.
type ProviderConnectOAuthMsg struct {
	ProviderID   string
	ProviderName string
}

// ProviderConnectCancelledMsg is sent when the user closes the picker
// without entering a key.
type ProviderConnectCancelledMsg struct{}

// connectStage is the step the /connect dialog is on.
type connectStage int

const (
	connectStageList connectStage = iota // choose a provider
	connectStageKey                      // type the key
)

// ProviderConnectComponent is the /connect dialog. It is a two-step modal:
// a searchable provider list (reusing PopupList, like /model) followed by a
// masked key input for the chosen provider. It mirrors the "Connect a
// provider" flow in opencode but hides the key while it is typed.
type ProviderConnectComponent struct {
	popup  *PopupList
	stage  connectStage
	entry  ProviderConnectEntry
	input  textinput.Model
	errMsg string
	width  int
	height int
	active bool
}

// connectPriority orders the most common providers first; everything else
// follows alphabetically by display name.
var connectPriority = map[string]int{
	"anthropic":      0,
	"openai":         1,
	"google":         2,
	"openrouter":     3,
	"opencode":       4,
	"github-copilot": 5,
	"groq":           6,
	"deepseek":       7,
	"xai":            8,
	"mistral":        9,
}

// ListConnectableProviders builds the provider entries for the picker from
// the model registry. Providers without an environment variable (local or
// SDK-credential providers such as ollama, bedrock, custom) are skipped:
// they do not take an API key.
func ListConnectableProviders() []ProviderConnectEntry {
	registry := models.GetGlobalRegistry()
	var cm *auth.CredentialManager
	if m, err := auth.NewCredentialManager(); err == nil {
		cm = m
	}

	var entries []ProviderConnectEntry
	for _, id := range registry.GetLLMProviders() {
		info := registry.GetProviderInfo(id)
		if info == nil {
			continue
		}
		oauthOnly := id == "github-copilot"
		if len(info.Env) == 0 && !oauthOnly {
			continue
		}
		name := info.Name
		if name == "" {
			name = id
		}
		connected := registry.ValidateEnvironment(id, "") == nil
		if !connected && cm != nil {
			if has, _ := cm.HasProviderCredentials(id); has {
				connected = true
			}
		}
		entries = append(entries, ProviderConnectEntry{
			ID:        id,
			Name:      name,
			EnvVars:   info.Env,
			Connected: connected,
			OAuthOnly: oauthOnly,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		pi, iok := connectPriority[entries[i].ID]
		pj, jok := connectPriority[entries[j].ID]
		switch {
		case iok && jok:
			return pi < pj
		case iok:
			return true
		case jok:
			return false
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries
}

// NewProviderConnect creates the picker. When preselect names a provider
// (e.g. "/connect groq"), the list step is skipped and the key input opens
// for that provider directly.
func NewProviderConnect(preselect string, width, height int) *ProviderConnectComponent {
	entries := ListConnectableProviders()

	items := make([]PopupItem, 0, len(entries))
	for _, e := range entries {
		desc := strings.Join(e.EnvVars, " / ")
		if e.OAuthOnly {
			desc = "device login (kit auth login copilot)"
		}
		items = append(items, PopupItem{
			Label:       e.Name,
			Description: desc,
			Active:      e.Connected,
			Meta:        e,
		})
	}

	popup := NewPopupList("Connect a provider", items, width, height)
	popup.Subtitle = "✓ = credentials found · keys are stored in " + tildeHome(credentialsPathOrDefault())
	popup.FilterFunc = filterProviderEntries
	popup.SetCursor(0)

	c := &ProviderConnectComponent{
		popup:  popup,
		stage:  connectStageList,
		width:  width,
		height: height,
		active: true,
	}

	if preselect != "" {
		want := strings.ToLower(strings.TrimSpace(preselect))
		if want == "copilot" {
			want = "github-copilot"
		}
		if want == "gemini" {
			want = "google"
		}
		for _, e := range entries {
			if e.ID == want && !e.OAuthOnly {
				c.openKeyInput(e)
				break
			}
		}
		if c.stage == connectStageList {
			// Unknown or OAuth-only provider: fall back to the list with the
			// query pre-filled so the user sees the closest matches.
			popup.SetSearch(preselect)
			popup.rebuildFiltered()
		}
	}
	return c
}

// credentialsPathOrDefault returns the credentials file path for display.
func credentialsPathOrDefault() string {
	if cm, err := auth.NewCredentialManager(); err == nil {
		return cm.GetCredentialsPath()
	}
	return "~/.config/.kit/credentials.json"
}

// openKeyInput switches to the masked key input for entry.
func (c *ProviderConnectComponent) openKeyInput(entry ProviderConnectEntry) {
	theme := style.GetTheme()
	ti := textinput.New()
	ti.Placeholder = "paste or type the API key"
	ti.Prompt = "› "
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 0
	ti.SetWidth(c.inputWidth())
	ti.SetVirtualCursor(true)

	styles := ti.Styles()
	styles.Focused.Text = lipgloss.NewStyle().Foreground(theme.Text)
	styles.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.Accent)
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.VeryMuted)
	styles.Cursor.Color = theme.Accent
	ti.SetStyles(styles)

	ti.Focus()

	c.entry = entry
	c.input = ti
	c.errMsg = ""
	c.stage = connectStageKey
}

// inputWidth returns the usable width for the key input.
func (c *ProviderConnectComponent) inputWidth() int {
	popupW := max(min(c.width-4, 80), 20)
	return max(popupW-6-2, 10) // border+padding, then the prompt glyph
}

// SetError shows a validation/save error under the key input and keeps the
// dialog open so the user can correct the key.
func (c *ProviderConnectComponent) SetError(msg string) {
	c.errMsg = msg
	c.active = true
	c.stage = connectStageKey
}

// Init implements tea.Model.
func (c *ProviderConnectComponent) Init() tea.Cmd { return textinput.Blink }

// Update implements tea.Model.
func (c *ProviderConnectComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
		c.popup.SetSize(msg.Width, msg.Height)
		if c.stage == connectStageKey {
			c.input.SetWidth(c.inputWidth())
		}
		return c, nil

	case tea.KeyPressMsg:
		if c.stage == connectStageList {
			return c.updateList(msg)
		}
		return c.updateKey(msg)
	}

	// Cursor blink, paste, etc. belong to the text input.
	if c.stage == connectStageKey {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return c, cmd
	}
	return c, nil
}

func (c *ProviderConnectComponent) updateList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	result := c.popup.HandleKey(msg.String(), msg.Text)

	if result.Selected != nil {
		entry, _ := result.Selected.Meta.(ProviderConnectEntry)
		if entry.OAuthOnly {
			c.active = false
			return c, func() tea.Msg {
				return ProviderConnectOAuthMsg{ProviderID: entry.ID, ProviderName: entry.Name}
			}
		}
		c.openKeyInput(entry)
		return c, textinput.Blink
	}
	if result.Cancelled {
		c.active = false
		return c, func() tea.Msg { return ProviderConnectCancelledMsg{} }
	}
	return c, nil
}

func (c *ProviderConnectComponent) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		key := strings.TrimSpace(c.input.Value())
		if key == "" {
			c.errMsg = "The key cannot be empty."
			return c, nil
		}
		c.active = false
		entry := c.entry
		return c, func() tea.Msg {
			return ProviderKeySubmittedMsg{ProviderID: entry.ID, ProviderName: entry.Name, Key: key}
		}
	case "esc":
		// Back to the provider list; a second Esc there closes the dialog.
		c.stage = connectStageList
		c.errMsg = ""
		return c, nil
	}

	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	c.errMsg = ""
	return c, cmd
}

// View implements tea.Model. Not used for overlay rendering; see RenderOverlay.
func (c *ProviderConnectComponent) View() tea.View {
	v := tea.NewView(compositeCentered("", c.RenderOverlay(), c.width, c.height))
	v.AltScreen = true
	return v
}

// RenderOverlay returns the dialog as a bare box for compositing over the
// conversation, matching the other selector overlays.
func (c *ProviderConnectComponent) RenderOverlay() string {
	if c.stage == connectStageList {
		return c.popup.Render()
	}
	return c.renderKeyInput()
}

// IsActive returns whether the dialog is still accepting input.
func (c *ProviderConnectComponent) IsActive() bool { return c.active }

// renderKeyInput draws the second step: a box styled like PopupList holding
// the provider title, a short description, the masked input and a hint line.
func (c *ProviderConnectComponent) renderKeyInput() string {
	theme := style.GetTheme()
	popupW := max(min(c.width-4, 80), 20)
	innerW := max(popupW-6, 10)
	bg := theme.Background

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Background(bg).
		Padding(1, 2).
		Width(popupW)

	title := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Background(bg).Width(innerW)
	muted := lipgloss.NewStyle().Foreground(theme.Muted).Background(bg).Width(innerW)
	text := lipgloss.NewStyle().Foreground(theme.Text).Background(bg).Width(innerW)
	errStyle := lipgloss.NewStyle().Foreground(theme.Error).Background(bg).Width(innerW)
	sep := lipgloss.NewStyle().Foreground(theme.Muted).Background(bg)

	var lines []string
	lines = append(lines, title.Render(c.entry.Name+" API key"))

	desc := fmt.Sprintf("Enter your %s API key. It is saved with mode 0600 to", c.entry.Name)
	lines = append(lines, muted.Render(desc))
	lines = append(lines, muted.Render(tildeHome(credentialsPathOrDefault())))
	if len(c.entry.EnvVars) > 0 {
		lines = append(lines, muted.Render("The stored key takes precedence over "+strings.Join(c.entry.EnvVars, " / ")+"."))
	}
	lines = append(lines, sep.Render(strings.Repeat("─", innerW)))
	lines = append(lines, text.Render(c.input.View()))
	if c.errMsg != "" {
		lines = append(lines, errStyle.Render(c.errMsg))
	} else {
		lines = append(lines, muted.Render("Input is hidden."))
	}
	lines = append(lines, sep.Render(strings.Repeat("─", innerW)))
	lines = append(lines, muted.Render("Enter save · Esc back to provider list"))

	return box.Render(strings.Join(lines, "\n"))
}

// filterProviderEntries matches the query against provider ID, display
// name and env var names, then ranks by match quality.
func filterProviderEntries(query string, items []PopupItem) []PopupItem {
	if query == "" {
		return items
	}
	q := strings.ToLower(query)

	type scored struct {
		item  PopupItem
		score int
	}
	var matches []scored
	for _, item := range items {
		e, ok := item.Meta.(ProviderConnectEntry)
		if !ok {
			continue
		}
		s := scoreProviderEntry(q, e)
		if s > 0 {
			matches = append(matches, scored{item: item, score: s})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})
	out := make([]PopupItem, len(matches))
	for i, m := range matches {
		out[i] = m.item
	}
	return out
}

func scoreProviderEntry(q string, e ProviderConnectEntry) int {
	id := strings.ToLower(e.ID)
	name := strings.ToLower(e.Name)
	switch {
	case id == q || name == q:
		return 1000
	case strings.HasPrefix(id, q):
		return 900 - len(id)
	case strings.HasPrefix(name, q):
		return 850 - len(name)
	case strings.Contains(id, q):
		return 600
	case strings.Contains(name, q):
		return 550
	}
	for _, env := range e.EnvVars {
		if strings.Contains(strings.ToLower(env), q) {
			return 400
		}
	}
	if s := fuzzyCharacterMatch(q, id); s > 0 {
		return s
	}
	if s := fuzzyCharacterMatch(q, name); s > 0 {
		return s - 10
	}
	return 0
}
