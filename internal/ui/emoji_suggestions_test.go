package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/ui/core"
	"github.com/mark3labs/kit/internal/ui/emoji"
)

func TestExtractColonPrefix(t *testing.T) {
	tests := []struct {
		line       string
		wantOK     bool
		wantPrefix string
		wantIdx    int
	}{
		{":ch", true, "ch", 0},
		{"hi :ch", true, "ch", 3},
		{"hi\t:Flag_ch", true, "flag_ch", 3},
		{"a :+1", true, "+1", 2},
		{":c", false, "", 0},         // too short
		{":)", false, "", 0},         // emoticon
		{"hi :smile:", false, "", 0}, // closed shortcode
		{"http://example", false, "", 0},
		{"at 12:30", false, "", 0},
		{"key:value", false, "", 0},
		{"hi :ch x", false, "", 0}, // word already ended
		{"no colon", false, "", 0},
		{"", false, "", 0},
	}
	for _, tt := range tests {
		ok, prefix, idx := ExtractColonPrefix(tt.line)
		if ok != tt.wantOK || prefix != tt.wantPrefix || idx != tt.wantIdx {
			t.Errorf("ExtractColonPrefix(%q) = (%v, %q, %d), want (%v, %q, %d)",
				tt.line, ok, prefix, idx, tt.wantOK, tt.wantPrefix, tt.wantIdx)
		}
	}
}

func TestGetEmojiSuggestions_Ranking(t *testing.T) {
	got := GetEmojiSuggestions("ch")
	if len(got) == 0 || len(got) > maxEmojiSuggestions {
		t.Fatalf("got %d suggestions, want 1..%d", len(got), maxEmojiSuggestions)
	}
	// Name-prefix matches rank above word-prefix matches such as "flag_chad".
	for _, m := range got {
		if !strings.HasPrefix(m.Name, "ch") {
			t.Errorf("suggestion %q does not start with \"ch\"; prefix matches must fill the list first", m.Name)
		}
	}

	exact := GetEmojiSuggestions("chains")
	if len(exact) == 0 || exact[0].Name != "chains" {
		t.Errorf("exact match must rank first, got %+v", exact)
	}

	flag := GetEmojiSuggestions("chad")
	if len(flag) == 0 || flag[0].Name != "flag_chad" {
		t.Errorf("word-prefix match flag_chad must rank first for \"chad\", got %+v", flag)
	}

	if dash := GetEmojiSuggestions("flag-chad"); len(dash) == 0 || dash[0].Name != "flag_chad" {
		t.Errorf("dash must match underscore, got %+v", dash)
	}
}

func TestEmojiTable_UniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, e := range emoji.All() {
		if e.Name == "" || e.Glyph == "" {
			t.Errorf("empty entry: %+v", e)
		}
		if seen[e.Name] {
			t.Errorf("duplicate shortcode %q", e.Name)
		}
		seen[e.Name] = true
	}
}

func TestInputComponent_EmojiCompletion(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyTab}} {
		c := newTestInput(nil)
		c.textarea.SetValue("first line\nhi :chain")
		c, _ = sendInputMsg(c, tea.KeyPressMsg{Code: 's', Text: "s"})

		if !c.showPopup || !c.emojiMode {
			t.Fatalf("expected emoji popup after typing \":chains\" (showPopup=%v emojiMode=%v)", c.showPopup, c.emojiMode)
		}
		if c.emojiSuggestions[0].Name != "chains" {
			t.Fatalf("first suggestion = %q, want chains", c.emojiSuggestions[0].Name)
		}

		c, cmd := sendInputMsg(c, key)
		if _, ok := runCmd(cmd).(core.SubmitMsg); ok || c.submitNext {
			t.Fatalf("%s must not submit", key.String())
		}
		want := "first line\nhi " + c.emojiSuggestions[0].Glyph + " "
		if got := c.textarea.Value(); got != want {
			t.Errorf("%s: value = %q, want %q", key.String(), got, want)
		}
		if c.showPopup || c.emojiMode {
			t.Errorf("%s: popup must close after completion", key.String())
		}
	}
}

func TestInputComponent_EmojiPopupIgnoresURLs(t *testing.T) {
	c := newTestInput(nil)
	c.textarea.SetValue("see https:/")
	c, _ = sendInputMsg(c, tea.KeyPressMsg{Code: '/', Text: "/"})
	if c.emojiMode || c.showPopup {
		t.Error("URL must not open the emoji popup")
	}
}
