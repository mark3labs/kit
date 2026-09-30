// Package acpserver implements a Kit-backed ACP (Agent Client Protocol) agent.
//
// It bridges Kit's LLM execution, tool system, and session management to the
// ACP protocol over stdio, allowing ACP clients (such as OpenCode) to drive
// Kit as a remote coding agent.
package acpserver

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"

	"github.com/mark3labs/kit/internal/session"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// Version is injected at build time; fallback to "dev".
var Version = "dev"

// listPageSize is the number of sessions session/list returns per page.
const listPageSize = 100

// Agent implements the ACP agent interface on top of Kit's LLM execution,
// tool calls, and session management.
type Agent struct {
	conn     *acp.AgentSideConnection
	registry *sessionRegistry
}

// Compile-time checks that Agent implements the stable agent interfaces.
var (
	_ acp.Agent       = (*Agent)(nil)
	_ acp.AgentLoader = (*Agent)(nil)
)

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

// Authenticate handles authentication requests. Kit advertises no auth
// methods (provider credentials come from Kit's own config), so this is a
// no-op.
func (a *Agent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}

// Logout handles logout requests. Kit does not advertise the logout
// capability, so this is a no-op.
func (a *Agent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, nil
}

// Initialize negotiates the protocol version and capabilities. Kit supports
// only protocol version 1, so it answers with version 1 for every request:
// the spec requires the agent to reply with the requested version when it
// supports it, and with its latest version otherwise.
func (a *Agent) Initialize(_ context.Context, params acp.InitializeRequest) (acp.InitializeResponse, error) {
	log.Debug("acp: initialize", "protocol_version", params.ProtocolVersion)

	return acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.ProtocolVersionNumber),
		AgentCapabilities: acp.AgentCapabilities{
			LoadSession: true,
			McpCapabilities: acp.McpCapabilities{
				Http: true,
				Sse:  true,
			},
			PromptCapabilities: acp.PromptCapabilities{
				EmbeddedContext: true,
				Image:           true,
			},
			SessionCapabilities: acp.SessionCapabilities{
				Close:  &acp.SessionCloseCapabilities{},
				List:   &acp.SessionListCapabilities{},
				Resume: &acp.SessionResumeCapabilities{},
			},
		},
		AgentInfo: &acp.Implementation{
			Name:    "kit",
			Title:   ptr("Kit"),
			Version: Version,
		},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

// NewSession creates a new Kit session for the given working directory.
func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	if err := validateCwd(params.Cwd); err != nil {
		return acp.NewSessionResponse{}, err
	}

	log.Debug("acp: new_session", "cwd", params.Cwd, "mcp_servers", len(params.McpServers))

	sess, err := a.registry.create(ctx, sessionOptions{
		cwd:        params.Cwd,
		mcpServers: params.McpServers,
	})
	if err != nil {
		log.Error("acp: session creation failed", "cwd", params.Cwd, "error", err)
		return acp.NewSessionResponse{}, fmt.Errorf("create session: %w", err)
	}

	return acp.NewSessionResponse{
		SessionId:     acp.SessionId(sess.sessionID),
		ConfigOptions: configOptions(sess.kit),
	}, nil
}

// LoadSession opens a persisted session, replays its conversation to the
// client as session/update notifications, and then responds.
func (a *Agent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	sess, err := a.openSession(ctx, params.SessionId, params.Cwd, params.McpServers)
	if err != nil {
		return acp.LoadSessionResponse{}, err
	}
	if err := a.replayHistory(ctx, sess, params.SessionId); err != nil {
		return acp.LoadSessionResponse{}, err
	}
	return acp.LoadSessionResponse{ConfigOptions: configOptions(sess.kit)}, nil
}

