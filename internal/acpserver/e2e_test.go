package acpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
	acp "github.com/coder/acp-go-sdk"
	"github.com/spf13/viper"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// ---------------------------------------------------------------------------
// Scripted model: one tool call per step, then a text answer.
// ---------------------------------------------------------------------------

type scriptStep struct {
	tool  string
	input string
}

type scriptedModel struct {
	name   string
	script []scriptStep

	mu    sync.Mutex
	calls int
	// lastToolResult is the text of the last tool result the model saw.
	lastToolResult string
}

func (m *scriptedModel) Provider() string { return m.name }
func (m *scriptedModel) Model() string    { return "m" }

func (m *scriptedModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("streaming only")
}

func (m *scriptedModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	i := m.calls
	m.calls++
	if n := len(call.Prompt); n > 0 {
		for _, part := range call.Prompt[n-1].Content {
			if tr, ok := part.(fantasy.ToolResultPart); ok {
				if txt, ok := tr.Output.(fantasy.ToolResultOutputContentText); ok {
					m.lastToolResult = txt.Text
				}
				if e, ok := tr.Output.(fantasy.ToolResultOutputContentError); ok && e.Error != nil {
					m.lastToolResult = e.Error.Error()
				}
			}
		}
	}
	m.mu.Unlock()

	var parts []fantasy.StreamPart
	if i < len(m.script) {
		st := m.script[i]
		id := fmt.Sprintf("%s_%d", st.tool, i)
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeToolInputStart, ID: id, ToolCallName: st.tool},
			{Type: fantasy.StreamPartTypeToolInputDelta, ID: id, Delta: st.input},
			{Type: fantasy.StreamPartTypeToolInputEnd, ID: id},
			{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: st.tool, ToolCallInput: st.input},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
		}
	} else {
		parts = []fantasy.StreamPart{
			{Type: fantasy.StreamPartTypeTextStart, ID: "t"},
			{Type: fantasy.StreamPartTypeTextDelta, ID: "t", Delta: "done"},
			{Type: fantasy.StreamPartTypeTextEnd, ID: "t"},
			{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
		}
	}
	return iter.Seq[fantasy.StreamPart](func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}), nil
}

func (m *scriptedModel) toolResult() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastToolResult
}

// ---------------------------------------------------------------------------
// Fake editor client
// ---------------------------------------------------------------------------

type fakeClient struct {
	t *testing.T

	// permission answers the permission request. Nil allows once.
	permission func(acp.RequestPermissionRequest) acp.RequestPermissionOutcome

	mu          sync.Mutex
	updates     []acp.SessionUpdate
	permissions []acp.RequestPermissionRequest
	files       map[string]string
	terminals   []acp.CreateTerminalRequest
	released    []string
}

func (c *fakeClient) ReadTextFile(_ context.Context, p acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s, ok := c.files[p.Path]; ok {
		return acp.ReadTextFileResponse{Content: s}, nil
	}
	return acp.ReadTextFileResponse{}, resourceNotFound(p.Path)
}

func (c *fakeClient) WriteTextFile(_ context.Context, p acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[p.Path] = p.Content
	return acp.WriteTextFileResponse{}, nil
}

func (c *fakeClient) RequestPermission(_ context.Context, p acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	c.mu.Lock()
	c.permissions = append(c.permissions, p)
	answer := c.permission
	c.mu.Unlock()
	if answer == nil {
		return acp.RequestPermissionResponse{Outcome: acp.NewRequestPermissionOutcomeSelected(optAllowOnce)}, nil
	}
	return acp.RequestPermissionResponse{Outcome: answer(p)}, nil
}

func (c *fakeClient) SessionUpdate(_ context.Context, n acp.SessionNotification) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updates = append(c.updates, n.Update)
	return nil
}

