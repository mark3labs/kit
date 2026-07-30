package style

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

// saveStyleState snapshots the package-global styling state these tests mutate
// and returns a restore function, so a test never leaks capabilities or a theme
// into the ones that follow (this package's tests run non-parallel).
func saveStyleState(t *testing.T) {
	t.Helper()
	termCapsMu.Lock()
	caps := termCaps
	termCapsMu.Unlock()
	theme := currentTheme
	explicit := themeExplicitlySet
	gen := themeGeneration

	t.Cleanup(func() {
		termCapsMu.Lock()
		termCaps = caps
		termCapsMu.Unlock()
		currentTheme = theme
		themeExplicitlySet = explicit
		themeGeneration = gen
		styleCache = nil
		markdownTypographyCache = nil
		uiTypographyCache = nil
		syntaxStyleCache = nil
	})
}

func TestSetTerminalCapabilitiesOverridesDetection(t *testing.T) {
	saveStyleState(t)

	SetTerminalCapabilities(true, colorprofile.TrueColor)
	if !IsDarkBackground() {
		t.Fatal("IsDarkBackground() = false after setting dark background")
	}
	if !SupportsSmoothAnimation() {
		t.Fatal("SupportsSmoothAnimation() = false for a TrueColor profile")
	}

	SetTerminalCapabilities(false, colorprofile.Ascii)
	if IsDarkBackground() {
		t.Fatal("IsDarkBackground() = true after setting light background")
	}
	if SupportsSmoothAnimation() {
		t.Fatal("SupportsSmoothAnimation() = true for an Ascii profile")
	}
}

func TestSetTerminalCapabilitiesRebuildsDerivedTheme(t *testing.T) {
	saveStyleState(t)

	// Force a fresh, lazily-derived theme for a light terminal.
	currentTheme = nil
	themeExplicitlySet = false
	SetTerminalCapabilities(false, colorprofile.TrueColor)
	lightText := GetTheme().Text

	// Switching the terminal background must rebuild the derived theme so the
	// adaptive colours flip to the dark-mode branch.
	SetTerminalCapabilities(true, colorprofile.TrueColor)
	darkText := GetTheme().Text

	if colorsEqual(lightText, darkText) {
		t.Fatalf("derived theme Text did not change with background: %v", lightText)
	}
}

func TestSetTerminalCapabilitiesPreservesExplicitTheme(t *testing.T) {
	saveStyleState(t)

	custom := DefaultTheme()
	custom.Text = lipgloss.Color("#123456")
	SetTheme(custom)

	// An explicitly chosen theme must survive a capability change untouched.
	SetTerminalCapabilities(true, colorprofile.TrueColor)
	if got := GetTheme().Text; !colorsEqual(got, lipgloss.Color("#123456")) {
		t.Fatalf("explicit theme Text changed after SetTerminalCapabilities: got %v", got)
	}
}

func colorsEqual(a, b interface{ RGBA() (r, g, b, a uint32) }) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
