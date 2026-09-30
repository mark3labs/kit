package acpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/mark3labs/kit/internal/message"
	kit "github.com/mark3labs/kit/pkg/kit"
)

func TestInitializeAdvertisesImplementedCapabilities(t *testing.T) {
	resp, err := NewAgent().Initialize(context.Background(), acp.InitializeRequest{ProtocolVersion: 7})
	if err != nil {
		t.Fatal(err)
	}
	// Only version 1 exists; the agent answers with its latest version.
	if resp.ProtocolVersion != acp.ProtocolVersion(acp.ProtocolVersionNumber) {
		t.Errorf("protocol version = %d", resp.ProtocolVersion)
	}
	caps := resp.AgentCapabilities
	if !caps.LoadSession {
		t.Error("loadSession not advertised, but LoadSession is implemented")
	}
	sc := caps.SessionCapabilities
	if sc.List == nil || sc.Close == nil || sc.Resume == nil {
		t.Errorf("session capabilities = %+v, want list, close and resume", sc)
	}
	if resp.AuthMethods == nil {
		t.Error("authMethods must be an empty array, not null")
	}
	b, _ := json.Marshal(resp)
	if !strings.Contains(string(b), `"authMethods":[]`) {
		t.Errorf("authMethods not serialized as []: %s", b)
	}
}

// Agent must satisfy the optional loader interface, or the SDK answers
// session/load with "method not found" although loadSession is advertised.
func TestAgentImplementsLoader(t *testing.T) {
	var a any = NewAgent()
	if _, ok := a.(acp.AgentLoader); !ok {
		t.Fatal("Agent does not implement acp.AgentLoader")
	}
}

func TestValidateCwd(t *testing.T) {
	for _, tc := range []struct {
		cwd  string
		fail bool
	}{
		{"", true},
		{"relative/dir", true},
		{".", true},
		{string(filepath.Separator) + "tmp", false},
	} {
		err := validateCwd(tc.cwd)
		if (err != nil) != tc.fail {
			t.Errorf("validateCwd(%q) error = %v, want fail=%v", tc.cwd, err, tc.fail)
		}
		var re *acp.RequestError
		if err != nil && (!errors.As(err, &re) || re.Code != -32602) {
			t.Errorf("validateCwd(%q) = %v, want InvalidParams", tc.cwd, err)
		}
	}
}

func TestSetSessionModeIsRejected(t *testing.T) {
	_, err := NewAgent().SetSessionMode(context.Background(), acp.SetSessionModeRequest{SessionId: "s", ModeId: "code"})
	if err == nil {
		t.Fatal("SetSessionMode must fail: no modes are advertised")
	}
}

func TestSetSessionConfigOptionUnknownSession(t *testing.T) {
	_, err := NewAgent().SetSessionConfigOption(context.Background(), acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{SessionId: "missing", ConfigId: "model", Value: "x/y"},
	})
	var re *acp.RequestError
	if !errors.As(err, &re) || re.Code != -32002 {
		t.Fatalf("err = %v, want resource not found (-32002)", err)
	}
	_, err = NewAgent().SetSessionConfigOption(context.Background(), acp.SetSessionConfigOptionRequest{
		Boolean: &acp.SetSessionConfigOptionBoolean{SessionId: "s", ConfigId: "x", Value: true},
	})
	if !errors.As(err, &re) || re.Code != -32602 {
		t.Fatalf("boolean err = %v, want InvalidParams", err)
	}
}

