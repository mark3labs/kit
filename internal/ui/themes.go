package ui

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ThemeEntry is a named, loadable theme — either built-in or discovered from disk.
type ThemeEntry struct {
	Name   string // Display name (filename stem or preset name)
	Source string // "builtin" or absolute file path
	theme  Theme  // Resolved theme (lazy-loaded for file-based)
	loaded bool
}

// Theme returns the resolved ui.Theme, loading from disk on first access.
func (e *ThemeEntry) Theme() (Theme, error) {
	if e.loaded {
		return e.theme, nil
	}
	if e.Source == "builtin" {
		// Already set at registration time.
		return e.theme, nil
	}
	t, err := loadThemeFile(e.Source)
	if err != nil {
		return Theme{}, fmt.Errorf("loading theme %q: %w", e.Name, err)
	}
	e.theme = t
	e.loaded = true
	return e.theme, nil
}

// ---------------------------------------------------------------------------
// Built-in presets
// ---------------------------------------------------------------------------

// builtinThemes returns the set of themes shipped with Kit.
// makeTheme builds a full Theme from a compact palette spec. Fields left as
// zero color.Color inherit from the KITT default theme, keeping the preset
// definitions focused on what differs.
type presetColors struct {
	primary, secondary, success, warning, error_, info          [2]string // [light, dark]
	text, muted, veryMuted, background, border, mutedBorder     [2]string
	system, tool, accent, highlight                             [2]string
	mdKeyword, mdString, mdNumber, mdComment, mdHeading, mdLink [2]string
}

func makeTheme(p presetColors) Theme {
	ac := func(pair [2]string) color.Color { return AdaptiveColor(pair[0], pair[1]) }
	def := DefaultTheme()
	acOr := func(pair [2]string, fb color.Color) color.Color {
		if pair[0] == "" && pair[1] == "" {
			return fb
		}
		return ac(pair)
	}
	t := Theme{
		Primary:     ac(p.primary),
		Secondary:   acOr(p.secondary, ac(p.primary)),
		Success:     ac(p.success),
		Warning:     ac(p.warning),
		Error:       ac(p.error_),
		Info:        ac(p.info),
		Text:        ac(p.text),
		Muted:       acOr(p.muted, def.Muted),
		VeryMuted:   acOr(p.veryMuted, def.VeryMuted),
		Background:  ac(p.background),
		Border:      acOr(p.border, def.Border),
		MutedBorder: acOr(p.mutedBorder, def.MutedBorder),
		System:      acOr(p.system, ac(p.info)),
		Tool:        acOr(p.tool, ac(p.warning)),
		Accent:      acOr(p.accent, ac(p.primary)),
		Highlight:   acOr(p.highlight, def.Highlight),
	}
	// Derive diff/code backgrounds from the base background.
	t.DiffInsertBg = def.DiffInsertBg
	t.DiffDeleteBg = def.DiffDeleteBg
	t.DiffEqualBg = def.DiffEqualBg
	t.DiffMissingBg = def.DiffMissingBg
	t.CodeBg = def.CodeBg
	t.GutterBg = def.GutterBg
	t.WriteBg = def.WriteBg
	// Markdown colors.
	t.Markdown = MarkdownThemeColors{
		Text:    t.Text,
		Muted:   t.Muted,
		Heading: acOr(p.mdHeading, t.Primary),
		Emph:    t.Warning,
		Strong:  t.Text,
		Link:    acOr(p.mdLink, t.Info),
		Code:    t.Muted,
		Error:   t.Error,
		Keyword: acOr(p.mdKeyword, t.Primary),
		String:  acOr(p.mdString, t.Success),
		Number:  acOr(p.mdNumber, t.Warning),
		Comment: acOr(p.mdComment, t.VeryMuted),
	}
	return t
}

