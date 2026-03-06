package ui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/models"
)

// ModelEntry holds display metadata for a single model in the selector.
type ModelEntry struct {
	Provider     string
	ModelID      string
	Name         string // human-friendly name (e.g. "Claude Haiku 4.5")
	ContextLimit int
	Reasoning    bool
}

// ModelSelectedMsg is sent when the user selects a model from the selector.
type ModelSelectedMsg struct {
	ModelString string // "provider/model-id"
}

// ModelSelectorCancelledMsg is sent when the user cancels the selector.
type ModelSelectorCancelledMsg struct{}

// ModelSelectorComponent is a full-screen Bubble Tea component that displays
// a filterable list of available models. It follows the same pattern as
// TreeSelectorComponent: inline text search, scrolling list, and custom
// messages for result delivery.
type ModelSelectorComponent struct {
	allModels    []ModelEntry // all available models (pre-sorted)
	filtered     []ModelEntry // subset matching the current search
	cursor       int
	search       string
	currentModel string // "provider/model" of the active model (for checkmark)
	width        int
	height       int
	active       bool
}

// NewModelSelector creates a model selector populated from the global registry,
// filtered to only providers with configured API keys.
func NewModelSelector(currentModel string, width, height int) *ModelSelectorComponent {
	registry := models.GetGlobalRegistry()
	var allModels []ModelEntry

	for _, providerID := range registry.GetFantasyProviders() {
		// Only include providers with valid API keys configured.
		if err := registry.ValidateEnvironment(providerID, ""); err != nil {
			continue
		}

		modelsMap, err := registry.GetModelsForProvider(providerID)
		if err != nil {
			continue
		}

		for modelID, info := range modelsMap {
			allModels = append(allModels, ModelEntry{
				Provider:     providerID,
				ModelID:      modelID,
				Name:         info.Name,
				ContextLimit: info.Limit.Context,
				Reasoning:    info.Reasoning,
			})
		}
	}

	// Sort: alphabetically by model ID, grouped by provider.
	sort.Slice(allModels, func(i, j int) bool {
		if allModels[i].Provider != allModels[j].Provider {
			return allModels[i].Provider < allModels[j].Provider
		}
		return allModels[i].ModelID < allModels[j].ModelID
	})

	ms := &ModelSelectorComponent{
		allModels:    allModels,
		filtered:     allModels,
		currentModel: currentModel,
		width:        width,
		height:       height,
		active:       true,
	}

	// Position cursor on the current model if found.
	for i, m := range ms.filtered {
		if m.Provider+"/"+m.ModelID == currentModel {
			ms.cursor = i
			break
		}
	}

	return ms
}

// Init implements tea.Model.
func (ms *ModelSelectorComponent) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (ms *ModelSelectorComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		ms.width = msg.Width
		ms.height = msg.Height
		return ms, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if ms.cursor > 0 {
				ms.cursor--
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if ms.cursor < len(ms.filtered)-1 {
				ms.cursor++
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgup"))):
			ms.cursor -= ms.visibleHeight()
			if ms.cursor < 0 {
				ms.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("pgdown"))):
			ms.cursor += ms.visibleHeight()
			if ms.cursor >= len(ms.filtered) {
				ms.cursor = len(ms.filtered) - 1
			}
			if ms.cursor < 0 {
				ms.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			ms.cursor = 0

		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			ms.cursor = max(len(ms.filtered)-1, 0)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if ms.cursor < len(ms.filtered) {
				entry := ms.filtered[ms.cursor]
				ms.active = false
				return ms, func() tea.Msg {
					return ModelSelectedMsg{
						ModelString: entry.Provider + "/" + entry.ModelID,
					}
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if ms.search != "" {
				ms.search = ""
				ms.rebuildFiltered()
			} else {
				ms.active = false
				return ms, func() tea.Msg {
					return ModelSelectorCancelledMsg{}
				}
			}

		default:
			// Inline text search.
			if msg.Text != "" && len(msg.Text) == 1 {
				ch := msg.Text[0]
				if ch >= 32 && ch < 127 {
					ms.search += string(ch)
					ms.rebuildFiltered()
				}
			}
			if key.Matches(msg, key.NewBinding(key.WithKeys("backspace"))) && len(ms.search) > 0 {
				ms.search = ms.search[:len(ms.search)-1]
				ms.rebuildFiltered()
			}
		}
	}
	return ms, nil
}

// View implements tea.Model.
func (ms *ModelSelectorComponent) View() tea.View {
	theme := GetTheme()

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Accent).
		PaddingLeft(2)

	helpStyle := lipgloss.NewStyle().
		Foreground(theme.Muted).
		PaddingLeft(2)

	infoStyle := lipgloss.NewStyle().
		Foreground(theme.Warning).
		PaddingLeft(2)

	var b strings.Builder

	// Header.
	b.WriteString(headerStyle.Render("Model Selector"))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓: move  enter: select  esc: cancel  type to filter"))
	b.WriteString("\n")
	b.WriteString(infoStyle.Render("Only showing models with configured API keys"))
	b.WriteString("\n")

	// Search input.
	searchStyle := lipgloss.NewStyle().Foreground(theme.Info).PaddingLeft(2)
	if ms.search != "" {
		b.WriteString(searchStyle.Render(fmt.Sprintf("> %s", ms.search)))
	} else {
		b.WriteString(searchStyle.Render("> "))
	}
	b.WriteString("\n")

	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", ms.width)))
	b.WriteString("\n")

	if len(ms.filtered) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(theme.Muted).PaddingLeft(2)
		if ms.search != "" {
			b.WriteString(emptyStyle.Render("No models matching \"" + ms.search + "\""))
		} else {
			b.WriteString(emptyStyle.Render("No models available (check API keys)"))
		}
		b.WriteString("\n")
	} else {
		// Visible window.
		visH := ms.visibleHeight()
		startIdx := 0
		if ms.cursor >= visH {
			startIdx = ms.cursor - visH + 1
		}
		endIdx := min(startIdx+visH, len(ms.filtered))

		for i := startIdx; i < endIdx; i++ {
			entry := ms.filtered[i]
			line := ms.renderEntry(entry, i == ms.cursor)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}

	// Footer.
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(strings.Repeat("─", ms.width)))
	b.WriteString("\n")

	footerParts := []string{
		fmt.Sprintf("(%d/%d)", ms.cursor+1, len(ms.filtered)),
	}
	if ms.cursor < len(ms.filtered) {
		entry := ms.filtered[ms.cursor]
		if entry.Name != "" {
			footerParts = append(footerParts, fmt.Sprintf("Model Name: %s", entry.Name))
		}
		if entry.ContextLimit > 0 {
			footerParts = append(footerParts, fmt.Sprintf("Context: %dK", entry.ContextLimit/1000))
		}
	}

	footerStyle := lipgloss.NewStyle().Foreground(theme.Muted).PaddingLeft(2)
	b.WriteString(footerStyle.Render(strings.Join(footerParts, "  ")))

	return tea.NewView(b.String())
}