// ResumeSession opens a persisted session without replaying its history.
func (a *Agent) ResumeSession(ctx context.Context, params acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	sess, err := a.openSession(ctx, params.SessionId, params.Cwd, params.McpServers)
	if err != nil {
		return acp.ResumeSessionResponse{}, err
	}
	// The spec forbids a replay here, but the client may still show the old
	// tool calls, so register their IDs to keep new ones unique.
	_ = historyUpdates(sess.kit.GetStructuredMessages(), sess.cwd, sess.toolIDs)
	return acp.ResumeSessionResponse{ConfigOptions: configOptions(sess.kit)}, nil
}

// openSession finds a persisted session by ID and opens it with a new Kit
// instance. An active instance of the same session is closed first so that
// two instances never write the same session file.
func (a *Agent) openSession(ctx context.Context, sessionID acp.SessionId, cwd string, mcpServers []acp.McpServer) (*acpSession, error) {
	if err := validateCwd(cwd); err != nil {
		return nil, err
	}
	id := string(sessionID)
	if id == "" {
		return nil, acp.NewInvalidParams("sessionId is required")
	}

	path, err := session.FindSessionPathByID(cwd, id)
	if err != nil {
		return nil, resourceNotFound(fmt.Sprintf("session not found: %s", id))
	}

	log.Debug("acp: open session", "session", id, "cwd", cwd, "path", path)

	if old, ok := a.registry.get(id); ok {
		old.cancelPrompt()
		a.registry.remove(id)
	}

	sess, err := a.registry.create(ctx, sessionOptions{
		cwd:         cwd,
		sessionPath: path,
		mcpServers:  mcpServers,
	})
	if err != nil {
		return nil, fmt.Errorf("open session: %w", err)
	}
	if sess.sessionID != id {
		a.registry.remove(sess.sessionID)
		return nil, fmt.Errorf("open session: file %s has session ID %s, want %s", path, sess.sessionID, id)
	}
	return sess, nil
}

// ListSessions lists the persisted Kit sessions, newest first. When the
// client sends a cwd, only sessions of that directory are listed. Subagent
// sessions are internal and are not listed.
func (a *Agent) ListSessions(_ context.Context, params acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	var (
		infos []kit.SessionInfo
		err   error
	)
	if params.Cwd != nil && *params.Cwd != "" {
		if err := validateCwd(*params.Cwd); err != nil {
			return acp.ListSessionsResponse{}, err
		}
		infos, err = kit.ListSessions(*params.Cwd)
	} else {
		infos, err = kit.ListAllSessions()
	}
	if err != nil {
		return acp.ListSessionsResponse{}, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]acp.SessionInfo, 0, len(infos))
	for _, info := range infos {
		if info.ParentSessionID != "" || info.ID == "" {
			continue
		}
		si := acp.SessionInfo{
			SessionId: acp.SessionId(info.ID),
			Cwd:       info.Cwd,
		}
		if title := sessionTitle(info); title != "" {
			si.Title = &title
		}
		if !info.Modified.IsZero() {
			ts := info.Modified.UTC().Format(time.RFC3339)
			si.UpdatedAt = &ts
		}
		sessions = append(sessions, si)
	}

	// The cursor is the offset of the next page, encoded so that clients
	// treat it as opaque.
	offset := 0
	if params.Cursor != nil && *params.Cursor != "" {
		offset, err = decodeCursor(*params.Cursor)
		if err != nil || offset > len(sessions) {
			return acp.ListSessionsResponse{}, acp.NewInvalidParams("invalid cursor")
		}
	}
	end := min(offset+listPageSize, len(sessions))
	resp := acp.ListSessionsResponse{Sessions: sessions[offset:end]}
	if end < len(sessions) {
		next := encodeCursor(end)
		resp.NextCursor = &next
	}
	return resp, nil
}

