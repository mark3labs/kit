package ui

import (
	"strings"
	"testing"
)

// TestRenderReadImageBody checks the compact card the read tool's image
// summary renders as. Before this renderer existed, an image summary fell
// through to the source-code path and drew an empty line-number gutter.
func TestRenderReadImageBody(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		wantAny  []string
		wantNone []string
		wantSkip bool // renderer must decline and fall through
	}{
		{
			name:    "plain image summary",
			result:  "Read image /tmp/a/card.png (image/png, 900x400, 7551 bytes)",
			wantAny: []string{"card.png", "900x400", "png", "7.4 KB"},
		},
		{
			name:   "resized image summary",
			result: "Read image /tmp/a/big.png (image/jpeg, 1568x1019, 1845188 bytes) [resized from 4000x2600, 17890728 bytes]",
			wantAny: []string{
				"big.png", "1568x1019", "jpeg", "1.8 MB",
				"resized from", "4000x2600", "17.1 MB",
			},
		},
		{
			name:     "numbered source falls through",
			result:   "1: package main\n2: \n3: func main() {}",
			wantSkip: true,
		},
		{
			name:     "unrelated text falls through",
			result:   "offset 400 exceeds file length (12 lines)",
			wantSkip: true,
		},
		{
			name:     "error text falls through",
			result:   "cannot read image 'a.png': image could not be decoded",
			wantSkip: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := renderReadImageBody(tc.result, 100)
			if tc.wantSkip {
				if got != "" {
					t.Errorf("renderReadImageBody() = %q, want an empty string so the caller falls through", got)
				}
				return
			}
			if got == "" {
				t.Fatal("renderReadImageBody() = \"\", want a rendered card")
			}
			for _, want := range tc.wantAny {
				if !strings.Contains(got, want) {
					t.Errorf("rendered card is missing %q; got:\n%s", want, got)
				}
			}
			for _, unwanted := range tc.wantNone {
				if strings.Contains(got, unwanted) {
					t.Errorf("rendered card contains %q; got:\n%s", unwanted, got)
				}
			}
			// The card must never show a line-number gutter.
			if strings.Contains(got, "  1 ") {
				t.Errorf("rendered card has a line-number gutter; got:\n%s", got)
			}
		})
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{7551, "7.4 KB"},
		{1 << 20, "1.0 MB"},
		{17890728, "17.1 MB"},
	}
	for _, tc := range tests {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestRenderReadBodyUsesImageCard checks the wiring: the read tool dispatcher
// must route an image summary to the card, not the code block.
func TestRenderReadBodyUsesImageCard(t *testing.T) {
	args := `{"path":"/tmp/a/card.png"}`
	result := "Read image /tmp/a/card.png (image/png, 900x400, 7551 bytes)"

	got := renderReadBody(args, result, 100, 50)
	if got == "" {
		t.Fatal("renderReadBody() = \"\", want the image card")
	}
	if !strings.Contains(got, "900x400") {
		t.Errorf("renderReadBody() did not use the image card; got:\n%s", got)
	}
}
