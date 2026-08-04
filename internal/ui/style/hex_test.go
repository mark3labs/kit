package style

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestHexOf(t *testing.T) {
	tests := []struct {
		name string
		in   color.Color
		want string
	}{
		{"nil is unset not black", nil, ""},
		{"lipgloss hex round-trips", lipgloss.Color("#89b4fa"), "#89b4fa"},
		{"black", lipgloss.Color("#000000"), "#000000"},
		{"white", lipgloss.Color("#ffffff"), "#ffffff"},
		{"rgba", color.RGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}, "#123456"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := HexOf(tc.in); got != tc.want {
				t.Fatalf("HexOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every slot the extension API exposes must yield a usable hex string, or an
// extension painting with ctx.GetTheme() silently loses that color.
func TestHexOfCoversDefaultTheme(t *testing.T) {
	th := DefaultTheme()
	slots := map[string]color.Color{
		"Primary": th.Primary, "Secondary": th.Secondary,
		"Success": th.Success, "Warning": th.Warning,
		"Error": th.Error, "Info": th.Info,
		"Text": th.Text, "Muted": th.Muted,
		"VeryMuted": th.VeryMuted, "Background": th.Background,
		"Border": th.Border, "MutedBorder": th.MutedBorder,
		"System": th.System, "Tool": th.Tool,
		"Accent": th.Accent, "Highlight": th.Highlight,
		"MdHeading": th.Markdown.Heading, "MdLink": th.Markdown.Link,
		"MdKeyword": th.Markdown.Keyword, "MdString": th.Markdown.String,
		"MdNumber": th.Markdown.Number, "MdComment": th.Markdown.Comment,
	}
	for name, c := range slots {
		hex := HexOf(c)
		if len(hex) != 7 || hex[0] != '#' {
			t.Errorf("%s: HexOf produced %q, want a #rrggbb string", name, hex)
		}
	}
}

func TestCurrentThemeNameTracksApply(t *testing.T) {
	prev := CurrentThemeName()
	t.Cleanup(func() { activeThemeName = prev })

	if err := ApplyThemeWithoutSave("dracula"); err != nil {
		t.Fatalf("ApplyThemeWithoutSave: %v", err)
	}
	if got := CurrentThemeName(); got != "dracula" {
		t.Fatalf("CurrentThemeName = %q, want %q", got, "dracula")
	}

	// A failed apply must not rename the still-active theme.
	if err := ApplyThemeWithoutSave("no-such-theme"); err == nil {
		t.Fatal("expected an error for an unknown theme")
	}
	if got := CurrentThemeName(); got != "dracula" {
		t.Fatalf("CurrentThemeName after failed apply = %q, want %q", got, "dracula")
	}
}