// Prompt handles the main agent execution. It subscribes to Kit's event bus,
// converts events to ACP session updates, and runs the prompt through Kit's
// full turn lifecycle (hooks, LLM, tool calls, persistence).
func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	sessionID := string(params.SessionId)
	sess, ok := a.registry.get(sessionID)
	if !ok {
		return acp.PromptResponse{}, resourceNotFound(fmt.Sprintf("session not found: %s", sessionID))
	}

	// Extract text and file attachments from prompt content blocks.
	promptText, files := extractPromptContent(params.Prompt)
	if promptText == "" && len(files) == 0 {
		return acp.PromptResponse{}, acp.NewInvalidParams("empty prompt")
	}

	// The LLM library needs a non-empty text prompt when the conversation
	// has no previous messages, so add one for file-only prompts.
	if promptText == "" && len(files) > 0 {
		promptText = "Please analyze the attached file."
	}

	log.Debug("acp: prompt", "session", sessionID, "prompt_len", len(promptText), "files", len(files))

	// Create a cancellable context for this prompt turn. The SDK also
	// cancels ctx when it receives session/cancel or $/cancel_request.
	promptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sess.setCancel(cancel)
	defer sess.clearCancel()

	// Subscribe to Kit events and stream them as ACP session updates. The
	// subscription ends before this method returns, so no update can follow
	// the session/prompt response.
	unsub := a.subscribeEvents(promptCtx, sess, params.SessionId)
	defer unsub()

	// Run the prompt through Kit's full turn lifecycle.
	var (
		result *kit.TurnResult
		err    error
	)
	if len(files) > 0 {
		result, err = sess.kit.PromptResultWithFiles(promptCtx, promptText, files)
	} else {
		result, err = sess.kit.PromptResult(promptCtx, promptText)
	}

	// The spec requires the cancelled stop reason (not an error) for a
	// cancelled turn, even when the cancellation surfaced as an error.
	if promptCtx.Err() != nil {
		return acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
	}
	if err != nil {
		return acp.PromptResponse{}, fmt.Errorf("prompt failed: %w", err)
	}

	return acp.PromptResponse{StopReason: stopReason(result)}, nil
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

// SetSessionMode rejects every request: Kit advertises no session modes (it
// uses session config options instead), so no mode ID is valid.
func (a *Agent) SetSessionMode(_ context.Context, params acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, acp.NewInvalidParams(
		fmt.Sprintf("unknown mode %q: Kit has no session modes, use session/set_config_option", params.ModeId),
	)
}

// CloseSession cancels any ongoing work for the session and frees its resources.
func (a *Agent) CloseSession(_ context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	sessionID := string(params.SessionId)
	sess, ok := a.registry.get(sessionID)
	if !ok {
		return acp.CloseSessionResponse{}, nil
	}

	log.Debug("acp: close session", "session", sessionID)
	sess.cancelPrompt()
	a.registry.remove(sessionID)
	return acp.CloseSessionResponse{}, nil
}

// SetSessionConfigOption changes a session config option ("model" or
// "thinking_level") and responds with the full, updated option list.
func (a *Agent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	if params.ValueId == nil {
		// Kit advertises only select options, so boolean values are invalid.
		return acp.SetSessionConfigOptionResponse{}, acp.NewInvalidParams("only select config options are supported")
	}
	sessionID := string(params.ValueId.SessionId)
	configID := string(params.ValueId.ConfigId)
	value := string(params.ValueId.Value)

	sess, ok := a.registry.get(sessionID)
	if !ok {
		return acp.SetSessionConfigOptionResponse{}, resourceNotFound(fmt.Sprintf("session not found: %s", sessionID))
	}

	log.Debug("acp: set_session_config_option", "session", sessionID, "config", configID, "value", value)

	if err := applyConfigOption(ctx, sess.kit, configID, value); err != nil {
		return acp.SetSessionConfigOptionResponse{}, err
	}

	return acp.SetSessionConfigOptionResponse{ConfigOptions: configOptions(sess.kit)}, nil
}

// ---------------------------------------------------------------------------
// Event streaming: Kit events → ACP SessionUpdate notifications
// ---------------------------------------------------------------------------