func TestStopReason(t *testing.T) {
	for in, want := range map[string]acp.StopReason{
		kit.FinishReasonStop:          acp.StopReasonEndTurn,
		kit.FinishReasonLength:        acp.StopReasonMaxTokens,
		kit.FinishReasonContentFilter: acp.StopReasonRefusal,
		kit.FinishReasonToolCalls:     acp.StopReasonMaxTurnRequests,
		"":                            acp.StopReasonEndTurn,
	} {
		if got := stopReason(&kit.TurnResult{StopReason: in}); got != want {
			t.Errorf("stopReason(%q) = %q, want %q", in, got, want)
		}
	}
	if got := stopReason(nil); got != acp.StopReasonEndTurn {
		t.Errorf("stopReason(nil) = %q", got)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	for _, n := range []int{0, 1, 100, 12345} {
		got, err := decodeCursor(encodeCursor(n))
		if err != nil || got != n {
			t.Errorf("round trip %d = %d, %v", n, got, err)
		}
	}
	for _, bad := range []string{"!!", "bm9wZQ", encodeCursor(-1)} {
		if _, err := decodeCursor(bad); err == nil {
			t.Errorf("decodeCursor(%q) accepted a bad cursor", bad)
		}
	}
}

func TestToolIDMapperMakesRepeatedIDsUnique(t *testing.T) {
	m := newToolIDMapper()
	a := m.start("ls_0")
	if m.lookup("ls_0") != a {
		t.Fatal("lookup does not return the ID of the call")
	}
	b := m.start("ls_0") // provider reused the ID in a later response
	if a == b {
		t.Fatalf("repeated provider ID mapped to the same ACP ID %q", a)
	}
	if m.lookup("ls_0") != b {
		t.Error("lookup must return the latest call's ID")
	}
	c := m.start("")
	if c == "" || m.lookup("") != c {
		t.Errorf("empty ID: start=%q lookup=%q", c, m.lookup(""))
	}
}

func TestHistoryUpdates(t *testing.T) {
	msgs := []kit.StructuredMessage{
		{Role: kit.RoleUser, Parts: []kit.ContentPart{
			message.TextContent{Text: "list files"},
			message.ImageContent{Data: []byte{1, 2, 3}, MediaType: "image/png"},
		}},
		{Role: kit.RoleAssistant, Parts: []kit.ContentPart{
			message.ReasoningContent{Thinking: "use ls"},
			message.TextContent{Text: "Listing."},
			message.ToolCall{ID: "ls_0", Name: "ls", Input: `{"path":"sub"}`},
		}},
		{Role: kit.RoleTool, Parts: []kit.ContentPart{
			message.ToolResult{ToolCallID: "ls_0", Name: "ls", Content: "a.txt"},
		}},
		{Role: kit.RoleAssistant, Parts: []kit.ContentPart{
			message.ToolCall{ID: "ls_0", Name: "ls", Input: `{}`},
		}},
		{Role: kit.RoleTool, Parts: []kit.ContentPart{
			message.ToolResult{ToolCallID: "ls_0", Name: "ls", Content: "boom", IsError: true},
		}},
	}
	updates := historyUpdates(msgs, "/work", newToolIDMapper())

	var kinds []string
	for _, u := range updates {
		b, err := json.Marshal(u)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		kinds = append(kinds, m["sessionUpdate"].(string))
	}
	want := []string{
		"user_message_chunk", "user_message_chunk",
		"agent_thought_chunk", "agent_message_chunk",
		"tool_call", "tool_call_update",
		"tool_call", "tool_call_update",
	}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("kinds = %v\nwant    %v", kinds, want)
	}

	first := updates[4].ToolCall
	if first == nil || first.Kind != acp.ToolKindRead || first.Title != "ls sub" {
		t.Fatalf("first tool call = %+v", first)
	}
	if len(first.Locations) != 1 || first.Locations[0].Path != filepath.Join("/work", "sub") {
		t.Errorf("locations = %+v", first.Locations)
	}
	second := updates[6].ToolCall
	if second.ToolCallId == first.ToolCallId {
		t.Error("reused provider tool ID must map to a new ACP ID")
	}
	if updates[7].ToolCallUpdate.ToolCallId != second.ToolCallId ||
		updates[7].ToolCallUpdate.Status == nil || *updates[7].ToolCallUpdate.Status != acp.ToolCallStatusFailed {
		t.Errorf("second result = %+v", updates[7].ToolCallUpdate)
	}
}

