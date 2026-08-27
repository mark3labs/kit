package cmd

import "testing"

// TestSuppressChrome documents the contract that both --quiet and --json keep
// decorative output (startup banner, spinner, warnings) off stdout. Without
// this, `kit "..." --json | jq` fails on the leading "Model loaded: ..." block.
func TestSuppressChrome(t *testing.T) {
	origQuiet, origJSON := quietFlag, jsonFlag
	t.Cleanup(func() {
		quietFlag, jsonFlag = origQuiet, origJSON
	})

	tests := []struct {
		name  string
		quiet bool
		json  bool
		want  bool
	}{
		{name: "interactive", quiet: false, json: false, want: false},
		{name: "quiet", quiet: true, json: false, want: true},
		{name: "json", quiet: false, json: true, want: true},
		{name: "quiet and json", quiet: true, json: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quietFlag, jsonFlag = tt.quiet, tt.json
			if got := suppressChrome(); got != tt.want {
				t.Errorf("suppressChrome() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBuildAppOptionsQuietInJSONMode verifies that --json propagates the quiet
// flag into the app layer, which routes unlevelled extension prints to stderr
// instead of stdout.
func TestBuildAppOptionsQuietInJSONMode(t *testing.T) {
	origQuiet, origJSON := quietFlag, jsonFlag
	t.Cleanup(func() {
		quietFlag, jsonFlag = origQuiet, origJSON
	})

	quietFlag, jsonFlag = false, true
	opts := BuildAppOptions(nil, "test-model", nil, nil)
	if !opts.Quiet {
		t.Error("BuildAppOptions().Quiet = false in --json mode, want true")
	}
}