// builtinThemes returns the set of themes shipped with Kit.
// Inspired by the OpenCode theme collection.
func builtinThemes() map[string]Theme {
	return map[string]Theme{
		"kitt": DefaultTheme(),

		"catppuccin": makeTheme(presetColors{
			primary: [2]string{"#8839ef", "#cba6f7"}, secondary: [2]string{"#04a5e5", "#89dceb"},
			success: [2]string{"#40a02b", "#a6e3a1"}, warning: [2]string{"#df8e1d", "#f9e2af"},
			error_: [2]string{"#d20f39", "#f38ba8"}, info: [2]string{"#1e66f5", "#89b4fa"},
			text: [2]string{"#4c4f69", "#cdd6f4"}, muted: [2]string{"#6c6f85", "#a6adc8"},
			veryMuted: [2]string{"#9ca0b0", "#6c7086"}, background: [2]string{"#eff1f5", "#1e1e2e"},
			border: [2]string{"#acb0be", "#585b70"}, mutedBorder: [2]string{"#ccd0da", "#313244"},
			system: [2]string{"#179299", "#94e2d5"}, tool: [2]string{"#fe640b", "#fab387"},
			accent: [2]string{"#ea76cb", "#f5c2e7"}, highlight: [2]string{"#e6e9ef", "#181825"},
			mdKeyword: [2]string{"#8839ef", "#cba6f7"}, mdString: [2]string{"#40a02b", "#a6e3a1"},
			mdNumber: [2]string{"#fe640b", "#fab387"}, mdComment: [2]string{"#9ca0b0", "#6c7086"},
		}),

		"dracula": makeTheme(presetColors{
			primary: [2]string{"#7c6bf5", "#bd93f9"}, secondary: [2]string{"#d16090", "#ff79c6"},
			success: [2]string{"#2fbf71", "#50fa7b"}, warning: [2]string{"#f7a14d", "#ffb86c"},
			error_: [2]string{"#d9536f", "#ff5555"}, info: [2]string{"#1d7fc5", "#8be9fd"},
			text: [2]string{"#1f1f2f", "#f8f8f2"}, background: [2]string{"#f8f8f2", "#1d1e28"},
			accent:    [2]string{"#d16090", "#ff79c6"},
			mdKeyword: [2]string{"#7c6bf5", "#bd93f9"}, mdString: [2]string{"#2fbf71", "#50fa7b"},
			mdComment: [2]string{"#6272a4", "#6272a4"},
		}),

		"tokyonight": makeTheme(presetColors{
			primary: [2]string{"#2e7de9", "#7aa2f7"}, secondary: [2]string{"#b15c00", "#ff9e64"},
			success: [2]string{"#587539", "#9ece6a"}, warning: [2]string{"#8c6c3e", "#e0af68"},
			error_: [2]string{"#c94060", "#f7768e"}, info: [2]string{"#007197", "#7dcfff"},
			text: [2]string{"#273153", "#c0caf5"}, background: [2]string{"#e1e2e7", "#1a1b26"},
			mdKeyword: [2]string{"#2e7de9", "#7aa2f7"}, mdString: [2]string{"#587539", "#9ece6a"},
			mdComment: [2]string{"#848cb5", "#565f89"},
		}),

		"nord": makeTheme(presetColors{
			primary: [2]string{"#5e81ac", "#88c0d0"}, secondary: [2]string{"#bf616a", "#d57780"},
			success: [2]string{"#8fbcbb", "#a3be8c"}, warning: [2]string{"#d08770", "#d08770"},
			error_: [2]string{"#bf616a", "#bf616a"}, info: [2]string{"#81a1c1", "#81a1c1"},
			text: [2]string{"#2e3440", "#e5e9f0"}, background: [2]string{"#eceff4", "#2e3440"},
			mdKeyword: [2]string{"#5e81ac", "#81a1c1"}, mdString: [2]string{"#8fbcbb", "#a3be8c"},
			mdComment: [2]string{"#616e88", "#616e88"},
		}),

		"gruvbox": makeTheme(presetColors{
			primary: [2]string{"#076678", "#83a598"}, secondary: [2]string{"#9d0006", "#fb4934"},
			success: [2]string{"#79740e", "#b8bb26"}, warning: [2]string{"#b57614", "#fabd2f"},
			error_: [2]string{"#9d0006", "#fb4934"}, info: [2]string{"#8f3f71", "#d3869b"},
			text: [2]string{"#3c3836", "#ebdbb2"}, background: [2]string{"#fbf1c7", "#282828"},
			mdKeyword: [2]string{"#9d0006", "#fb4934"}, mdString: [2]string{"#79740e", "#b8bb26"},
			mdComment: [2]string{"#928374", "#928374"},
		}),

		"monokai": makeTheme(presetColors{
			primary: [2]string{"#bf7bff", "#ae81ff"}, secondary: [2]string{"#d9487c", "#f92672"},
			success: [2]string{"#4fb54b", "#a6e22e"}, warning: [2]string{"#f1a948", "#fd971f"},
			error_: [2]string{"#e54b4b", "#f92672"}, info: [2]string{"#2d9ad7", "#66d9ef"},
			text: [2]string{"#292318", "#f8f8f2"}, background: [2]string{"#fdf8ec", "#272822"},
			mdKeyword: [2]string{"#d9487c", "#f92672"}, mdString: [2]string{"#4fb54b", "#a6e22e"},
			mdComment: [2]string{"#888888", "#75715e"},
		}),

		"solarized": makeTheme(presetColors{
			primary: [2]string{"#268bd2", "#6c71c4"}, secondary: [2]string{"#d33682", "#d33682"},
			success: [2]string{"#859900", "#859900"}, warning: [2]string{"#b58900", "#b58900"},
			error_: [2]string{"#dc322f", "#dc322f"}, info: [2]string{"#2aa198", "#2aa198"},
			text: [2]string{"#586e75", "#93a1a1"}, background: [2]string{"#fdf6e3", "#002b36"},
			mdKeyword: [2]string{"#268bd2", "#6c71c4"}, mdString: [2]string{"#859900", "#859900"},
			mdComment: [2]string{"#93a1a1", "#586e75"},
		}),

		"github": makeTheme(presetColors{
			primary: [2]string{"#0969da", "#58a6ff"}, secondary: [2]string{"#1b7c83", "#39c5cf"},
			success: [2]string{"#1a7f37", "#3fb950"}, warning: [2]string{"#9a6700", "#e3b341"},
			error_: [2]string{"#cf222e", "#f85149"}, info: [2]string{"#bc4c00", "#d29922"},
			text: [2]string{"#24292f", "#c9d1d9"}, background: [2]string{"#ffffff", "#0d1117"},
			mdKeyword: [2]string{"#0969da", "#58a6ff"}, mdString: [2]string{"#1a7f37", "#3fb950"},
			mdComment: [2]string{"#6e7781", "#8b949e"},
		}),

		"one-dark": makeTheme(presetColors{
			primary: [2]string{"#4078f2", "#61afef"}, secondary: [2]string{"#0184bc", "#56b6c2"},
			success: [2]string{"#50a14f", "#98c379"}, warning: [2]string{"#c18401", "#e5c07b"},
			error_: [2]string{"#e45649", "#e06c75"}, info: [2]string{"#986801", "#d19a66"},
			text: [2]string{"#383a42", "#abb2bf"}, background: [2]string{"#fafafa", "#282c34"},
			mdKeyword: [2]string{"#a626a4", "#c678dd"}, mdString: [2]string{"#50a14f", "#98c379"},
			mdComment: [2]string{"#a0a1a7", "#5c6370"},
		}),

		"rose-pine": makeTheme(presetColors{
			primary: [2]string{"#31748f", "#9ccfd8"}, secondary: [2]string{"#d7827e", "#ebbcba"},
			success: [2]string{"#286983", "#31748f"}, warning: [2]string{"#ea9d34", "#f6c177"},
			error_: [2]string{"#b4637a", "#eb6f92"}, info: [2]string{"#56949f", "#9ccfd8"},
			text: [2]string{"#575279", "#e0def4"}, background: [2]string{"#faf4ed", "#191724"},
			mdKeyword: [2]string{"#31748f", "#9ccfd8"}, mdString: [2]string{"#ea9d34", "#f6c177"},
			mdComment: [2]string{"#9893a5", "#6e6a86"},
		}),

		"ayu": makeTheme(presetColors{
			primary: [2]string{"#4aa8c8", "#3fb7e3"}, secondary: [2]string{"#ef7d71", "#f2856f"},
			success: [2]string{"#5fb978", "#78d05c"}, warning: [2]string{"#ea9f41", "#e4a75c"},
			error_: [2]string{"#e6656a", "#f58572"}, info: [2]string{"#2f9bce", "#66c6f1"},
			text: [2]string{"#4f5964", "#d6dae0"}, background: [2]string{"#fdfaf4", "#0f1419"},
			mdKeyword: [2]string{"#4aa8c8", "#3fb7e3"}, mdString: [2]string{"#5fb978", "#78d05c"},
			mdComment: [2]string{"#abb0b6", "#5c6773"},
		}),

		"material": makeTheme(presetColors{
			primary: [2]string{"#6182b8", "#82aaff"}, secondary: [2]string{"#39adb5", "#89ddff"},
			success: [2]string{"#91b859", "#c3e88d"}, warning: [2]string{"#ffb300", "#ffcb6b"},
			error_: [2]string{"#e53935", "#f07178"}, info: [2]string{"#f4511e", "#ffcb6b"},
			text: [2]string{"#263238", "#eeffff"}, background: [2]string{"#fafafa", "#263238"},
			mdKeyword: [2]string{"#6182b8", "#82aaff"}, mdString: [2]string{"#91b859", "#c3e88d"},
			mdComment: [2]string{"#aabfc5", "#546e7a"},
		}),

		"everforest": makeTheme(presetColors{
			primary: [2]string{"#8da101", "#a7c080"}, secondary: [2]string{"#df69ba", "#d699b6"},
			success: [2]string{"#8da101", "#a7c080"}, warning: [2]string{"#f57d26", "#e69875"},
			error_: [2]string{"#f85552", "#e67e80"}, info: [2]string{"#35a77c", "#83c092"},
			text: [2]string{"#5c6a72", "#d3c6aa"}, background: [2]string{"#fdf6e3", "#2d353b"},
			mdKeyword: [2]string{"#8da101", "#a7c080"}, mdString: [2]string{"#35a77c", "#83c092"},
			mdComment: [2]string{"#939b84", "#859289"},
		}),

		"kanagawa": makeTheme(presetColors{
			primary: [2]string{"#2D4F67", "#7E9CD8"}, secondary: [2]string{"#D27E99", "#D27E99"},
			success: [2]string{"#98BB6C", "#98BB6C"}, warning: [2]string{"#D7A657", "#D7A657"},
			error_: [2]string{"#E82424", "#E82424"}, info: [2]string{"#76946A", "#76946A"},
			text: [2]string{"#54433A", "#DCD7BA"}, background: [2]string{"#F2E9DE", "#1F1F28"},
			mdKeyword: [2]string{"#2D4F67", "#7E9CD8"}, mdString: [2]string{"#98BB6C", "#98BB6C"},
			mdComment: [2]string{"#A09D98", "#727169"},
		}),

		"amoled": makeTheme(presetColors{
			primary: [2]string{"#6200ff", "#b388ff"}, secondary: [2]string{"#ff0080", "#ff4081"},
			success: [2]string{"#00e676", "#00ff88"}, warning: [2]string{"#ffab00", "#ffea00"},
			error_: [2]string{"#ff1744", "#ff1744"}, info: [2]string{"#00b0ff", "#18ffff"},
			text: [2]string{"#0a0a0a", "#ffffff"}, background: [2]string{"#f0f0f0", "#000000"},
			mdKeyword: [2]string{"#6200ff", "#b388ff"}, mdString: [2]string{"#00e676", "#00ff88"},
			mdComment: [2]string{"#757575", "#424242"},
		}),

		"synthwave": makeTheme(presetColors{
			primary: [2]string{"#00bcd4", "#36f9f6"}, secondary: [2]string{"#9c27b0", "#b084eb"},
			success: [2]string{"#4caf50", "#72f1b8"}, warning: [2]string{"#ff9800", "#fede5d"},
			error_: [2]string{"#f44336", "#fe4450"}, info: [2]string{"#ff5722", "#ff8b39"},
			text: [2]string{"#262335", "#ffffff"}, background: [2]string{"#fafafa", "#262335"},
			mdKeyword: [2]string{"#9c27b0", "#b084eb"}, mdString: [2]string{"#4caf50", "#72f1b8"},
			mdComment: [2]string{"#848bbd", "#848bbd"},
		}),

		"vesper": makeTheme(presetColors{
			primary: [2]string{"#FFC799", "#FFC799"}, secondary: [2]string{"#B30000", "#FF8080"},
			success: [2]string{"#99FFE4", "#99FFE4"}, warning: [2]string{"#FFC799", "#FFC799"},
			error_: [2]string{"#FF8080", "#FF8080"}, info: [2]string{"#FFC799", "#FFC799"},
			text: [2]string{"#1a1a1a", "#FFF"}, background: [2]string{"#F0F0F0", "#101010"},
			mdKeyword: [2]string{"#FFC799", "#FFC799"}, mdString: [2]string{"#99FFE4", "#99FFE4"},
			mdComment: [2]string{"#7a7a7a", "#505050"},
		}),

		"flexoki": makeTheme(presetColors{
			primary: [2]string{"#205EA6", "#DA702C"}, secondary: [2]string{"#BC5215", "#8B7EC8"},
			success: [2]string{"#66800B", "#879A39"}, warning: [2]string{"#BC5215", "#DA702C"},
			error_: [2]string{"#AF3029", "#D14D41"}, info: [2]string{"#24837B", "#3AA99F"},
			text: [2]string{"#100F0F", "#CECDC3"}, background: [2]string{"#FFFCF0", "#100F0F"},
			mdKeyword: [2]string{"#205EA6", "#DA702C"}, mdString: [2]string{"#66800B", "#879A39"},
			mdComment: [2]string{"#878580", "#878580"},
		}),

		"matrix": makeTheme(presetColors{
			primary: [2]string{"#1cc24b", "#2eff6a"}, secondary: [2]string{"#c770ff", "#c770ff"},
			success: [2]string{"#1cc24b", "#62ff94"}, warning: [2]string{"#e6ff57", "#e6ff57"},
			error_: [2]string{"#ff4b4b", "#ff4b4b"}, info: [2]string{"#30b3ff", "#30b3ff"},
			text: [2]string{"#203022", "#62ff94"}, background: [2]string{"#eef3ea", "#0a0e0a"},
			mdKeyword: [2]string{"#1cc24b", "#2eff6a"}, mdString: [2]string{"#1cc24b", "#62ff94"},
			mdComment: [2]string{"#5a7a5e", "#3a5a3e"},
		}),

		"vercel": makeTheme(presetColors{
			primary: [2]string{"#0070F3", "#0070F3"}, secondary: [2]string{"#8E4EC6", "#8E4EC6"},
			success: [2]string{"#388E3C", "#46A758"}, warning: [2]string{"#FF9500", "#FFB224"},
			error_: [2]string{"#DC3545", "#E5484D"}, info: [2]string{"#0070F3", "#52A8FF"},
			text: [2]string{"#171717", "#EDEDED"}, background: [2]string{"#FFFFFF", "#000000"},
			mdKeyword: [2]string{"#0070F3", "#0070F3"}, mdString: [2]string{"#388E3C", "#46A758"},
			mdComment: [2]string{"#6B6B6B", "#666666"},
		}),

		"zenburn": makeTheme(presetColors{
			primary: [2]string{"#5f7f8f", "#8cd0d3"}, secondary: [2]string{"#5f8f8f", "#93e0e3"},
			success: [2]string{"#5f8f5f", "#7f9f7f"}, warning: [2]string{"#8f8f5f", "#f0dfaf"},
			error_: [2]string{"#8f5f5f", "#cc9393"}, info: [2]string{"#8f7f5f", "#dfaf8f"},
			text: [2]string{"#3f3f3f", "#dcdccc"}, background: [2]string{"#ffffef", "#3f3f3f"},
			mdKeyword: [2]string{"#5f7f8f", "#8cd0d3"}, mdString: [2]string{"#5f8f5f", "#cc9393"},
			mdComment: [2]string{"#7f7f7f", "#7f9f7f"},
		}),
	}
}