// subscribeEvents subscribes to Kit's event bus and forwards events as ACP
// session update notifications to the client.
func (a *Agent) subscribeEvents(ctx context.Context, sess *acpSession, sessionID acp.SessionId) func() {
	return sess.kit.Subscribe(func(e kit.Event) {
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
			tcID := sess.toolIDs.start(ev.ToolCallID)
			args := ev.ParsedArgs
			if args == nil {
				args = parseToolArgs(ev.ToolArgs)
			}
			opts := []acp.ToolCallStartOpt{
				acp.WithStartKind(acpToolKind(ev.ToolName)),
				acp.WithStartStatus(acp.ToolCallStatusPending),
			}
			if args != nil {
				opts = append(opts, acp.WithStartRawInput(args))
			}
			if locs := toolLocations(ev.ToolName, args, sess.cwd); len(locs) > 0 {
				opts = append(opts, acp.WithStartLocations(locs))
			}
			u := acp.StartToolCall(tcID, toolTitle(ev.ToolName, args), opts...)
			update = &u

		case kit.ToolExecutionStartEvent:
			u := acp.UpdateToolCall(sess.toolIDs.lookup(ev.ToolCallID),
				acp.WithUpdateStatus(acp.ToolCallStatusInProgress),
			)
			update = &u

		case kit.ToolResultEvent:
			tcID := sess.toolIDs.lookup(ev.ToolCallID)
			status := acp.ToolCallStatusCompleted
			if ev.IsError {
				status = acp.ToolCallStatusFailed
			}
			u := acp.UpdateToolCall(tcID,
				acp.WithUpdateStatus(status),
				acp.WithUpdateContent(toolResultContent(ev.Result, ev.Metadata)),
			)
			update = &u

			// kit.ToolCallContentEvent is ignored on purpose: it repeats text
			// that MessageUpdateEvent already streamed (streaming is always
			// on in ACP mode), so forwarding it duplicated agent messages.
		}

		if update != nil {
			if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: sessionID,
				Update:    *update,
			}); err != nil {
				log.Debug("acp: session update failed", "session", sessionID, "error", err)
			}
		}
	})
}

// stopReason maps Kit's provider finish reason to an ACP stop reason.
func stopReason(result *kit.TurnResult) acp.StopReason {
	if result == nil {
		return acp.StopReasonEndTurn
	}
	switch result.StopReason {
	case kit.FinishReasonLength:
		return acp.StopReasonMaxTokens
	case kit.FinishReasonContentFilter:
		return acp.StopReasonRefusal
	case kit.FinishReasonToolCalls:
		// The turn ended while the model still wanted to call tools: Kit
		// stopped the loop at its max-steps limit.
		return acp.StopReasonMaxTurnRequests
	}
	return acp.StopReasonEndTurn
}

// validateCwd checks the cwd of session/new, session/load, session/resume
// and session/list. The spec requires an absolute path.
func validateCwd(cwd string) error {
	if cwd == "" {
		return acp.NewInvalidParams("cwd is required")
	}
	if !filepath.IsAbs(cwd) {
		return acp.NewInvalidParams(fmt.Sprintf("cwd must be an absolute path: %s", cwd))
	}
	return nil
}

// resourceNotFound returns the ACP "resource not found" error (-32002).
func resourceNotFound(msg string) *acp.RequestError {
	return &acp.RequestError{Code: -32002, Message: "Resource not found", Data: msg}
}

// sessionTitle returns the display title of a persisted session.
func sessionTitle(info kit.SessionInfo) string {
	title := strings.TrimSpace(info.Name)
	if title == "" {
		title = strings.TrimSpace(info.FirstMessage)
	}
	if i := strings.IndexByte(title, '\n'); i >= 0 {
		title = title[:i]
	}
	const maxLen = 100
	if r := []rune(title); len(r) > maxLen {
		title = string(r[:maxLen]) + "…"
	}
	return title
}

// encodeCursor and decodeCursor convert a list offset to an opaque cursor.
func encodeCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte("offset:" + strconv.Itoa(offset)))
}

func decodeCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, err
	}
	n, ok := strings.CutPrefix(string(raw), "offset:")
	if !ok {
		return 0, fmt.Errorf("bad cursor")
	}
	offset, err := strconv.Atoi(n)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("bad cursor")
	}
	return offset, nil
}

