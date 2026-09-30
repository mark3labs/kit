package acpserver

import (
	"context"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"
	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/extbridge"
	"github.com/mark3labs/kit/internal/extensions"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// acpSession maps an ACP session to a Kit instance with its own tree session.
type acpSession struct {
	kit       *kit.Kit
	cancelFn  context.CancelFunc // cancels the current prompt
	cancelMu  sync.Mutex
	cwd       string
	sessionID string // Kit-generated session ID (from JSONL header)
	toolIDs   *toolIDMapper

	// mu guards the fields below.
	mu sync.Mutex
	// promptCtx is the context of the running prompt turn. Client requests
	// made during the turn (permissions, files, terminals) use it, so they
	// end when the turn is cancelled.
	promptCtx context.Context
	// approval is the tool approval mode (approvalAsk, approvalAutoEdit or
	// approvalAuto).
	approval string
	// alwaysAllow and alwaysReject hold tool names the user answered with
	// "always" in this session.
	alwaysAllow  map[string]bool
	alwaysReject map[string]bool
	// terminals maps a tool call to the client terminal it runs in, so the
	// final tool call update keeps showing the terminal.
	terminals map[acp.ToolCallId]string
}

// id returns the ACP session ID.
func (s *acpSession) id() acp.SessionId { return acp.SessionId(s.sessionID) }

// context returns the context of the running prompt turn, or a background
// context when no turn runs.
func (s *acpSession) context() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.promptCtx != nil {
		return s.promptCtx
	}
	return context.Background()
}

func (s *acpSession) setPromptContext(ctx context.Context) {
	s.mu.Lock()
	s.promptCtx = ctx
	s.mu.Unlock()
}

func (s *acpSession) clearPromptContext() {
	s.mu.Lock()
	s.promptCtx = nil
	s.mu.Unlock()
}

// sessionRegistry is a thread-safe registry of ACP session ID → Kit sessions.
type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*acpSession // ACP session ID → session
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		sessions: make(map[string]*acpSession),
	}
}

// sessionOptions controls how create builds a session.
type sessionOptions struct {
	// cwd is the absolute working directory of the session. Kit uses it for
	// session storage, project discovery (AGENTS.md, skills, .kit.yml) and
	// as the base directory of the file and shell tools.
	cwd string
	// sessionPath opens an existing JSONL session file instead of creating
	// a new one (session/load and session/resume).
	sessionPath string
	// mcpServers are the MCP servers the client asked the agent to connect
	// to for this session.
	mcpServers []acp.McpServer
	// approval is the initial tool approval mode.
	approval string
	// toolOptions returns extra options for the core tools of the session
	// (client file system and terminal).
	toolOptions func(*acpSession) []kit.ToolOption
	// extraTools returns ACP-only tools for the session.
	extraTools func(*acpSession) []kit.Tool
	// setup runs after the Kit instance exists and before the session is
	// registered, for example to install hooks.
	setup func(*acpSession)
}