func TestToolKindAndTitle(t *testing.T) {
	cases := []struct {
		name  string
		args  map[string]any
		kind  acp.ToolKind
		title string
	}{
		{"shell", map[string]any{"command": "go test ./...\necho done"}, acp.ToolKindExecute, "shell: go test ./... …"},
		{"edit", map[string]any{"path": "main.go"}, acp.ToolKindEdit, "edit main.go"},
		{"grep", map[string]any{"pattern": "TODO", "path": "internal"}, acp.ToolKindSearch, "grep TODO in internal"},
		{"read", nil, acp.ToolKindRead, "read"},
		{"github__create_issue", map[string]any{"title": "x"}, acp.ToolKindOther, "github__create_issue"},
	}
	for _, c := range cases {
		if got := acpToolKind(c.name); got != c.kind {
			t.Errorf("acpToolKind(%q) = %q, want %q", c.name, got, c.kind)
		}
		if got := toolTitle(c.name, c.args); got != c.title {
			t.Errorf("toolTitle(%q) = %q, want %q", c.name, got, c.title)
		}
	}
}

func TestToolResultContentIncludesDiffs(t *testing.T) {
	content := toolResultContent("ok", &kit.ToolResultMetadata{FileDiffs: []kit.FileDiffInfo{{
		Path:       "/w/a.go",
		DiffBlocks: []kit.DiffBlock{{OldText: "a", NewText: "b"}},
	}}})
	if len(content) != 2 || content[0].Diff == nil || content[1].Content == nil {
		t.Fatalf("content = %+v", content)
	}
	if content[0].Diff.OldText == nil || *content[0].Diff.OldText != "a" || content[0].Diff.NewText != "b" {
		t.Errorf("diff = %+v", content[0].Diff)
	}
}

func TestMCPServerConfig(t *testing.T) {
	name, cfg, err := mcpServerConfig(acp.McpServer{Stdio: &acp.McpServerStdio{
		Name: "fs", Command: "npx", Args: []string{"-y", "srv"},
		Env: []acp.EnvVariable{{Name: "TOKEN", Value: "t"}},
	}})
	if err != nil || name != "fs" || cfg.GetTransportType() != "stdio" ||
		strings.Join(cfg.Command, " ") != "npx -y srv" || cfg.Environment["TOKEN"] != "t" {
		t.Errorf("stdio: %q %+v %v", name, cfg, err)
	}

	name, cfg, err = mcpServerConfig(acp.McpServer{Http: &acp.McpServerHttpInline{
		Name: "api", Url: "https://x/mcp", Headers: []acp.HttpHeader{{Name: "Authorization", Value: "Bearer k"}},
	}})
	if err != nil || name != "api" || cfg.GetTransportType() != "streamable" ||
		len(cfg.Headers) != 1 || cfg.Headers[0] != "Authorization: Bearer k" {
		t.Errorf("http: %q %+v %v", name, cfg, err)
	}

	_, cfg, err = mcpServerConfig(acp.McpServer{Sse: &acp.McpServerSseInline{Name: "old", Url: "https://x/sse"}})
	if err != nil || cfg.GetTransportType() != "sse" {
		t.Errorf("sse: %+v %v", cfg, err)
	}

	if _, _, err := mcpServerConfig(acp.McpServer{Stdio: &acp.McpServerStdio{Name: "x"}}); err == nil {
		t.Error("stdio server without a command must be rejected")
	}
	if _, _, err := mcpServerConfig(acp.McpServer{}); err == nil {
		t.Error("empty server entry must be rejected")
	}
}

func TestResourceURIs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "my file#1.txt")
	if err := os.WriteFile(p, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(p)}).String()
	if !strings.Contains(uri, "%20") {
		t.Fatalf("test URI is not percent-encoded: %s", uri)
	}
	got, err := readResourceFromURI(uri)
	if err != nil || string(got) != "hello" {
		t.Fatalf("readResourceFromURI(%q) = %q, %v", uri, got, err)
	}
	if name := extractFilenameFromURI(uri); name != "my file#1.txt" {
		t.Errorf("extractFilenameFromURI = %q", name)
	}
	if _, err := readResourceFromURI("https://example.com/a.txt"); err == nil {
		t.Error("non-file URI must be rejected")
	}
	if _, err := readResourceFromURI("file://remote-host/etc/passwd"); err == nil {
		t.Error("remote file URI must be rejected")
	}
}

