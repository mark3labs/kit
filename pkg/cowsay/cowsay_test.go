package cowsay

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// bubbleLines returns the text lines of the bubble in s: everything
// between the top border and the line before the cow's back.
func bubbleLines(s string) []string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.Contains(line, "^__^") {
			return lines[1 : i-1]
		}
	}
	return nil
}

func TestSaySingleLine(t *testing.T) {
	want := strings.Join([]string{
		" _______",
		"< hello >",
		" -------",
		"        \\   ^__^",
		"         \\  (oo)\\_______",
		"            (__)\\       )\\/\\",
		"                ||----w |",
		"                ||     ||",
		"",
	}, "\n")
	if got := Say("hello"); got != want {
		t.Errorf("Say() mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSayEmptyMessage(t *testing.T) {
	got := Say("")
	if !strings.Contains(got, "<  >") {
		t.Errorf("Say(\"\") should keep an empty bubble, got:\n%s", got)
	}
	if !strings.Contains(got, cow) {
		t.Error("Say(\"\") is missing the cow")
	}
}

func TestSayWrapsLongMessage(t *testing.T) {
	got := Say("the quick brown fox jumps over the lazy dog again and again")
	textLines := bubbleLines(got)
	if len(textLines) < 2 {
		t.Fatalf("expected wrapped bubble lines, got %d", len(textLines))
	}
	for _, line := range textLines {
		// Open and close marks plus one space of padding on each side.
		if runes := utf8.RuneCountInString(line); runes > maxLineWidth+4 {
			t.Errorf("bubble line too wide (%d runes): %q", runes, line)
		}
	}
	if !strings.HasPrefix(textLines[0], "/ ") || !strings.HasPrefix(textLines[len(textLines)-1], "\\ ") {
		t.Errorf("multi-line bubble should open with / and close with \\:\n%s", got)
	}
}

func TestSayHardBreaksLongWords(t *testing.T) {
	got := Say(strings.Repeat("m", maxLineWidth+10))
	textLines := bubbleLines(got)
	if len(textLines) < 2 {
		t.Fatalf("expected hard-broken bubble lines, got %d", len(textLines))
	}
	for i, line := range textLines {
		inner := strings.Trim(line, "<>\\/| ")
		if runes := utf8.RuneCountInString(inner); runes > maxLineWidth {
			t.Errorf("line %d exceeds maxLineWidth: %q (%d runes)", i, inner, runes)
		}
	}
	if !strings.Contains(got, cow) {
		t.Error("hard-broken bubble is missing the cow")
	}
}

func TestSayUnicode(t *testing.T) {
	got := Say("héllo wörld — ünicode 🐮 works")
	if !strings.Contains(got, "héllo wörld — ünicode 🐮 works") {
		t.Errorf("unicode message mangled:\n%s", got)
	}
}

func TestFirstRunes(t *testing.T) {
	if got := firstRunes("héllo", 3); got != "hél" {
		t.Errorf("firstRunes() = %q, want %q", got, "hél")
	}
	if got := firstRunes("🐮🐮🐮🐮", 2); utf8.RuneCountInString(got) != 2 {
		t.Errorf("firstRunes() = %q, want 2 runes", got)
	}
}