// ---------------------------------------------------------------------------
// Theme registry (global)
// ---------------------------------------------------------------------------

var themeRegistry []ThemeEntry

// initThemeRegistry populates the registry from built-ins, user themes, and
// project-local themes. Later sources override earlier ones with the same name:
//  1. Built-in presets
//  2. User themes   (~/.config/kit/themes/)
//  3. Project-local (.kit/themes/ in the working directory)
func initThemeRegistry() {
	themeRegistry = nil

	// 1. Built-in presets.
	for name, t := range builtinThemes() {
		themeRegistry = append(themeRegistry, ThemeEntry{
			Name:   name,
			Source: "builtin",
			theme:  t,
			loaded: true,
		})
	}

	// 2. User themes from ~/.config/kit/themes/
	scanThemesDir(userThemesDir())

	// 3. Project-local themes from .kit/themes/
	scanThemesDir(projectThemesDir())

	sortRegistry()
}

// scanThemesDir adds all .yml/.yaml/.json theme files from dir to the registry.
// Files override any existing entry with the same stem name.
func scanThemesDir(dir string) {
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" && ext != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		removeFromRegistry(name)
		themeRegistry = append(themeRegistry, ThemeEntry{
			Name:   name,
			Source: filepath.Join(dir, entry.Name()),
		})
	}
}

