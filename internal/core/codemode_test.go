package core

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/codemode"
)

func fakeTool(name string, fn func(ctx context.Context, call fantasy.ToolCall) fantasy.ToolResponse) fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        name,
			Description: "fake " + name,
			Parameters:  map[string]any{"q": map[string]any{"type": "string"}},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fn(ctx, call), nil
		},
	}
}

func runCodeMode(t *testing.T, tool fantasy.AgentTool, ctx context.Context, code string) fantasy.ToolResponse {
	t.Helper()
	input, _ := json.Marshal(map[string]string{"code": code})
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call_9", Name: CodeModeToolName, Input: string(input)})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return resp
}

func TestCodeModeToolEndToEnd(t *testing.T) {
	var nestedOutputCB bool
	echo := fakeTool("echo", func(ctx context.Context, call fantasy.ToolCall) fantasy.ToolResponse {
		if toolOutputCallbackFromContext(ctx) != nil {
			nestedOutputCB = true
		}
		return fantasy.NewTextResponse("echo:" + call.Input + ":" + call.ID)
	})
	img := fakeTool("shot", func(context.Context, fantasy.ToolCall) fantasy.ToolResponse {
		return fantasy.NewImageResponse([]byte{1, 2, 3}, "image/png")
	})
	cm := NewCodeModeTool().(*CodeModeTool)
	cm.ObserveToolSet([]fantasy.AgentTool{echo, img, cm})

	var mu sync.Mutex
	var progress []string
	ctx := ContextWithToolOutputCallback(context.Background(), func(id, name, chunk string, isErr bool) {
		mu.Lock()
		defer mu.Unlock()
		if id != "call_9" || name != CodeModeToolName {
			t.Errorf("progress for %s/%s", id, name)
		}
		progress = append(progress, chunk)
	})

	resp := runCodeMode(t, cm, ctx, `
console.log("start")
const r = await tools.echo({q: "hi"})
const s = await tools.shot({})
return [r, s]`)
	if resp.IsError {
		t.Fatalf("unexpected error: %s", resp.Content)
	}
	if nestedOutputCB {
		t.Error("nested call received the streaming callback")
	}
	for _, want := range []string{"Script completed", "Tool calls: 2 — echo ×1, shot ×1", "Logs:\nstart", `echo:{\"q\":\"hi\"}:call_9.1`, "cannot be passed into a script"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("result missing %q:\n%s", want, resp.Content)
		}
	}
	joined := strings.Join(progress, "\n")
	for _, want := range []string{"▸ echo {\"q\":\"hi\"}", "✓ echo", "start"} {
		if !strings.Contains(joined, want) {
			t.Errorf("progress missing %q:\n%s", want, joined)
		}
	}
	var meta map[string]struct {
		OK    bool            `json:"ok"`
		Calls []codemode.Call `json:"calls"`
	}
	if err := json.Unmarshal([]byte(resp.Metadata), &meta); err != nil {
		t.Fatalf("metadata: %v (%s)", err, resp.Metadata)
	}
	if m := meta["codemode"]; !m.OK || len(m.Calls) != 2 {
		t.Fatalf("metadata = %+v", m)
	}
}

func TestCodeModeToolCannotCallItself(t *testing.T) {
	cm := NewCodeModeTool().(*CodeModeTool)
	cm.ObserveToolSet([]fantasy.AgentTool{cm})
	resp := runCodeMode(t, cm, context.Background(), `await tools.codemode({code: "1"})`)
	if !resp.IsError || !strings.Contains(resp.Content, "UnknownTool") {
		t.Fatalf("expected UnknownTool error, got: %s", resp.Content)
	}
}

func TestCodeModeToolFailureFormat(t *testing.T) {
	cm := NewCodeModeTool().(*CodeModeTool)
	resp := runCodeMode(t, cm, context.Background(), "const x = 1\nnull.y")
	if !resp.IsError {
		t.Fatal("expected error response")
	}
	for _, want := range []string{"Script failed after", "ExecutionFailure at line 2", "TypeError"} {
		if !strings.Contains(resp.Content, want) {
			t.Errorf("missing %q:\n%s", want, resp.Content)
		}
	}
}