// create creates a Kit instance with a persisted tree session. The
// Kit-generated session ID is used as the ACP session ID so the mapping is
// 1:1. When opts.sessionPath is set, the existing session is opened.
func (r *sessionRegistry) create(ctx context.Context, opts sessionOptions) (*acpSession, error) {
	cwd := opts.cwd
	// The request context ends when the ACP response is sent, but the Kit
	// instance and the MCP servers it starts must live as long as the session.
	ctx = context.WithoutCancel(ctx)

	// The session exists before the Kit instance, so that tools built for
	// it can reach its state. The session ID is set once Kit made it.
	sess := &acpSession{
		cwd:          cwd,
		toolIDs:      newToolIDMapper(),
		approval:     opts.approval,
		alwaysAllow:  map[string]bool{},
		alwaysReject: map[string]bool{},
		terminals:    map[acp.ToolCallId]string{},
	}
	if sess.approval == "" {
		sess.approval = approvalAsk
	}
	var toolOpts []kit.ToolOption
	if opts.toolOptions != nil {
		toolOpts = opts.toolOptions(sess)
	}
	var extraTools []kit.Tool
	if opts.extraTools != nil {
		extraTools = opts.extraTools(sess)
	}

	// Each ACP session gets its own isolated config store (CLI is left nil) so
	// per-session SetModel / SetThinkingLevel calls cannot race or bleed across
	// the sessionRegistry. We seed the relevant root-command flag values from
	// the process-global store (which cobra populated from flags) so launching
	// `kit acp -m <model> [--thinking-level ...] [--provider-url ...]
	// [--shell ...]` is still honored; .kit.yml and KIT_* env vars are loaded
	// per session by kit.New.
	streamOn := true
	kitInstance, err := kit.New(ctx, &kit.Options{
		SessionDir:      cwd,
		SessionPath:     opts.sessionPath,
		WorkDir:         cwd,
		CoreToolOptions: toolOpts,
		ExtraTools:      extraTools,
		Quiet:           true,
		Streaming:       &streamOn,
		Model:           viper.GetString("model"),
		ThinkingLevel:   viper.GetString("thinking-level"),
		ProviderURL:     viper.GetString("provider-url"),
		ProviderAPIKey:  viper.GetString("provider-api-key"),
		Shell:           viper.GetStringSlice("shell"),
	})
	if err != nil {
		// Missing provider credentials are the most common failure in ACP
		// mode. Report them with the spec's "authentication required"
		// error so clients can tell the user what to do.
		if kit.IsMissingCredentialsError(err) {
			return nil, authRequired(err)
		}
		return nil, fmt.Errorf("create kit instance: %w", err)
	}

	sessionID := kitInstance.GetSessionID()
	if sessionID == "" {
		_ = kitInstance.Close()
		return nil, fmt.Errorf("kit instance has no session ID")
	}

	// Wire extension context with headless implementations so extensions
	// work in ACP mode. TUI-dependent features (widgets, prompts, editor)
	// become no-ops or return cancelled; all data/model/tool APIs come from
	// extbridge.BaseContext and work identically to interactive mode.
	if kitInstance.Extensions().HasExtensions() {
		// Use a background context for subagent spawns: the create() ctx is
		// request-scoped and may be cancelled before extensions spawn anything.
		ec := extbridge.BaseContext(context.Background(), kitInstance)

		ec.SessionID = sessionID
		ec.CWD = cwd
		ec.Model = kitInstance.GetModelString()
		ec.Interactive = false

		// Output — route through structured logger.
		ec.Print = func(text string) { log.Debug("extension: print", "text", text) }
		ec.PrintInfo = func(text string) { log.Info("extension: info", "text", text) }
		ec.PrintError = func(text string) { log.Error("extension: error", "text", text) }
		ec.PrintBlock = func(opts extensions.PrintBlockOpts) {
			log.Info("extension: block", "subtitle", opts.Subtitle, "text", opts.Text)
		}

		// Message injection — no-ops for now; ACP clients drive prompts.
		ec.SendMessage = func(string) {}
		ec.CancelAndSend = func(string) {}
		ec.NewSession = func(string) error {
			return fmt.Errorf("new session not available in ACP mode")
		}
		ec.Exit = func() {}

		// TUI widgets/chrome — silent no-ops (no TUI in ACP).
		ec.SetWidget = func(extensions.WidgetConfig) {}
		ec.RemoveWidget = func(string) {}
		ec.SetHeader = func(extensions.HeaderFooterConfig) {}
		ec.RemoveHeader = func() {}
		ec.SetFooter = func(extensions.HeaderFooterConfig) {}
		ec.RemoveFooter = func() {}
		ec.SetEditor = func(extensions.EditorConfig) {}
		ec.ResetEditor = func() {}
		ec.SetEditorText = func(string) {}
		ec.SetUIVisibility = func(extensions.UIVisibility) {}
		ec.SetStatus = func(string, string, int) {}
		ec.RemoveStatus = func(string) {}

		// Interactive prompts — return cancelled (no user to prompt).
		ec.PromptSelect = func(extensions.PromptSelectConfig) extensions.PromptSelectResult {
			return extensions.PromptSelectResult{Cancelled: true}
		}
		ec.PromptConfirm = func(extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
			return extensions.PromptConfirmResult{Cancelled: true}
		}
		ec.PromptInput = func(extensions.PromptInputConfig) extensions.PromptInputResult {
			return extensions.PromptInputResult{Cancelled: true}
		}
		ec.ShowOverlay = func(extensions.OverlayConfig) extensions.OverlayResult {
			return extensions.OverlayResult{Cancelled: true, Index: -1}
		}
		ec.SuspendTUI = func(callback func()) error { callback(); return nil }

		// Render — fall back to logging.
		ec.RenderMessage = func(name, content string) {
			renderer := kitInstance.Extensions().GetMessageRenderer(name)
			if renderer != nil && renderer.Render != nil {
				content = renderer.Render(content, 80)
			}
			log.Info("extension: message", "renderer", name, "content", content)
		}

		kitInstance.Extensions().SetContext(ec)
		kitInstance.Extensions().EmitSessionStart()
	}

	// Connect the MCP servers the client supplied. The spec says agents
	// SHOULD connect to all of them; a server that fails is logged and
	// skipped so one bad entry does not block the whole session.
	for _, srv := range opts.mcpServers {
		name, cfg, err := mcpServerConfig(srv)
		if err != nil {
			log.Warn("acp: skipping MCP server", "error", err)
			continue
		}
		n, err := kitInstance.AddMCPServer(ctx, name, cfg)
		if err != nil {
			log.Warn("acp: MCP server connection failed", "server", name, "error", err)
			continue
		}
		log.Debug("acp: MCP server connected", "server", name, "tools", n)
	}

	sess.kit = kitInstance
	sess.sessionID = sessionID
	if opts.setup != nil {
		opts.setup(sess)
	}

	r.mu.Lock()
	old := r.sessions[sessionID]
	r.sessions[sessionID] = sess
	r.mu.Unlock()

	// Opening a session that is already active replaces the old instance.
	if old != nil {
		old.cancelPrompt()
		if old.kit != nil {
			_ = old.kit.Close()
		}
	}

	return sess, nil
}

