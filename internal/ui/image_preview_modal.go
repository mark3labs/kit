package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mark3labs/kit/internal/ui/imagepreview"
	"github.com/mark3labs/kit/internal/ui/style"
)

// imagePreviewModal owns a separate, viewport-sized rendering of an attachment.
type imagePreviewModal struct {
	index, generation int
	content, place    string
	imageID           uint32
}

type attachmentPreviewReadyMsg struct{ thumbnailReadyMsg }

func (m *AppModel) openImagePreview(index int) tea.Cmd {
	m.imagePreviewGeneration++
	m.imagePreview = &imagePreviewModal{index: index, generation: m.imagePreviewGeneration}
	var drops string
	if in, ok := m.input.(*InputComponent); ok {
		for _, id := range in.imageIDs {
			drops += imagepreview.DeletePlacements(id)
		}
	}
	return tea.Sequence(tea.Raw(drops), m.renderImagePreviewCmd())
}

func (m *AppModel) renderImagePreviewCmd() tea.Cmd {
	p := m.imagePreview
	in := m.input.(*InputComponent)
	cmd := renderThumbnailCmd(in.pendingImages[p.index], max(1, m.width-8), max(1, m.height-8), style.GetTheme().Background, p.generation, p.index)
	return func() tea.Msg { return attachmentPreviewReadyMsg{cmd().(thumbnailReadyMsg)} }
}

func (m *AppModel) updateImagePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	p := m.imagePreview
	switch msg := msg.(type) {
	case attachmentPreviewReadyMsg:
		if msg.gen != p.generation {
			return m, nil
		}
		p.content, p.place, p.imageID = msg.thumb, msg.place, msg.imageID
		if p.content == "" {
			p.content = "Image preview unavailable"
		}
		return m, tea.Sequence(tea.Raw(msg.transmit), gfxNudgeCmd())
	case tea.KeyPressMsg:
		switch msg.Code {
		case tea.KeyEscape, tea.KeyEnter:
			m.imagePreview = nil
			return m, tea.Sequence(tea.Raw(imagepreview.DeleteImage(p.imageID)), gfxNudgeCmd())
		case tea.KeyLeft, tea.KeyRight:
			in := m.input.(*InputComponent)
			delta := 1
			if msg.Code == tea.KeyLeft {
				delta = -1
			}
			index := (p.index + delta + len(in.pendingImages)) % len(in.pendingImages)
			cleanup := tea.Raw(imagepreview.DeleteImage(p.imageID))
			return m, tea.Sequence(cleanup, m.openImagePreview(index))
		}
	case tea.WindowSizeMsg:
		cleanup := tea.Raw(imagepreview.DeleteImage(p.imageID))
		m.width, m.height = msg.Width, msg.Height
		return m, tea.Sequence(cleanup, m.openImagePreview(p.index))
	}
	return m, nil
}

func (m *AppModel) renderImagePreview() string {
	p := m.imagePreview
	in := m.input.(*InputComponent)
	content := p.content
	if content == "" {
		content = "Loading image…"
	}
	width := max(1, m.width-4)
	body := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.PlaceHorizontal(width, lipgloss.Center, fmt.Sprintf("Image %d/%d", p.index+1, len(in.pendingImages))),
		lipgloss.PlaceHorizontal(width, lipgloss.Center, content),
		lipgloss.PlaceHorizontal(width, lipgloss.Center, "←/→ browse · Enter/Esc close"))
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Background(style.GetTheme().Background).
		Width(width).MaxWidth(max(1, m.width)).MaxHeight(max(1, m.height)).Render(body)
}

func (m *AppModel) imagePreviewPlacement() []gfxPlacement {
	p := m.imagePreview
	if p == nil || p.place == "" {
		return nil
	}
	box := m.renderImagePreview()
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	// One border row and one title row precede the centered image.
	return []gfxPlacement{{row: max(0, (m.height-bh)/2) + 3,
		col: max(0, (m.width-bw)/2) + 2 + max(0, (bw-2-lipgloss.Width(p.content))/2), id: p.imageID, seq: p.place}}
}