// IsActive returns whether the selector is still accepting input.
func (ms *ModelSelectorComponent) IsActive() bool {
	return ms.active
}

// --- Internal helpers ---

func (ms *ModelSelectorComponent) visibleHeight() int {
	// Reserve: header(1) + help(1) + info(1) + search(1) + separator(1) + footer(2) = 7
	h := max(ms.height-7, 5)
	return h
}

func (ms *ModelSelectorComponent) rebuildFiltered() {
	if ms.search == "" {
		ms.filtered = ms.allModels
	} else {
		query := strings.ToLower(ms.search)
		ms.filtered = ms.filtered[:0]

		type scored struct {
			entry ModelEntry
			score int
		}
		var matches []scored

		for _, entry := range ms.allModels {
			s := ms.fuzzyScoreModel(query, entry)
			if s > 0 {
				matches = append(matches, scored{entry: entry, score: s})
			}
		}

		// Sort by score descending, then alphabetically.
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].score != matches[j].score {
				return matches[i].score > matches[j].score
			}
			return matches[i].entry.ModelID < matches[j].entry.ModelID
		})

		ms.filtered = make([]ModelEntry, len(matches))
		for i, m := range matches {
			ms.filtered[i] = m.entry
		}
	}

	// Clamp cursor.
	if ms.cursor >= len(ms.filtered) {
		ms.cursor = max(len(ms.filtered)-1, 0)
	}
}

// fuzzyScoreModel scores a model entry against the search query.
func (ms *ModelSelectorComponent) fuzzyScoreModel(query string, entry ModelEntry) int {
	modelID := strings.ToLower(entry.ModelID)
	provider := strings.ToLower(entry.Provider)
	name := strings.ToLower(entry.Name)
	combined := provider + "/" + modelID

	// Exact match on combined provider/model.
	if combined == query {
		return 1000
	}

	// Exact match on model ID.
	if modelID == query {
		return 950
	}

	// Prefix match on model ID.
	if strings.HasPrefix(modelID, query) {
		return 800 - len(modelID) + len(query)
	}

	// Prefix match on combined.
	if strings.HasPrefix(combined, query) {
		return 750 - len(combined) + len(query)
	}

	// Contains match on model ID.
	if strings.Contains(modelID, query) {
		return 600
	}

	// Contains match on combined.
	if strings.Contains(combined, query) {
		return 550
	}

	// Contains match on name.
	if strings.Contains(name, query) {
		return 400
	}

	// Character-by-character fuzzy match on model ID.
	if s := fuzzyCharacterMatch(query, modelID); s > 0 {
		return s
	}

	// Fuzzy match on combined.
	if s := fuzzyCharacterMatch(query, combined); s > 0 {
		return s - 20
	}

	return 0
}

func (ms *ModelSelectorComponent) renderEntry(entry ModelEntry, isCursor bool) string {
	theme := GetTheme()
	modelStr := entry.ModelID
	providerStr := fmt.Sprintf("[%s]", entry.Provider)

	// Cursor indicator.
	var cursor string
	if isCursor {
		cursor = lipgloss.NewStyle().Foreground(theme.Accent).Render("-> ")
	} else {
		cursor = "   "
	}

	// Active model checkmark.
	var active string
	if entry.Provider+"/"+entry.ModelID == ms.currentModel {
		active = lipgloss.NewStyle().Foreground(theme.Success).Render(" \u2713")
	}

	// Style the model ID.
	modelStyle := lipgloss.NewStyle().Foreground(theme.Text)
	if isCursor {
		modelStyle = modelStyle.Bold(true).Foreground(theme.Accent)
	}

	// Style the provider tag.
	providerStyle := lipgloss.NewStyle().Foreground(theme.Muted)

	return cursor + modelStyle.Render(modelStr) + " " + providerStyle.Render(providerStr) + active
}
