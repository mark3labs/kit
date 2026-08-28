package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// dirPickerTitle is the modal heading shown on connection in remote mode.
const dirPickerTitle = "Select a working directory"

// RunDirPicker runs a standalone directory-picker modal, starting in
// startDir. It is used by `kit --pick-dir` (spawned by the daemon in the
// user's home directory) so a remote peer can choose where the session
// starts, and is useful locally for launching kit from any shell.
//
// Enter on a directory descends into it; Enter on the trailing
// "use this directory" entry selects the current directory. An empty
// directory (no subdirectories) is selected directly. Returns "" when the
// user cancels.
func RunDirPicker(startDir string) (string, error) {
	if startDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		startDir = home
	}
	m := newDirPickerModel(startDir)
	prog := tea.NewProgram(m)
	final, err := prog.Run()
	if err != nil {
		return "", fmt.Errorf("directory picker: %w", err)
	}
	if dp, ok := final.(*dirPickerModel); ok {
		return dp.selected, nil
	}
	return "", nil
}

// dirEntry is one row in the picker list.
type dirEntry struct {
	path  string // absolute path this row represents
	label string // display label
	here  bool   // true for the trailing "use this directory" row
}

type dirPickerModel struct {
	popup      *PopupList
	cwd        string
	width      int
	height     int
	showHidden bool
	errMsg     string
	quitting   bool

	selected  string
	cancelled bool
}

func newDirPickerModel(startDir string) *dirPickerModel {
	return &dirPickerModel{
		popup: &PopupList{
			Title:      dirPickerTitle,
			Subtitle:   startDir,
			ShowSearch: true,
		},
		cwd: startDir,
	}
}

func (m *dirPickerModel) Init() tea.Cmd { return nil }

func (m *dirPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.popup.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *dirPickerModel) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		m.quitting = true
		return m, tea.Quit
	case "ctrl+h":
		m.showHidden = !m.showHidden
		m.rebuild()
		return m, nil
	case "backspace":
		// PopupList uses backspace to edit the search; when the search is
		// empty, backspace navigates to the parent directory instead.
		if m.popup.Search() == "" {
			return m.goUp()
		}
	}

	result := m.popup.HandleKey(msg.String(), msg.Text)
	switch {
	case result.Cancelled:
		m.cancelled = true
		m.quitting = true
		return m, tea.Quit
	case result.Selected != nil:
		entry, ok := result.Selected.Meta.(dirEntry)
		if !ok {
			return m, nil
		}
		if entry.here {
			m.selected = m.cwd
			m.quitting = true
			return m, tea.Quit
		}
		return m.enterDir(entry.path)
	}
	return m, nil
}

// enterDir descends into path. A directory without subdirectories is
// selected as the session root instead, since there is nothing to descend.
func (m *dirPickerModel) enterDir(path string) (tea.Model, tea.Cmd) {
	subs, err := listDirs(path, m.showHidden)
	if err != nil {
		m.errMsg = err.Error()
		return m, nil
	}
	if len(subs) == 0 {
		m.selected = path
		m.quitting = true
		return m, tea.Quit
	}
	m.cwd = path
	m.errMsg = ""
	m.popup.Subtitle = path
	m.popup.SetSearch("")
	m.rebuild()
	return m, nil
}

func (m *dirPickerModel) goUp() (tea.Model, tea.Cmd) {
	parent := filepath.Dir(m.cwd)
	if parent == m.cwd {
		return m, nil // already at the filesystem root
	}
	return m.enterDir(parent)
}

// rebuild refreshes the popup items for the current directory.
func (m *dirPickerModel) rebuild() {
	subs, err := listDirs(m.cwd, m.showHidden)
	if err != nil {
		m.errMsg = err.Error()
		return
	}
	m.errMsg = ""
	items := make([]PopupItem, 0, len(subs)+1)
	for _, dir := range subs {
		items = append(items, PopupItem{
			Label: "▸ " + dir.name,
			Meta:  dirEntry{path: dir.path, label: dir.name},
		})
	}
	items = append(items, PopupItem{
		Label:       "✓ Use this directory",
		Description: m.cwd,
		Meta:        dirEntry{path: m.cwd, here: true},
	})
	m.popup.SetItems(items)
}

type listedDir struct {
	name string
	path string
}

// listDirs returns the subdirectories of dir, sorted by name. Read errors
// on the directory itself are returned; entries that vanish mid-read or
// cannot be stat'ed are skipped.
func listDirs(dir string, includeHidden bool) ([]listedDir, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", dir, err)
	}
	var out []listedDir
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") && !includeHidden {
			continue
		}
		out = append(out, listedDir{name: name, path: filepath.Join(dir, name)})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].name) < strings.ToLower(out[j].name)
	})
	return out, nil
}

func (m *dirPickerModel) View() tea.View {
	// Leave alt screen on the final render so the terminal returns to the
	// normal buffer cleanly — mirrors AppModel's quitting behavior. Without
	// this the picker's last frame stays painted in the normal buffer and
	// reappears when the session TUI later leaves alt screen (very visible
	// in remote sessions, where the client terminal mirrors the PTY).
	if m.quitting {
		v := tea.NewView("")
		v.AltScreen = false
		v.MouseMode = tea.MouseModeNone
		return v
	}

	if m.popup.Items() == nil || (m.popup.Cursor() == 0 && len(m.popup.Items()) == 0) {
		m.rebuild()
	}

	content := m.popup.Render()
	if m.errMsg != "" {
		errLine := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("⚠ " + m.errMsg)
		content = strings.TrimRight(content, "\n") + "\n" + errLine
	}

	width, height := m.width, m.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	v := tea.NewView(lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center, content))
	v.AltScreen = true
	return v
}
