package agent

import (
	"errors"
	"strings"
	"testing"

	"charm.land/fantasy"
)

// TestExtractToolResultTextMedia checks that a media tool result never leaks
// its base64 payload into the display or the session log.
func TestExtractToolResultTextMedia(t *testing.T) {
	// A payload long enough that a stringify fallback would be obvious.
	payload := strings.Repeat("QUJDRA==", 500)

	tests := []struct {
		name      string
		result    fantasy.ToolResultContent
		want      string
		wantError bool
	}{
		{
			name: "media with summary uses the summary",
			result: fantasy.ToolResultContent{
				Result: fantasy.ToolResultOutputContentMedia{
					Data:      payload,
					MediaType: "image/png",
					Text:      "Read image shot.png (image/png, 120x80, 900 bytes)",
				},
			},
			want: "Read image shot.png (image/png, 120x80, 900 bytes)",
		},
		{
			name: "media without summary gets a placeholder",
			result: fantasy.ToolResultContent{
				Result: fantasy.ToolResultOutputContentMedia{
					Data:      "QUJD",
					MediaType: "image/jpeg",
				},
			},
			want: "[image/jpeg attachment, 4 base64 bytes]",
		},
		{
			name: "text is unchanged",
			result: fantasy.ToolResultContent{
				Result: fantasy.ToolResultOutputContentText{Text: "plain output"},
			},
			want: "plain output",
		},
		{
			name: "error is reported as an error",
			result: fantasy.ToolResultContent{
				Result: fantasy.ToolResultOutputContentError{Error: errors.New("boom")},
			},
			want:      "boom",
			wantError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, isErr := extractToolResultText(tc.result)
			if got != tc.want {
				t.Errorf("text = %q, want %q", got, tc.want)
			}
			if isErr != tc.wantError {
				t.Errorf("isError = %v, want %v", isErr, tc.wantError)
			}
			if strings.Contains(got, payload) {
				t.Error("the base64 payload leaked into the result text")
			}
		})
	}
}