func ptr[T any](v T) *T { return &v }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractPromptContent extracts text and file attachments from ACP content blocks.
// It converts supported content blocks (image, audio, resource) to Kit's LLMFilePart.
func extractPromptContent(blocks []acp.ContentBlock) (string, []kit.LLMFilePart) {
	var textParts []string
	var files []kit.LLMFilePart

	log.Debug("acp: extracting content", "blocks", len(blocks))

	for i, block := range blocks {
		switch {
		// Text content
		case block.Text != nil:
			log.Debug("acp: content block", "index", i, "type", "text", "len", len(block.Text.Text))
			textParts = append(textParts, block.Text.Text)

		// Image data (base64)
		case block.Image != nil:
			mimeType := block.Image.MimeType
			if mimeType == "" {
				mimeType = "image/png" // Default fallback
			}
			log.Debug("acp: content block", "index", i, "type", "image", "mime", mimeType, "data_len", len(block.Image.Data))
			if data, err := base64.StdEncoding.DecodeString(block.Image.Data); err == nil {
				files = append(files, kit.LLMFilePart{
					Filename:  mediaFilename("image", mimeType),
					Data:      data,
					MediaType: mimeType,
				})
			} else {
				log.Debug("acp: failed to decode image", "error", err)
			}

		// Audio data (base64)
		case block.Audio != nil:
			mimeType := block.Audio.MimeType
			if mimeType == "" {
				mimeType = "audio/wav" // Default fallback
			}
			log.Debug("acp: content block", "index", i, "type", "audio", "mime", mimeType)
			if data, err := base64.StdEncoding.DecodeString(block.Audio.Data); err == nil {
				files = append(files, kit.LLMFilePart{
					Filename:  mediaFilename("audio", mimeType),
					Data:      data,
					MediaType: mimeType,
				})
			} else {
				log.Debug("acp: failed to decode audio", "error", err)
			}

		// Embedded resource (text or binary file content)
		case block.Resource != nil:
			log.Debug("acp: content block", "index", i, "type", "resource")
			res := block.Resource.Resource
			// Text resource - append as text content with file reference
			if res.TextResourceContents != nil {
				uri := res.TextResourceContents.Uri
				content := res.TextResourceContents.Text
				mimeType := "text/plain"
				if res.TextResourceContents.MimeType != nil {
					mimeType = *res.TextResourceContents.MimeType
				}
				log.Debug("acp: text resource", "uri", uri, "mime", mimeType, "len", len(content))
				// Text files are included as formatted text, NOT as FilePart
				// FilePart is for binary files (images, audio, PDFs) only
				textParts = append(textParts, fmt.Sprintf("[File: %s]\n```\n%s\n```", uri, content))
			}
			// Binary resource (base64 blob) - these become FilePart
			if res.BlobResourceContents != nil {
				uri := res.BlobResourceContents.Uri
				mimeType := "application/octet-stream"
				if res.BlobResourceContents.MimeType != nil {
					mimeType = *res.BlobResourceContents.MimeType
				}
				log.Debug("acp: binary resource", "uri", uri, "mime", mimeType, "blob_len", len(res.BlobResourceContents.Blob))
				if data, err := base64.StdEncoding.DecodeString(res.BlobResourceContents.Blob); err == nil {
					files = append(files, kit.LLMFilePart{
						Filename:  extractFilenameFromURI(uri),
						Data:      data,
						MediaType: mimeType,
					})
				} else {
					log.Debug("acp: failed to decode binary resource", "error", err)
				}
			}

		// Resource link (file reference without embedded content)
		case block.ResourceLink != nil:
			uri := block.ResourceLink.Uri
			name := block.ResourceLink.Name
			log.Debug("acp: content block", "index", i, "type", "resource_link", "uri", uri, "name", name)
			// For resource links, we'll try to read the file from disk
			// This requires the file URI to be accessible (file:// scheme)
			if content, err := readResourceFromURI(uri); err == nil {
				// Detect if it's a text file or binary file
				mimeType := "text/plain"
				if block.ResourceLink.MimeType != nil {
					mimeType = *block.ResourceLink.MimeType
				}
				log.Debug("acp: resource link loaded", "uri", uri, "mime", mimeType, "size", len(content))

				// Only create FilePart for binary files (images, audio, PDFs, etc.)
				// Text files are included as formatted text in the message
				if isTextMimeType(mimeType) || looksLikeText(content) {
					textParts = append(textParts, fmt.Sprintf("[File: %s]\n```\n%s\n```", uri, string(content)))
				} else {
					// Binary file - create FilePart for models that support it
					files = append(files, kit.LLMFilePart{
						Filename:  extractFilenameFromURI(uri),
						Data:      content,
						MediaType: mimeType,
					})
				}
			} else {
				// If we can't read it, include as a text reference
				log.Debug("acp: resource link failed to load", "uri", uri, "error", err)
				textParts = append(textParts, fmt.Sprintf("[Referenced file: %s]", uri))
			}

		default:
			log.Debug("acp: content block", "index", i, "type", "unknown/unhandled")
		}
	}

	// Debug log the extracted content
	for i, f := range files {
		log.Debug("acp: extracted file", "index", i, "filename", f.Filename, "mime", f.MediaType, "size", len(f.Data))
	}

	return strings.Join(textParts, "\n"), files
}

