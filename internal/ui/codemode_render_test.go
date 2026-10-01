package ui

import (
	"encoding/json"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

func TestRenderCodeModeBody(t *testing.T) {
	code := "const a = await tools.read({path: \"x\"})\nreturn a"
	args, _ := json.Marshal(map[string]string{"code": code})
	result := "Script completed in 12ms.\nTool calls: 1 — read ×1\n\nOutput:\nhello"

	out := xansi.Strip(renderToolBody("codemode", string(args), result, 100))
	for _, want := range []string{"const a = await tools.read", "return a", "Script completed", "hello"} {
		if !strings.Contains(out, want) {
			t.Errorf("body missing %q:\n%s", want, out)
		}
	}

	var long []string
	for range maxCodeLines + 5 {
		long = append(long, "x++")
	}
	args, _ = json.Marshal(map[string]string{"code": strings.Join(long, "\n")})
	out = xansi.Strip(renderToolBody("codemode", string(args), "", 100))
	if !strings.Contains(out, "5 more lines") {
		t.Errorf("long script not capped:\n%s", out)
	}

	if got := renderToolBody("codemode", `{"code": ""}`, "x", 100); got != "" {
		t.Errorf("empty code should fall back to the default body, got %q", got)
	}
}

func TestCodeModeHeaderAndActivity(t *testing.T) {
	args := `{"code": "return 1"}`
	if got := formatToolParams(args, 80); got != "" {
		t.Errorf("header params = %q, want the code kept out of the header", got)
	}
	if got := activityVerb("codemode", args); got != "Running script" {
		t.Errorf("activity = %q", got)
	}
}
