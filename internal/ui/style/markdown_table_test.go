package style

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

const wideTableMarkdown = "Intro paragraph.\n" +
	"| Possible cause | Common signs | Treatments a vet may recommend |\n" +
	"|---|---|---|\n" +
	"| **Yeast infection** | Red, itchy skin, licking, greasy buildup, sometimes a musty odor | Veterinary paw wipes, shampoo, or mousse containing **miconazole or ketoconazole**, often combined with **chlorhexidine**. More extensive or persistent infections sometimes need prescription oral antifungals. |\n" +
	"| **Bacterial infection / interdigital furuncle** | Tender red lump, swelling, pus or bloody drainage, limping | Antiseptic treatment such as veterinary chlorhexidine; some cases need prescription antibiotics. |\n" +
	"| - dash | 1. one | _em_ and `a \\| b` |\n" +
	"\nAfter the table.\n"

// TestToMarkdownFitsWideTables guards against wide tables being cut into
// broken fragments by the final wrap: every line of a fitted table must be
// within the width, and every table line must start and end with a border.
func TestToMarkdownFitsWideTables(t *testing.T) {
	SetTheme(DefaultTheme())
	for _, width := range []int{40, 80, 120} {
		out := xansi.Strip(ToMarkdown(wideTableMarkdown, width))
		if !strings.Contains(out, "Intro paragraph.") || !strings.Contains(out, "After the table.") {
			t.Fatalf("width %d: surrounding text lost:\n%s", width, out)
		}
		if strings.Contains(out, placeholderToken) {
			t.Fatalf("width %d: placeholder leaked:\n%s", width, out)
		}
		for _, line := range strings.Split(out, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Errorf("width %d: line is %d wide: %q", width, w, line)
			}
			r := []rune(line)
			if len(r) == 0 || !strings.ContainsRune("┌│├└", r[0]) {
				continue
			}
			if !strings.ContainsRune("┐│┤┘", r[len(r)-1]) {
				t.Errorf("width %d: table line is not closed: %q", width, line)
			}
		}
		if strings.Contains(out, "•") {
			t.Errorf("width %d: cell text rendered as a list:\n%s", width, out)
		}
		if width >= 80 && (!strings.Contains(out, "- dash") || !strings.Contains(out, "1. one") || strings.Contains(out, "\\")) {
			t.Errorf("width %d: block-like cell text changed:\n%s", width, out)
		}
	}
}

// TestToMarkdownKeepsNarrowTables checks that a table which fits is left
// exactly as herald draws it.
func TestToMarkdownKeepsNarrowTables(t *testing.T) {
	SetTheme(DefaultTheme())
	md := "| a | b |\n|---|---|\n| 1 | 2 |\n"
	if _, ok := fitMarkdownTables(GetMarkdownTypography(), []byte(md), 80); ok {
		t.Fatal("narrow table was re-rendered")
	}
}