func TestMediaFilename(t *testing.T) {
	for in, want := range map[string]string{
		"image/png":           "image.png",
		"image/jpeg":          "image.jpg",
		"image/svg+xml":       "image.svg",
		"audio/x-wav":         "image.wav",
		"text/plain; a=b":     "image.plain",
		"broken":              "image",
		"image/webp;charset=": "image.webp",
	} {
		if got := mediaFilename("image", in); got != want {
			t.Errorf("mediaFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOptionHasValue(t *testing.T) {
	groups := acp.SessionConfigSelectOptionsGrouped{{
		Group: "p", Name: "p",
		Options: []acp.SessionConfigSelectOption{{Name: "M", Value: "p/m"}},
	}}
	opt := acp.SessionConfigOption{Select: &acp.SessionConfigOptionSelect{
		Id: "model", Name: "Model", CurrentValue: "p/m",
		Options: acp.SessionConfigSelectOptions{Grouped: &groups},
	}}
	if !optionHasValue(opt, "p/m") || optionHasValue(opt, "p/x") {
		t.Error("optionHasValue wrong for grouped options")
	}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"type":"select"`) || !strings.Contains(string(b), `"group":"p"`) {
		t.Errorf("marshaled option = %s", b)
	}
}

func TestNeedsApproval(t *testing.T) {
	for _, c := range []struct {
		mode, tool string
		want       bool
	}{
		{approvalAsk, "read", false},
		{approvalAsk, "grep", false},
		{approvalAsk, planToolName, false},
		{approvalAsk, "edit", true},
		{approvalAsk, "shell", true},
		{approvalAsk, "github__create_issue", true},
		{approvalAutoEdit, "edit", false},
		{approvalAutoEdit, "write", false},
		{approvalAutoEdit, "shell", true},
		{approvalAutoEdit, "subagent", true},
		{approvalAuto, "shell", false},
	} {
		if got := needsApproval(c.mode, c.tool); got != c.want {
			t.Errorf("needsApproval(%s, %s) = %v, want %v", c.mode, c.tool, got, c.want)
		}
	}
	if err := NewAgent().SetDefaultApproval("sometimes"); err == nil {
		t.Error("unknown approval mode accepted")
	}
}

func TestPlanEntries(t *testing.T) {
	entries, err := planEntries(planInput{Entries: []planEntryInput{
		{Content: " a ", Status: "in_progress"},
		{Content: "b", Priority: "high"},
		{Content: "c", Status: "completed", Priority: "urgent"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []acp.PlanEntry{
		{Content: "a", Status: acp.PlanEntryStatusInProgress, Priority: acp.PlanEntryPriorityMedium},
		{Content: "b", Status: acp.PlanEntryStatusPending, Priority: acp.PlanEntryPriorityHigh},
		{Content: "c", Status: acp.PlanEntryStatusCompleted, Priority: acp.PlanEntryPriorityMedium},
	}
	for i := range want {
		if entries[i].Content != want[i].Content || entries[i].Status != want[i].Status || entries[i].Priority != want[i].Priority {
			t.Errorf("entry %d = %+v, want %+v", i, entries[i], want[i])
		}
	}
	if _, err := planEntries(planInput{Entries: []planEntryInput{{Content: ""}}}); err == nil {
		t.Error("empty step accepted")
	}
	if _, err := planEntries(planInput{Entries: []planEntryInput{{Content: "x", Status: "done"}}}); err == nil {
		t.Error("unknown status accepted")
	}

	// History replay turns update_plan calls into plan updates.
	updates := historyUpdates([]kit.StructuredMessage{
		{Role: kit.RoleAssistant, Parts: []kit.ContentPart{
			message.ToolCall{ID: "p1", Name: planToolName, Input: `{"entries":[{"content":"x","status":"pending"}]}`},
		}},
		{Role: kit.RoleTool, Parts: []kit.ContentPart{
			message.ToolResult{ToolCallID: "p1", Name: planToolName, Content: "Plan updated (1 steps)."},
		}},
	}, "/w", newToolIDMapper())
	if len(updates) != 1 || updates[0].Plan == nil || updates[0].Plan.Entries[0].Content != "x" {
		t.Errorf("replayed plan = %+v", updates)
	}
}

func TestSessionErrorKeepsACPErrors(t *testing.T) {
	auth := authRequired(errors.New("no API key for anthropic"))
	if auth.Code != -32000 {
		t.Fatalf("auth code = %d", auth.Code)
	}
	var re *acp.RequestError
	if err := sessionError("create session", auth); !errors.As(err, &re) || err != error(auth) {
		t.Errorf("sessionError wrapped an ACP error: %v", err)
	}
	if err := sessionError("create session", errors.New("boom")); err.Error() != "create session: boom" {
		t.Errorf("sessionError = %v", err)
	}
}

func TestExpandPromptTemplate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cwd := t.TempDir()
	dir := filepath.Join(cwd, ".kit", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte("Review $1 carefully."), 0o600); err != nil {
		t.Fatal(err)
	}
	for in, want := range map[string]string{
		"/review main.go":     "Review main.go carefully.",
		"plain text":          "plain text",
		"/unknown-skill args": "/unknown-skill args",
		"/":                   "/",
	} {
		got, err := expandPromptTemplate(cwd, in)
		if err != nil || got != want {
			t.Errorf("expand(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := expandPromptTemplate(cwd, "/review"); err == nil {
		t.Error("missing argument accepted")
	}
}

func TestTrimDriveSlash(t *testing.T) {
	for _, c := range []struct {
		in      string
		windows bool
		want    string
	}{
		{"/C:/work/a.txt", true, "C:/work/a.txt"},
		{"/c:/a", true, "c:/a"},
		{"/C:/work/a.txt", false, "/C:/work/a.txt"}, // a valid Unix path
		{"/home/u/a.txt", true, "/home/u/a.txt"},
		{"/1:/a", true, "/1:/a"},
		{"/C", true, "/C"},
	} {
		if got := trimDriveSlash(c.in, c.windows); got != c.want {
			t.Errorf("trimDriveSlash(%q, %v) = %q, want %q", c.in, c.windows, got, c.want)
		}
	}
}

// A provider that sends the same (here: empty) raw ID for every call must
// not make the replay skip the result of a later call after update_plan.
func TestHistoryUpdatesPlanIDReused(t *testing.T) {
	updates := historyUpdates([]kit.StructuredMessage{
		{Role: kit.RoleAssistant, Parts: []kit.ContentPart{
			message.ToolCall{ID: "", Name: planToolName, Input: `{"entries":[{"content":"x","status":"pending"}]}`},
		}},
		{Role: kit.RoleTool, Parts: []kit.ContentPart{
			message.ToolResult{ToolCallID: "", Name: planToolName, Content: "Plan updated (1 steps)."},
		}},
		{Role: kit.RoleAssistant, Parts: []kit.ContentPart{
			message.ToolCall{ID: "", Name: "ls", Input: `{}`},
		}},
		{Role: kit.RoleTool, Parts: []kit.ContentPart{
			message.ToolResult{ToolCallID: "", Name: "ls", Content: "a.txt"},
		}},
	}, "/w", newToolIDMapper())

	var call, result bool
	for _, u := range updates {
		if u.ToolCall != nil {
			call = true
		}
		if u.ToolCallUpdate != nil && u.ToolCallUpdate.Status != nil && *u.ToolCallUpdate.Status == acp.ToolCallStatusCompleted {
			result = true
		}
	}
	if !call || !result {
		t.Fatalf("ls call=%v result=%v; the result after a plan call with the same ID was skipped", call, result)
	}
}