// get retrieves a session by ACP session ID.
func (r *sessionRegistry) get(sessionID string) (*acpSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[sessionID]
	return s, ok
}

// closeAll closes all sessions.
func (r *sessionRegistry) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, sess := range r.sessions {
		if sess.kit != nil {
			_ = sess.kit.Close()
		}
		delete(r.sessions, id)
	}
}

// remove closes and removes a single session by ID.
func (r *sessionRegistry) remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sess, ok := r.sessions[sessionID]
	if !ok {
		return
	}
	if sess.kit != nil {
		_ = sess.kit.Close()
	}
	delete(r.sessions, sessionID)
}

// cancelPrompt cancels the current prompt for a session, if any.
func (s *acpSession) cancelPrompt() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
}

// setCancel stores a cancel function for the current prompt.
func (s *acpSession) setCancel(cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelFn = cancel
}

// clearCancel clears the stored cancel function (called when prompt completes).
func (s *acpSession) clearCancel() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelFn = nil
}

// authRequired returns the ACP "authentication required" error (-32000) for
// missing provider credentials, with steps to fix it. Kit offers no ACP
// auth methods: credentials come from Kit's own config.
func authRequired(err error) *acp.RequestError {
	msg := fmt.Sprintf("%v. Run 'kit auth login <provider>' or set the provider's API key environment variable, then restart 'kit acp'.", err)
	return acp.NewAuthRequired(map[string]any{"message": msg})
}