// isTextMimeType returns true if the MIME type indicates text content.
func isTextMimeType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		mimeType == "application/json" ||
		mimeType == "application/xml" ||
		mimeType == "application/javascript" ||
		mimeType == "application/typescript" ||
		mimeType == "application/x-sh" ||
		mimeType == "application/x-python" ||
		mimeType == "application/x-yaml" ||
		mimeType == "application/x-toml"
}

// looksLikeText checks if the content appears to be text (not binary).
// It samples the first 512 bytes and checks for null bytes or high
// concentration of non-printable characters.
func looksLikeText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	// Check first 512 bytes (or less if file is smaller)
	sampleSize := min(len(data), 512)
	sample := data[:sampleSize]

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range sample {
		// Null byte indicates binary
		if b == 0 {
			return false
		}
		// Count control characters (except common whitespace)
		if b < 32 && b != '\n' && b != '\r' && b != '\t' {
			nonPrintable++
		}
	}

	// If more than 30% non-printable, consider it binary
	return float64(nonPrintable)/float64(sampleSize) < 0.3
}

// extractFilenameFromURI returns the base name of a file URI or path.
func extractFilenameFromURI(uri string) string {
	if u, err := url.Parse(uri); err == nil && u.Scheme != "" {
		if name := path.Base(u.Path); name != "." && name != "/" {
			return name
		}
		return uri
	}
	return filepath.Base(uri)
}

// readResourceFromURI reads the content of a file:// URI. The path part may
// be percent-encoded, as RFC 8089 requires for special characters.
func readResourceFromURI(uri string) ([]byte, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI %q: %w", uri, err)
	}
	if u.Scheme != "file" {
		return nil, fmt.Errorf("unsupported URI scheme: %s", uri)
	}
	if u.Host != "" && u.Host != "localhost" {
		return nil, fmt.Errorf("unsupported remote file URI: %s", uri)
	}
	return os.ReadFile(filepath.FromSlash(u.Path))
}

// mediaFilename returns a file name for an attachment with no name, using
// the extension that matches its MIME type.
func mediaFilename(base, mimeType string) string {
	_, sub, ok := strings.Cut(mimeType, "/")
	if !ok || sub == "" {
		return base
	}
	sub, _, _ = strings.Cut(sub, ";")
	sub = strings.TrimPrefix(sub, "x-")
	switch sub {
	case "jpeg":
		sub = "jpg"
	case "svg+xml":
		sub = "svg"
	case "mpeg":
		sub = "mp3"
	}
	return base + "." + sub
}