func TestCodeModeToolTruncatesToFile(t *testing.T) {
	cm := NewCodeModeTool(WithCodeMode(CodeModeConfig{Limits: codemode.Limits{MaxOutputBytes: 200}})).(*CodeModeTool)
	resp := runCodeMode(t, cm, context.Background(), `return "é".repeat(1000)`)
	if resp.IsError {
		t.Fatalf("unexpected error: %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "[output truncated") {
		t.Fatalf("not truncated:\n%s", resp.Content)
	}
	var meta map[string]CodeModeRunInfo
	_ = json.Unmarshal([]byte(resp.Metadata), &meta)
	path := meta["codemode"].FullPath
	if path == "" {
		t.Fatal("no full output path")
	}
	t.Cleanup(func() {
		if err := os.Remove(path); err != nil {
			t.Errorf("remove %s: %v", path, err)
		}
	})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), strings.Repeat("é", 1000)) {
		t.Fatal("full output file is incomplete")
	}
	if !strings.Contains(resp.Content, path) {
		t.Fatal("result does not name the file")
	}
}

func TestCodeModeToolCancelledTurn(t *testing.T) {
	cm := NewCodeModeTool().(*CodeModeTool)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	input, _ := json.Marshal(map[string]string{"code": "while(true){}"})
	if _, err := cm.Run(ctx, fantasy.ToolCall{ID: "x", Input: string(input)}); err == nil {
		t.Fatal("expected the context error for a cancelled turn")
	}
}

func TestCodeModeVisibilityAndDescription(t *testing.T) {
	cm := NewCodeModeTool(WithCodeMode(CodeModeConfig{
		Policy: codemode.Policy{MCPDefault: codemode.ExposureCodeMode},
	})).(*CodeModeTool)
	if cm.ModelVisible("srv__tool") {
		t.Error("MCP tool should be hidden with mcp default codemode")
	}
	if !cm.ModelVisible("read") || !cm.ModelVisible(CodeModeToolName) {
		t.Error("core tools and code mode must stay visible")
	}
	cm.ObserveToolSet([]fantasy.AgentTool{fakeTool("read", nil), fakeTool("srv__tool", nil)})
	desc := cm.Info().Description
	if !strings.Contains(desc, "tools.read({q?})") || !strings.Contains(desc, "tools.srv.tool(args: { q?: string })") {
		t.Fatalf("description:\n%s", desc)
	}
}

func TestDefaultCoreToolNamesExcludeCodeMode(t *testing.T) {
	for _, n := range DefaultCoreToolNames() {
		if n == CodeModeToolName {
			t.Fatal("code mode must be opt-in")
		}
	}
	found := false
	for _, n := range ListAllCoreToolNames() {
		found = found || n == CodeModeToolName
	}
	if !found {
		t.Fatal("code mode must be a valid core tool name")
	}
}

func TestUnwrapMCPResult(t *testing.T) {
	cases := map[string]string{
		`{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`:              "a\nb",
		`{"content":[{"type":"text","text":"raw"}],"structuredContent":{"content":"raw"}}`: "raw",
		`{"content":[],"structuredContent":{"n":1}}`:                                       `{"n":1}`,
		`{"content":[{"type":"image","mimeType":"image/png","data":"xx"}]}`:                "[image content (image/png) omitted]",
		`{"content":[{"type":"resource","resource":{"uri":"file:///x","text":"body"}}]}`:   "body",
		`plain text`:      "plain text",
		`{"other": true}`: `{"other": true}`,
	}
	for in, want := range cases {
		if got := unwrapMCPResult(in); got != want {
			t.Errorf("unwrapMCPResult(%s) = %q, want %q", in, got, want)
		}
	}
}
