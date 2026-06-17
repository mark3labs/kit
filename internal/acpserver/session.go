package acpserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/log"
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
	// Each ACP session gets its own isolated config store (CLI is left nil) so
	// per-session SetModel / SetThinkingLevel calls cannot race or bleed across
	// the sessionRegistry. We seed the relevant root-command flag values from
	// the process-global store (which cobra populated from flags) so launching
	// `kit acp -m <model> [--thinking-level ...] [--provider-url ...]` is still
	// honored; .kit.yml and KIT_* env vars are loaded per session by kit.New.
	streamOn := true
	kitInstance, err := kit.New(ctx, &kit.Options{
		SessionDir:     cwd,
		Quiet:          true,
		Streaming:      &streamOn,
		Model:          viper.GetString("model"),
		ThinkingLevel:  viper.GetString("thinking-level"),
		ProviderURL:    viper.GetString("provider-url"),
		ProviderAPIKey: viper.GetString("provider-api-key"),
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
