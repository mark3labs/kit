package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

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

// ModelSelectorComponent is a Bubble Tea component that displays a filterable
// list of available models as a centered overlay popup. It delegates rendering
// and keyboard navigation to PopupList and converts results into the
// ModelSelectedMsg / ModelSelectorCancelledMsg messages expected by AppModel.
type ModelSelectorComponent struct {
	popup        *PopupList
	allModels    []ModelEntry // kept for the custom filter callback
	currentModel string       // "provider/model" of the active model
	width        int
	height       int
	active       bool
}

// NewModelSelector creates a model selector populated from the global registry,
// filtered to only providers with configured API keys.
func NewModelSelector(currentModel string, width, height int) *ModelSelectorComponent {
	registry := models.GetGlobalRegistry()
	var allModels []ModelEntry

	for _, providerID := range registry.GetLLMProviders() {
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

	// Build PopupItems from model entries.
	items := make([]PopupItem, len(allModels))
	for i, m := range allModels {
		items[i] = PopupItem{
			Label:       m.ModelID,
			Description: fmt.Sprintf("[%s]", m.Provider),
			Active:      m.Provider+"/"+m.ModelID == currentModel,
			Meta:        m,
		}
	}

	popup := NewPopupList("Model Selector", items, width, height)
	popup.Subtitle = "Only showing models with configured API keys"
	popup.FilterFunc = func(query string, allItems []PopupItem) []PopupItem {
		return filterModels(query, allItems)
	}

	return &ModelSelectorComponent{
		popup:        popup,
		allModels:    allModels,
		currentModel: currentModel,
		width:        width,
		height:       height,
		active:       true,
	}
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
		ms.popup.SetSize(msg.Width, msg.Height)
		return ms, nil

	case tea.KeyPressMsg:
		result := ms.popup.HandleKey(msg.String(), msg.Text)

		if result.Selected != nil {
			ms.active = false
			entry := result.Selected.Meta.(ModelEntry)
			modelStr := entry.Provider + "/" + entry.ModelID
			return ms, func() tea.Msg {
				return ModelSelectedMsg{ModelString: modelStr}
			}
		}
		if result.Cancelled {
			ms.active = false
			return ms, func() tea.Msg {
				return ModelSelectorCancelledMsg{}
			}
		}
	}
	return ms, nil
}

// View implements tea.Model — not used for overlay rendering.
// Use RenderOverlay for the centered overlay approach.
func (ms *ModelSelectorComponent) View() tea.View {
	// Fallback full-screen rendering (unused when rendered as overlay).
	v := tea.NewView(ms.popup.RenderCentered(ms.width, ms.height))
	v.AltScreen = true
	return v
}

// RenderOverlay returns the popup as a centered overlay string, ready to be
// composited on top of the main content via overlayContent().
func (ms *ModelSelectorComponent) RenderOverlay(termWidth, termHeight int) string {
	return ms.popup.RenderCentered(termWidth, termHeight)
}

// IsActive returns whether the selector is still accepting input.
func (ms *ModelSelectorComponent) IsActive() bool {
	return ms.active
}

// --- Model-specific fuzzy filter ---

// filterModels scores and filters PopupItems whose Meta is a ModelEntry.
func filterModels(query string, items []PopupItem) []PopupItem {
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
		entry, ok := item.Meta.(ModelEntry)
		if !ok {
			continue
		}
		s := fuzzyScoreModelEntry(q, entry)
		if s > 0 {
			matches = append(matches, scored{item: item, score: s})
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		a := matches[i].item.Meta.(ModelEntry)
		b := matches[j].item.Meta.(ModelEntry)
		return a.ModelID < b.ModelID
	})

	result := make([]PopupItem, len(matches))
	for i, m := range matches {
		result[i] = m.item
	}
	return result
}

// fuzzyScoreModelEntry scores a model entry against the search query.
func fuzzyScoreModelEntry(query string, entry ModelEntry) int {
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
