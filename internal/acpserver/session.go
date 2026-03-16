package acpserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"

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

// create creates a new Kit instance with a persisted tree session for the
// given working directory. The Kit-generated session ID is used as the ACP
// session ID so the mapping is 1:1.
func (r *sessionRegistry) create(ctx context.Context, cwd string) (*acpSession, error) {
	kitInstance, err := kit.New(ctx, &kit.Options{
		SessionDir: cwd,
		Quiet:      true,
		Streaming:  true,
	})
	if err != nil {
		// Provide actionable guidance for provider auth errors, which are
		// the most common failure mode when running via ACP.
		msg := err.Error()
		if strings.Contains(msg, "API key") || strings.Contains(msg, "credentials") || strings.Contains(msg, "OAuth") {
			return nil, fmt.Errorf("provider authentication failed: %w — run 'kit auth login <provider>' or set the appropriate environment variable before starting 'kit acp'", err)
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
	// become no-ops or return cancelled; all data/model/tool APIs work
	// identically to interactive mode.
	if kitInstance.HasExtensions() {
		kitInstance.SetExtensionContext(extensions.Context{
			SessionID:   sessionID,
			CWD:         cwd,
			Model:       kitInstance.GetModelString(),
			Interactive: false,

			// Output — route through structured logger.
			Print:      func(text string) { log.Debug("extension: print", "text", text) },
			PrintInfo:  func(text string) { log.Info("extension: info", "text", text) },
			PrintError: func(text string) { log.Error("extension: error", "text", text) },
			PrintBlock: func(opts extensions.PrintBlockOpts) {
				log.Info("extension: block", "subtitle", opts.Subtitle, "text", opts.Text)
			},

			// Message injection — no-ops for now; ACP clients drive prompts.
			SendMessage:   func(string) {},
			CancelAndSend: func(string) {},
			Exit:          func() {},

			// TUI widgets/chrome — silent no-ops (no TUI in ACP).
			SetWidget:       func(extensions.WidgetConfig) {},
			RemoveWidget:    func(string) {},
			SetHeader:       func(extensions.HeaderFooterConfig) {},
			RemoveHeader:    func() {},
			SetFooter:       func(extensions.HeaderFooterConfig) {},
			RemoveFooter:    func() {},
			SetEditor:       func(extensions.EditorConfig) {},
			ResetEditor:     func() {},
			SetEditorText:   func(string) {},
			SetUIVisibility: func(extensions.UIVisibility) {},
			SetStatus:       func(string, string, int) {},
			RemoveStatus:    func(string) {},

			// Interactive prompts — return cancelled (no user to prompt).
			PromptSelect: func(extensions.PromptSelectConfig) extensions.PromptSelectResult {
				return extensions.PromptSelectResult{Cancelled: true}
			},
			PromptConfirm: func(extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
				return extensions.PromptConfirmResult{Cancelled: true}
			},
			PromptInput: func(extensions.PromptInputConfig) extensions.PromptInputResult {
				return extensions.PromptInputResult{Cancelled: true}
			},
			ShowOverlay: func(extensions.OverlayConfig) extensions.OverlayResult {
				return extensions.OverlayResult{Cancelled: true, Index: -1}
			},
			SuspendTUI: func(callback func()) error { callback(); return nil },

			// Data access — delegate to Kit instance.
			GetContextStats: func() extensions.ContextStats {
				s := kitInstance.GetContextStats()
				return extensions.ContextStats{
					EstimatedTokens: s.EstimatedTokens,
					ContextLimit:    s.ContextLimit,
					UsagePercent:    s.UsagePercent,
					MessageCount:    s.MessageCount,
				}
			},
			GetMessages:    func() []extensions.SessionMessage { return kitInstance.GetSessionMessages() },
			GetSessionPath: func() string { return kitInstance.GetSessionFilePath() },
			AppendEntry: func(entryType, data string) (string, error) {
				return kitInstance.AppendExtensionEntry(entryType, data)
			},
			GetEntries: func(entryType string) []extensions.ExtensionEntry {
				return kitInstance.GetExtensionEntries(entryType)
			},

			// Options, model, and tool management.
			GetOption: func(name string) string { return kitInstance.GetExtensionOption(name) },
			SetOption: func(name, value string) { kitInstance.SetExtensionOption(name, value) },
			SetModel: func(modelString string) error {
				previousModel := kitInstance.GetExtensionContext().Model
				if err := kitInstance.SetModel(context.Background(), modelString); err != nil {
					return err
				}
				kitInstance.UpdateExtensionContextModel(modelString)
				kitInstance.EmitModelChange(modelString, previousModel, "extension")
				return nil
			},
			GetAvailableModels: func() []extensions.ModelInfoEntry { return kitInstance.GetAvailableModels() },
			EmitCustomEvent:    func(name, data string) { kitInstance.EmitExtensionCustomEvent(name, data) },
			GetAllTools:        func() []extensions.ToolInfo { return kitInstance.GetExtensionToolInfos() },
			SetActiveTools:     func(names []string) { kitInstance.SetExtensionActiveTools(names) },

			// LLM completions and subagents.
			Complete: func(req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
				return kitInstance.ExecuteCompletion(context.Background(), req)
			},
			SpawnSubagent: func(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
				sdkCfg := kit.SubagentConfig{
					Prompt:       config.Prompt,
					Model:        config.Model,
					SystemPrompt: config.SystemPrompt,
					Timeout:      config.Timeout,
					NoSession:    config.NoSession,
				}
				if config.OnEvent != nil {
					sdkCfg.OnEvent = func(e kit.Event) {
						se := sdkEventToSubagentEvent(e)
						if se.Type != "" {
							config.OnEvent(se)
						}
					}
				}
				result, err := kitInstance.Subagent(context.Background(), sdkCfg)
				if result == nil {
					return nil, &extensions.SubagentResult{Error: err}, err
				}
				extResult := &extensions.SubagentResult{
					Response:  result.Response,
					Error:     result.Error,
					SessionID: result.SessionID,
					Elapsed:   result.Elapsed,
				}
				if result.Usage != nil {
					extResult.Usage = &extensions.SubagentUsage{
						InputTokens:  result.Usage.InputTokens,
						OutputTokens: result.Usage.OutputTokens,
					}
				}
				return nil, extResult, err
			},

			// Render — fall back to logging.
			RenderMessage: func(name, content string) {
				renderer := kitInstance.GetExtensionMessageRenderer(name)
				if renderer != nil && renderer.Render != nil {
					content = renderer.Render(content, 80)
				}
				log.Info("extension: message", "renderer", name, "content", content)
			},
			ReloadExtensions: func() error { return kitInstance.ReloadExtensions() },
		})
		kitInstance.EmitSessionStart()
	}

	sess := &acpSession{
		kit:       kitInstance,
		cwd:       cwd,
		sessionID: sessionID,
	}

	r.mu.Lock()
	r.sessions[sessionID] = sess
	r.mu.Unlock()

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

// sdkEventToSubagentEvent converts an SDK event to an extension SubagentEvent.
func sdkEventToSubagentEvent(e kit.Event) extensions.SubagentEvent {
	switch ev := e.(type) {
	case kit.MessageUpdateEvent:
		return extensions.SubagentEvent{Type: "text", Content: ev.Chunk}
	case kit.ReasoningDeltaEvent:
		return extensions.SubagentEvent{Type: "reasoning", Content: ev.Delta}
	case kit.ToolCallEvent:
		return extensions.SubagentEvent{
			Type: "tool_call", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind, ToolArgs: ev.ToolArgs,
		}
	case kit.ToolExecutionStartEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_start", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolExecutionEndEvent:
		return extensions.SubagentEvent{
			Type: "tool_execution_end", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
		}
	case kit.ToolResultEvent:
		return extensions.SubagentEvent{
			Type: "tool_result", ToolCallID: ev.ToolCallID,
			ToolName: ev.ToolName, ToolKind: ev.ToolKind,
			ToolResult: ev.Result, IsError: ev.IsError,
		}
	case kit.TurnStartEvent:
		return extensions.SubagentEvent{Type: "turn_start"}
	case kit.TurnEndEvent:
		return extensions.SubagentEvent{Type: "turn_end"}
	default:
		return extensions.SubagentEvent{}
	}
}