func sortRegistry() {
	sort.Slice(themeRegistry, func(i, j int) bool {
		return themeRegistry[i].Name < themeRegistry[j].Name
	})
}

func removeFromRegistry(name string) {
	for i := range themeRegistry {
		if themeRegistry[i].Name == name {
			themeRegistry = append(themeRegistry[:i], themeRegistry[i+1:]...)
			return
		}
	}
}

// userThemesDir returns ~/.config/kit/themes, creating it if needed.
func userThemesDir() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(cfgDir, "kit", "themes")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// projectThemesDir returns .kit/themes/ relative to the working directory.
// Returns "" if the directory doesn't exist (does NOT create it).
func projectThemesDir() string {
	dir := filepath.Join(".kit", "themes")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return ""
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	return abs
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ListThemes returns the names of all available themes (built-in + user).
func ListThemes() []string {
	if themeRegistry == nil {
		initThemeRegistry()
	}
	names := make([]string, len(themeRegistry))
	for i := range themeRegistry {
		names[i] = themeRegistry[i].Name
	}
	return names
}

// LoadThemeByName looks up a theme by name, loads it if needed, and returns it.
func LoadThemeByName(name string) (Theme, error) {
	if themeRegistry == nil {
		initThemeRegistry()
	}
	for i := range themeRegistry {
		if themeRegistry[i].Name == name {
			return themeRegistry[i].Theme()
		}
	}
	return Theme{}, fmt.Errorf("theme %q not found", name)
}

// ApplyTheme loads a theme by name and sets it as the active global theme.
func ApplyTheme(name string) error {
	t, err := LoadThemeByName(name)
	if err != nil {
		return err
	}
	SetTheme(t)
	return nil
}

// RefreshThemeRegistry re-scans the themes directory. Call after the user
// drops a new file into ~/.config/kit/themes/.
func RefreshThemeRegistry() {
	initThemeRegistry()
}

// ActiveThemeName returns the name of the currently active theme by comparing
// against known entries. Returns "custom" if no match is found.
func ActiveThemeName() string {
	if themeRegistry == nil {
		initThemeRegistry()
	}
	current := GetTheme()
	for _, e := range themeRegistry {
		if !e.loaded {
			continue
		}
		if e.theme.Primary == current.Primary &&
			e.theme.Secondary == current.Secondary &&
			e.theme.Error == current.Error &&
			e.theme.Text == current.Text {
			return e.Name
		}
	}
	return "custom"
}

// ---------------------------------------------------------------------------
// File loading
// ---------------------------------------------------------------------------

// themeFileConfig mirrors config.Theme for unmarshaling theme files.
// Uses the same adaptive color structure.
type themeFileConfig struct {
	Primary     adaptiveColorPair `json:"primary,omitzero" yaml:"primary,omitempty"`
	Secondary   adaptiveColorPair `json:"secondary,omitzero" yaml:"secondary,omitempty"`
	Success     adaptiveColorPair `json:"success,omitzero" yaml:"success,omitempty"`
	Warning     adaptiveColorPair `json:"warning,omitzero" yaml:"warning,omitempty"`
	Error       adaptiveColorPair `json:"error,omitzero" yaml:"error,omitempty"`
	Info        adaptiveColorPair `json:"info,omitzero" yaml:"info,omitempty"`
	Text        adaptiveColorPair `json:"text,omitzero" yaml:"text,omitempty"`
	Muted       adaptiveColorPair `json:"muted,omitzero" yaml:"muted,omitempty"`
	VeryMuted   adaptiveColorPair `json:"very-muted,omitzero" yaml:"very-muted,omitempty"`
	Background  adaptiveColorPair `json:"background,omitzero" yaml:"background,omitempty"`
	Border      adaptiveColorPair `json:"border,omitzero" yaml:"border,omitempty"`
	MutedBorder adaptiveColorPair `json:"muted-border,omitzero" yaml:"muted-border,omitempty"`
	System      adaptiveColorPair `json:"system,omitzero" yaml:"system,omitempty"`
	Tool        adaptiveColorPair `json:"tool,omitzero" yaml:"tool,omitempty"`
	Accent      adaptiveColorPair `json:"accent,omitzero" yaml:"accent,omitempty"`
	Highlight   adaptiveColorPair `json:"highlight,omitzero" yaml:"highlight,omitempty"`

	DiffInsertBg  adaptiveColorPair `json:"diff-insert-bg,omitzero" yaml:"diff-insert-bg,omitempty"`
	DiffDeleteBg  adaptiveColorPair `json:"diff-delete-bg,omitzero" yaml:"diff-delete-bg,omitempty"`
	DiffEqualBg   adaptiveColorPair `json:"diff-equal-bg,omitzero" yaml:"diff-equal-bg,omitempty"`
	DiffMissingBg adaptiveColorPair `json:"diff-missing-bg,omitzero" yaml:"diff-missing-bg,omitempty"`
	CodeBg        adaptiveColorPair `json:"code-bg,omitzero" yaml:"code-bg,omitempty"`
	GutterBg      adaptiveColorPair `json:"gutter-bg,omitzero" yaml:"gutter-bg,omitempty"`
	WriteBg       adaptiveColorPair `json:"write-bg,omitzero" yaml:"write-bg,omitempty"`

	Markdown struct {
		Text    adaptiveColorPair `json:"text,omitzero" yaml:"text,omitempty"`
		Muted   adaptiveColorPair `json:"muted,omitzero" yaml:"muted,omitempty"`
		Heading adaptiveColorPair `json:"heading,omitzero" yaml:"heading,omitempty"`
		Emph    adaptiveColorPair `json:"emph,omitzero" yaml:"emph,omitempty"`
		Strong  adaptiveColorPair `json:"strong,omitzero" yaml:"strong,omitempty"`
		Link    adaptiveColorPair `json:"link,omitzero" yaml:"link,omitempty"`
		Code    adaptiveColorPair `json:"code,omitzero" yaml:"code,omitempty"`
		Error   adaptiveColorPair `json:"error,omitzero" yaml:"error,omitempty"`
		Keyword adaptiveColorPair `json:"keyword,omitzero" yaml:"keyword,omitempty"`
		String  adaptiveColorPair `json:"string,omitzero" yaml:"string,omitempty"`
		Number  adaptiveColorPair `json:"number,omitzero" yaml:"number,omitempty"`
		Comment adaptiveColorPair `json:"comment,omitzero" yaml:"comment,omitempty"`
	} `json:"markdown,omitzero" yaml:"markdown,omitempty"`
}

type adaptiveColorPair struct {
	Light string `json:"light,omitempty" yaml:"light,omitempty"`
	Dark  string `json:"dark,omitempty" yaml:"dark,omitempty"`
}

// resolve converts an adaptiveColorPair to a resolved color.Color,
// falling back to fallback when both Light and Dark are empty.
func (a adaptiveColorPair) resolve(fallback color.Color) color.Color {
	if a.Light == "" && a.Dark == "" {
		return fallback
	}
	return AdaptiveColor(a.Light, a.Dark)
}

func loadThemeFile(path string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, err
	}

	var cfg themeFileConfig
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		err = json.Unmarshal(data, &cfg)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &cfg)
	default:
		return Theme{}, fmt.Errorf("unsupported theme file format: %s", ext)
	}
	if err != nil {
		return Theme{}, err
	}

	return fileConfigToTheme(cfg), nil
}