func (c *fakeClient) CreateTerminal(_ context.Context, p acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.terminals = append(c.terminals, p)
	return acp.CreateTerminalResponse{TerminalId: fmt.Sprintf("term-%d", len(c.terminals))}, nil
}

func (c *fakeClient) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{ExitCode: new(0)}, nil
}

func (c *fakeClient) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{Output: "hi from the editor terminal\n", ExitStatus: &acp.TerminalExitStatus{ExitCode: new(0)}}, nil
}

func (c *fakeClient) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, nil
}

func (c *fakeClient) ReleaseTerminal(_ context.Context, p acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.released = append(c.released, p.TerminalId)
	return acp.ReleaseTerminalResponse{}, nil
}

func (c *fakeClient) snapshot() ([]acp.SessionUpdate, []acp.RequestPermissionRequest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]acp.SessionUpdate(nil), c.updates...), append([]acp.RequestPermissionRequest(nil), c.permissions...)
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type harness struct {
	client *fakeClient
	conn   *acp.ClientSideConnection
	agent  *Agent
	model  *scriptedModel
	cwd    string
}

var providerSeq struct {
	sync.Mutex
	n int
}

// newHarness starts an agent and a fake client connected by pipes. The
// agent's model is a scripted model. HOME points to a temp dir, so sessions
// and config do not touch the real user files.
func newHarness(t *testing.T, script []scriptStep, caps acp.ClientCapabilities) *harness {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))

	providerSeq.Lock()
	providerSeq.n++
	name := fmt.Sprintf("acptest%d", providerSeq.n)
	providerSeq.Unlock()

	model := &scriptedModel{name: name, script: script}
	if err := kit.RegisterProvider(name, func(context.Context, *kit.ProviderConfig, string) (*kit.ProviderResult, error) {
		return &kit.ProviderResult{Model: model}, nil
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kit.UnregisterProvider(name) })

	oldModel := viper.Get("model")
	viper.Set("model", name+"/m")
	viper.Set("no-extensions", true)
	t.Cleanup(func() {
		viper.Set("model", oldModel)
		viper.Set("no-extensions", nil)
	})

	agentR, clientW := io.Pipe()
	clientR, agentW := io.Pipe()
	agent := NewAgent()
	aconn := acp.NewAgentSideConnection(agent, agentW, agentR)
	agent.SetAgentConnection(aconn)

	client := &fakeClient{t: t, files: map[string]string{}}
	cconn := acp.NewClientSideConnection(client, clientW, clientR)
	t.Cleanup(func() {
		agent.Close()
		_ = clientW.Close()
		_ = agentW.Close()
	})

	ctx := context.Background()
	if _, err := cconn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion:    acp.ProtocolVersionNumber,
		ClientCapabilities: caps,
	}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	cwd := t.TempDir()
	return &harness{client: client, conn: cconn, agent: agent, model: model, cwd: cwd}
}

func (h *harness) newSession(t *testing.T) acp.SessionId {
	t.Helper()
	resp, err := h.conn.NewSession(context.Background(), acp.NewSessionRequest{Cwd: h.cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		t.Fatalf("session/new: %v", err)
	}
	return resp.SessionId
}

func (h *harness) prompt(t *testing.T, sid acp.SessionId, text string) acp.PromptResponse {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := h.conn.Prompt(ctx, acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock(text)}})
	if err != nil {
		t.Fatalf("session/prompt: %v", err)
	}
	// Notifications are handled in order; give the last ones a moment.
	time.Sleep(50 * time.Millisecond)
	return resp
}

