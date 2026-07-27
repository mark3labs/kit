package style

import (
	"github.com/alecthomas/chroma/v2"

	"image/color"
	"math"
	"testing"
)

// rgb returns the 8-bit components of a color.
func rgb(c color.Color) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return float64(r >> 8), float64(g >> 8), float64(b >> 8)
}

// distance returns the Euclidean distance between two colors in RGB space.
// It is a crude perceptual proxy, but sufficient to catch the case this test
// exists for: two semantic slots drifting close enough to be confusable.
func distance(a, b color.Color) float64 {
	ar, ag, ab := rgb(a)
	br, bg, bb := rgb(b)
	return math.Sqrt((ar-br)*(ar-br) + (ag-bg)*(ag-bg) + (ab-bb)*(ab-bb))
}

// TestSemanticColorsAreDistinguishable guards the pairs whose confusion would
// actually mislead: an outcome must never look like a caution, and a failure
// must never look like the brand.
func TestSemanticColorsAreDistinguishable(t *testing.T) {
	const minDistance = 60

	theme := DefaultTheme()
	pairs := []struct {
		nameA, nameB string
		a, b         color.Color
	}{
		{"Success", "Warning", theme.Success, theme.Warning},
		{"Error", "Primary", theme.Error, theme.Primary},
		{"Error", "Warning", theme.Error, theme.Warning},
		{"Success", "Error", theme.Success, theme.Error},
	}

	for _, p := range pairs {
		if d := distance(p.a, p.b); d < minDistance {
			t.Errorf("%s and %s are too similar to tell apart (distance %.0f, want >= %d)",
				p.nameA, p.nameB, d, minDistance)
		}
	}
}

// TestSyntaxStyleFollowsTheme verifies code highlighting is derived from the
// active theme rather than a fixed external palette, and that switching
// themes actually rebuilds it.
func TestSyntaxStyleFollowsTheme(t *testing.T) {
	SetTheme(DefaultTheme())
	first := SyntaxStyle()

	if first == nil {
		t.Fatal("expected a syntax style")
	}
	// The keyword color must come from the theme, not from elsewhere.
	wantKeyword := hexOf(DefaultTheme().Markdown.Keyword)
	if got := first.Get(chromaKeyword()).Colour.String(); got != wantKeyword {
		t.Errorf("keyword color = %s, want the theme's %s", got, wantKeyword)
	}

	// A theme change must invalidate the cache.
	if err := ApplyThemeWithoutSave("nord"); err != nil {
		t.Fatalf("ApplyThemeWithoutSave(nord): %v", err)
	}
	second := SyntaxStyle()
	if second.Get(chromaKeyword()).Colour.String() == wantKeyword {
		t.Error("syntax style did not change after switching themes")
	}

	// Restore so later tests see the default palette.
	if err := ApplyThemeWithoutSave("kitt"); err != nil {
		t.Fatalf("ApplyThemeWithoutSave(kitt): %v", err)
	}
	if SyntaxStyle().Get(chromaKeyword()).Colour.String() != wantKeyword {
		t.Error("syntax style did not return to the default palette")
	}
}

// TestThemeFieldsAreWired guards against semantic slots that exist in config
// but drive no rendering. System and Tool were both settable and both dead.
func TestThemeFieldsAreWired(t *testing.T) {
	SetTheme(DefaultTheme())
	theme := GetTheme()

	if theme.System == nil {
		t.Error("Theme.System must have a value; it colors system notices")
	}
	if theme.Tool == nil {
		t.Error("Theme.Tool must have a value; it colors tool call names")
	}
	if theme.InputBg == nil {
		t.Error("Theme.InputBg must have a value; it fills the composer bar")
	}
}

// chromaKeyword returns the chroma token type for a keyword. It lives here so
// the test does not need a chroma import at the top of the file.
func chromaKeyword() chroma.TokenType { return chroma.Keyword }