func fileConfigToTheme(cfg themeFileConfig) Theme {
	def := DefaultTheme()
	return Theme{
		Primary:     cfg.Primary.resolve(def.Primary),
		Secondary:   cfg.Secondary.resolve(def.Secondary),
		Success:     cfg.Success.resolve(def.Success),
		Warning:     cfg.Warning.resolve(def.Warning),
		Error:       cfg.Error.resolve(def.Error),
		Info:        cfg.Info.resolve(def.Info),
		Text:        cfg.Text.resolve(def.Text),
		Muted:       cfg.Muted.resolve(def.Muted),
		VeryMuted:   cfg.VeryMuted.resolve(def.VeryMuted),
		Background:  cfg.Background.resolve(def.Background),
		Border:      cfg.Border.resolve(def.Border),
		MutedBorder: cfg.MutedBorder.resolve(def.MutedBorder),
		System:      cfg.System.resolve(def.System),
		Tool:        cfg.Tool.resolve(def.Tool),
		Accent:      cfg.Accent.resolve(def.Accent),
		Highlight:   cfg.Highlight.resolve(def.Highlight),

		DiffInsertBg:  cfg.DiffInsertBg.resolve(def.DiffInsertBg),
		DiffDeleteBg:  cfg.DiffDeleteBg.resolve(def.DiffDeleteBg),
		DiffEqualBg:   cfg.DiffEqualBg.resolve(def.DiffEqualBg),
		DiffMissingBg: cfg.DiffMissingBg.resolve(def.DiffMissingBg),
		CodeBg:        cfg.CodeBg.resolve(def.CodeBg),
		GutterBg:      cfg.GutterBg.resolve(def.GutterBg),
		WriteBg:       cfg.WriteBg.resolve(def.WriteBg),

		Markdown: MarkdownThemeColors{
			Text:    cfg.Markdown.Text.resolve(def.Markdown.Text),
			Muted:   cfg.Markdown.Muted.resolve(def.Markdown.Muted),
			Heading: cfg.Markdown.Heading.resolve(def.Markdown.Heading),
			Emph:    cfg.Markdown.Emph.resolve(def.Markdown.Emph),
			Strong:  cfg.Markdown.Strong.resolve(def.Markdown.Strong),
			Link:    cfg.Markdown.Link.resolve(def.Markdown.Link),
			Code:    cfg.Markdown.Code.resolve(def.Markdown.Code),
			Error:   cfg.Markdown.Error.resolve(def.Markdown.Error),
			Keyword: cfg.Markdown.Keyword.resolve(def.Markdown.Keyword),
			String:  cfg.Markdown.String.resolve(def.Markdown.String),
			Number:  cfg.Markdown.Number.resolve(def.Markdown.Number),
			Comment: cfg.Markdown.Comment.resolve(def.Markdown.Comment),
		},
	}
}