// toolStatuses returns the status sequence of each tool call, in order.
func toolStatuses(updates []acp.SessionUpdate) map[acp.ToolCallId][]acp.ToolCallStatus {
	out := map[acp.ToolCallId][]acp.ToolCallStatus{}
	for _, u := range updates {
		switch {
		case u.ToolCall != nil:
			out[u.ToolCall.ToolCallId] = append(out[u.ToolCall.ToolCallId], u.ToolCall.Status)
		case u.ToolCallUpdate != nil && u.ToolCallUpdate.Status != nil:
			out[u.ToolCallUpdate.ToolCallId] = append(out[u.ToolCallUpdate.ToolCallId], *u.ToolCallUpdate.Status)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

var fullClient = acp.ClientCapabilities{
	Fs:       acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
	Terminal: true,
}

func TestEndToEndPermissionsClientFSTerminalAndPlan(t *testing.T) {
	h := newHarness(t, []scriptStep{
		{tool: planToolName, input: `{"entries":[{"content":"Write the file","status":"in_progress"},{"content":"Run it","status":"pending","priority":"high"}]}`},
		{tool: "write", input: `{"path":"out.txt","content":"hello"}`},
		{tool: "read", input: `{"path":"out.txt"}`},
		{tool: "shell", input: `{"command":"echo hi"}`},
	}, fullClient)
	sid := h.newSession(t)

	resp := h.prompt(t, sid, "do it")
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("stop reason = %q", resp.StopReason)
	}
	updates, perms := h.client.snapshot()

	// Permissions: write and shell ask; plan and read do not.
	var asked []string
	for _, p := range perms {
		if p.ToolCall.Title != nil {
			asked = append(asked, *p.ToolCall.Title)
		}
		if len(p.Options) != 4 {
			t.Errorf("permission options = %d, want 4", len(p.Options))
		}
	}
	if strings.Join(asked, "|") != "write out.txt|shell: echo hi" {
		t.Errorf("permission requests = %q", asked)
	}

	// The write went through the client, not to disk.
	target := filepath.Join(h.cwd, "out.txt")
	if h.client.files[target] != "hello" {
		t.Errorf("client files = %v", h.client.files)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("write reached the disk: %v", err)
	}
	// The shell output came from the client terminal. (The read of the
	// file that exists only in the client buffer is checked by the tool
	// statuses below: it must complete.)
	if !strings.Contains(h.model.toolResult(), "hi from the editor terminal") {
		t.Errorf("last tool result = %q", h.model.toolResult())
	}

	// The shell ran in a client terminal in the session cwd, and the
	// terminal was embedded in the tool call and released.
	if len(h.client.terminals) != 1 {
		t.Fatalf("terminals = %+v", h.client.terminals)
	}
	term := h.client.terminals[0]
	if term.Cwd == nil || *term.Cwd != h.cwd || term.Args[len(term.Args)-1] != "echo hi" {
		t.Errorf("terminal request = %+v", term)
	}
	if len(h.client.released) != 1 {
		t.Errorf("released = %v", h.client.released)
	}
	var embedded bool
	for _, u := range updates {
		if u.ToolCallUpdate != nil {
			for _, c := range u.ToolCallUpdate.Content {
				if c.Terminal != nil && c.Terminal.TerminalId == "term-1" {
					embedded = true
				}
			}
		}
	}
	if !embedded {
		t.Error("terminal was not embedded in the tool call")
	}

	// The plan tool showed a plan and no tool call.
	var plan *acp.SessionUpdatePlan
	for _, u := range updates {
		if u.Plan != nil {
			plan = u.Plan
		}
		if u.ToolCall != nil && strings.HasPrefix(string(u.ToolCall.ToolCallId), planToolName) {
			t.Error("plan tool was shown as a tool call")
		}
	}
	if plan == nil || len(plan.Entries) != 2 || plan.Entries[1].Priority != acp.PlanEntryPriorityHigh {
		t.Errorf("plan = %+v", plan)
	}

	// Each tool call: pending -> in_progress -> completed.
	for id, st := range toolStatuses(updates) {
		want := []acp.ToolCallStatus{acp.ToolCallStatusPending, acp.ToolCallStatusInProgress, acp.ToolCallStatusCompleted}
		if fmt.Sprint(st) != fmt.Sprint(want) {
			t.Errorf("tool %s statuses = %v, want %v", id, st, want)
		}
	}

}

func TestEndToEndRejectContinuesTheTurn(t *testing.T) {
	h := newHarness(t, []scriptStep{
		{tool: "write", input: `{"path":"x.txt","content":"x"}`},
		{tool: "write", input: `{"path":"y.txt","content":"y"}`},
	}, fullClient)
	h.client.permission = func(acp.RequestPermissionRequest) acp.RequestPermissionOutcome {
		return acp.NewRequestPermissionOutcomeSelected(optRejectAlways)
	}
	sid := h.newSession(t)

	resp := h.prompt(t, sid, "write")
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("stop reason = %q, want end_turn: a rejection must not end the turn with an error", resp.StopReason)
	}
	updates, perms := h.client.snapshot()
	if len(perms) != 1 {
		t.Errorf("permission requests = %d, want 1 (the second write is rejected by 'always reject')", len(perms))
	}
	if len(h.client.files) != 0 {
		t.Errorf("rejected writes ran: %v", h.client.files)
	}
	if !strings.Contains(h.model.toolResult(), "does not allow") {
		t.Errorf("model saw %q", h.model.toolResult())
	}
	for id, st := range toolStatuses(updates) {
		if st[len(st)-1] != acp.ToolCallStatusFailed {
			t.Errorf("rejected tool %s statuses = %v, want failed at the end", id, st)
		}
	}
}

func TestEndToEndCancelDuringPermission(t *testing.T) {
	h := newHarness(t, []scriptStep{{tool: "write", input: `{"path":"x.txt","content":"x"}`}}, fullClient)
	var sid acp.SessionId
	h.client.permission = func(acp.RequestPermissionRequest) acp.RequestPermissionOutcome {
		// The user cancels the turn while the prompt is open. The spec
		// requires the client to answer pending requests with "cancelled".
		_ = h.conn.Cancel(context.Background(), acp.CancelNotification{SessionId: sid})
		return acp.NewRequestPermissionOutcomeCancelled()
	}
	sid = h.newSession(t)

	resp := h.prompt(t, sid, "write")
	if resp.StopReason != acp.StopReasonCancelled {
		t.Fatalf("stop reason = %q, want cancelled", resp.StopReason)
	}
	if len(h.client.files) != 0 {
		t.Errorf("cancelled write ran: %v", h.client.files)
	}
}

func TestEndToEndApprovalModes(t *testing.T) {
	h := newHarness(t, []scriptStep{
		{tool: "write", input: `{"path":"a.txt","content":"a"}`},
		{tool: "shell", input: `{"command":"true"}`},
	}, fullClient)
	sid := h.newSession(t)

	resp, err := h.conn.SetSessionConfigOption(context.Background(), acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{SessionId: sid, ConfigId: configIDApproval, Value: approvalAutoEdit},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.ConfigOptions[0].Select; got == nil || got.Id != configIDApproval || got.CurrentValue != approvalAutoEdit {
		t.Fatalf("approval option = %+v", got)
	}

	h.prompt(t, sid, "go")
	_, perms := h.client.snapshot()
	if len(perms) != 1 || perms[0].ToolCall.Title == nil || !strings.HasPrefix(*perms[0].ToolCall.Title, "shell") {
		t.Errorf("auto_edit: permission requests = %d, want only the shell", len(perms))
	}
	if h.client.files[filepath.Join(h.cwd, "a.txt")] != "a" {
		t.Error("auto_edit: the write did not run")
	}

	if _, err := h.conn.SetSessionConfigOption(context.Background(), acp.SetSessionConfigOptionRequest{
		ValueId: &acp.SetSessionConfigOptionValueId{SessionId: sid, ConfigId: configIDApproval, Value: "bogus"},
	}); err == nil {
		t.Error("unknown approval mode accepted")
	}
}

func TestEndToEndWithoutClientCapabilitiesUsesLocalTools(t *testing.T) {
	h := newHarness(t, []scriptStep{
		{tool: "write", input: `{"path":"local.txt","content":"disk"}`},
		{tool: "shell", input: `{"command":"cat local.txt"}`},
	}, acp.ClientCapabilities{})
	if err := h.agent.SetDefaultApproval(approvalAuto); err != nil {
		t.Fatal(err)
	}
	sid := h.newSession(t)
	h.prompt(t, sid, "go")

	if b, err := os.ReadFile(filepath.Join(h.cwd, "local.txt")); err != nil || string(b) != "disk" {
		t.Errorf("local write: %q, %v", b, err)
	}
	if len(h.client.terminals) != 0 || len(h.client.files) != 0 {
		t.Error("client methods used although the client did not advertise them")
	}
	if !strings.Contains(h.model.toolResult(), "disk") {
		t.Errorf("local shell output = %q", h.model.toolResult())
	}
	if _, perms := h.client.snapshot(); len(perms) != 0 {
		t.Errorf("auto mode asked %d time(s)", len(perms))
	}
}

func TestEndToEndPromptTemplateCommand(t *testing.T) {
	h := newHarness(t, nil, fullClient)
	dir := filepath.Join(h.cwd, ".kit", "prompts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	tpl := "---\ndescription: Greet someone\n---\nSay hello to $1."
	if err := os.WriteFile(filepath.Join(dir, "greet.md"), []byte(tpl), 0o600); err != nil {
		t.Fatal(err)
	}
	sid := h.newSession(t)
	time.Sleep(2 * commandsDelay)

	updates, _ := h.client.snapshot()
	var found *acp.AvailableCommand
	for _, u := range updates {
		if u.AvailableCommandsUpdate != nil {
			for i, c := range u.AvailableCommandsUpdate.AvailableCommands {
				if c.Name == "greet" {
					found = &u.AvailableCommandsUpdate.AvailableCommands[i]
				}
			}
		}
	}
	if found == nil || found.Description != "Greet someone" || found.Input == nil {
		t.Fatalf("greet command = %+v", found)
	}

	h.prompt(t, sid, "/greet Ada")
	msgs := h.agent.registry.sessions[string(sid)].kit.GetStructuredMessages()
	var user string
	for _, m := range msgs {
		if m.Role == kit.RoleUser {
			for _, p := range m.Parts {
				if tc, ok := p.(kit.TextContent); ok {
					user += tc.Text
				}
			}
		}
	}
	if !strings.Contains(user, "Say hello to Ada.") {
		t.Errorf("user message = %q, want the expanded template", user)
	}

	_, err := h.conn.Prompt(context.Background(), acp.PromptRequest{SessionId: sid, Prompt: []acp.ContentBlock{acp.TextBlock("/greet")}})
	var re *acp.RequestError
	if !errors.As(err, &re) || re.Code != -32602 {
		t.Errorf("template without its argument: err = %v, want InvalidParams", err)
	}
}

func TestEndToEndMissingCredentialsIsAuthRequired(t *testing.T) {
	h := newHarness(t, nil, acp.ClientCapabilities{})
	// A built-in provider with no key anywhere: HOME is a temp dir, and the
	// environment variable is cleared.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_OAUTH_TOKEN", "")
	viper.Set("model", "anthropic/claude-sonnet-4-5")

	_, err := h.conn.NewSession(context.Background(), acp.NewSessionRequest{Cwd: h.cwd, McpServers: []acp.McpServer{}})
	var re *acp.RequestError
	if !errors.As(err, &re) || re.Code != -32000 {
		t.Fatalf("session/new without credentials: err = %v, want auth_required (-32000)", err)
	}
	if !strings.Contains(fmt.Sprint(re.Data), "kit auth login") {
		t.Errorf("error data = %v, want steps to fix it", re.Data)
	}
}
