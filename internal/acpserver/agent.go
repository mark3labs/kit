// Package acpserver implements a Kit-backed ACP (Agent Client Protocol) agent.
//
// It bridges Kit's LLM execution, tool system, and session management to the
// ACP protocol over stdio, allowing ACP clients (such as OpenCode) to drive
// Kit as a remote coding agent.
package acpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Version is injected at build time; fallback to "dev".
var Version = "dev"

// Agent implements the acp.Agent interface, delegating to Kit for LLM
// execution, tool calls, and session management.
type Agent struct {
	conn     *acp.AgentSideConnection
	registry *sessionRegistry

	// toolCallCounter provides unique IDs for tool calls within a turn.
	toolCallCounter atomic.Int64
}

// NewAgent creates a new ACP agent backed by Kit.
func NewAgent() *Agent {
	return &Agent{
		registry: newSessionRegistry(),
	}
}

// SetAgentConnection stores the connection so the agent can send session
// updates (streaming, tool calls, etc.) back to the ACP client. This follows
// the AgentConnAware duck-typing pattern from the SDK.
func (a *Agent) SetAgentConnection(conn *acp.AgentSideConnection) {
	a.conn = conn
}

// Close shuts down all active sessions.
func (a *Agent) Close() {
	a.registry.closeAll()
}

// ---------------------------------------------------------------------------
// acp.Agent interface implementation
// ---------------------------------------------------------------------------

// Authenticate handles authentication requests. Kit doesn't require auth for
// local stdio usage, so this is a no-op.
func (a *Agent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// Initialize negotiates capabilities with the ACP client.
func (a *Agent) Initialize(_ context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	log.Debug("acp: initialize", "protocol_version", params.ProtocolVersion)

	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(1),
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			PromptCapabilities: acp.PromptCapabilities{
				EmbeddedContext: true,
				Image:           true,
			},
		},
		AgentInfo: &acp.Implementation{
			Name:    "Kit",
			Version: Version,
		},
	}, nil
}

// NewSession creates a new Kit session for the given working directory.
func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	cwd := params.Cwd
	if cwd == "" {
		return acp.NewSessionResponse{}, acp.NewInvalidParams("cwd is required")
	}

	log.Debug("acp: new_session", "cwd", cwd)

	sess, err := a.registry.create(ctx, cwd)
	if err != nil {
		log.Error("acp: session creation failed", "cwd", cwd, "error", err)
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}

	return acp.NewSessionResponse{
		SessionId: acp.SessionId(sess.sessionID),
	}, nil
}

// Prompt handles the main agent execution. It subscribes to Kit's event bus,
// converts events to ACP session updates, and runs the prompt through Kit's
// full turn lifecycle (hooks, LLM, tool calls, persistence).
func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sessionID := string(params.SessionId)
	sess, ok := a.registry.get(sessionID)
	if !ok {
		return acp.PromptResponse{}, acp.NewInvalidParams(
			fmt.Sprintf("session not found: %s", sessionID),
		)
	}

	// Extract text from prompt content blocks.
	promptText := extractPromptText(params.Prompt)
	if promptText == "" {
		return acp.PromptResponse{}, acp.NewInvalidParams("empty prompt")
	}

	log.Debug("acp: prompt", "session", sessionID, "prompt_len", len(promptText))

	// Create a cancellable context for this prompt turn.
	promptCtx, cancel := context.WithCancel(ctx)
	sess.setCancel(cancel)
	defer sess.clearCancel()

	// Subscribe to Kit events and stream them as ACP session updates.
	unsub := a.subscribeEvents(promptCtx, sess.kit, params.SessionId)
	defer unsub()

	// Run the prompt through Kit's full turn lifecycle.
	_, err := sess.kit.PromptResult(promptCtx, promptText)
	if err != nil {
		if promptCtx.Err() != nil {
			return acp.PromptResponse{
				StopReason: acp.StopReasonCancelled,
			}, nil
		}
		return acp.PromptResponse{}, fmt.Errorf("prompt failed: %w", err)
	}

	return acp.PromptResponse{
		StopReason: acp.StopReasonEndTurn,
	}, nil
}

// Cancel cancels the ongoing prompt for a session.
func (a *Agent) Cancel(_ context.Context, params acp.CancelNotification) error {
	sessionID := string(params.SessionId)
	sess, ok := a.registry.get(sessionID)
	if !ok {
		return nil // No-op if session doesn't exist.
	}

	log.Debug("acp: cancel", "session", sessionID)
	sess.cancelPrompt()
	return nil
}

// SetSessionMode is a no-op for now — Kit doesn't have built-in session modes.
func (a *Agent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

// ---------------------------------------------------------------------------
// Event streaming: Kit events → ACP SessionUpdate notifications
// ---------------------------------------------------------------------------

// subscribeEvents subscribes to Kit's event bus and forwards events as ACP
// session update notifications to the client.
func (a *Agent) subscribeEvents(ctx context.Context, k *kit.Kit, sessionID acp.SessionId) func() {
	return k.Subscribe(func(e kit.Event) {
		// Don't send updates after the context is cancelled.
		if ctx.Err() != nil {
			return
		}

		var update *acp.SessionUpdate
		switch ev := e.(type) {
		case kit.MessageUpdateEvent:
			u := acp.UpdateAgentMessageText(ev.Chunk)
			update = &u

		case kit.ReasoningDeltaEvent:
			u := acp.UpdateAgentThoughtText(ev.Delta)
			update = &u

		case kit.ToolCallEvent:
			tcID := acp.ToolCallId(ev.ToolCallID)
			if tcID == "" {
				tcID = acp.ToolCallId(fmt.Sprintf("tc_%d", a.toolCallCounter.Add(1)))
			}
			u := acp.StartToolCall(tcID, ev.ToolName,
				acp.WithStartStatus(acp.ToolCallStatusInProgress),
				acp.WithStartRawInput(parseToolArgs(ev.ToolArgs)),
			)
			update = &u

		case kit.ToolResultEvent:
			tcID := acp.ToolCallId(ev.ToolCallID)
			if tcID == "" {
				tcID = acp.ToolCallId(fmt.Sprintf("tc_%d", a.toolCallCounter.Load()))
			}
			status := acp.ToolCallStatusCompleted
			if ev.IsError {
				status = acp.ToolCallStatusFailed
			}
			u := acp.UpdateToolCall(tcID,
				acp.WithUpdateStatus(status),
				acp.WithUpdateContent([]acp.ToolCallContent{
					acp.ToolContent(acp.TextBlock(ev.Result)),
				}),
			)
			update = &u

		case kit.ToolCallContentEvent:
			u := acp.UpdateAgentMessageText(ev.Content)
			update = &u
		}

		if update != nil {
			_ = a.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: sessionID,
				Update:    *update,
			})
		}
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractPromptText extracts the concatenated text content from ACP content
// blocks. Non-text blocks are ignored for now.
func extractPromptText(blocks []acp.ContentBlock) string {
	var text string
	for _, block := range blocks {
		if block.Text != nil {
			if text != "" {
				text += "\n"
			}
			text += block.Text.Text
		}
	}
	return text
}

// parseToolArgs attempts to parse a JSON tool args string into a map for
// structured display. Falls back to a simple string wrapper.
func parseToolArgs(args string) any {
	if args == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err == nil {
		return m
	}
	return map[string]any{"input": args}
}
