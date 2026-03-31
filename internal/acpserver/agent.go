// Package acpserver implements a Kit-backed ACP (Agent Client Protocol) agent.
//
// It bridges Kit's LLM execution, tool system, and session management to the
// ACP protocol over stdio, allowing ACP clients (such as OpenCode) to drive
// Kit as a remote coding agent.
package acpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Version is injected at build time; fallback to "dev".
var Version = "dev"

// thinkingTagOpen and thinkingTagClose are the XML-style tags that some models
// (Qwen, DeepSeek) wrap reasoning content in. We parse these to extract
// reasoning/thinking content and send it as ACP thought updates.
// Also support <think> format used by some models.
const (
	thinkingTagOpen    = "<thinking>"
	thinkingTagClose   = "</thinking>"
	shortThinkTagOpen  = "<think>"
	shortThinkTagClose = "</think>"
)

// Agent implements the acp.Agent interface, delegating to Kit for LLM
// execution, tool calls, and session management.
type Agent struct {
	conn     *acp.AgentSideConnection
	registry *sessionRegistry

	// toolCallCounter provides unique IDs for tool calls within a turn.
	toolCallCounter atomic.Int64

	// inThinkingTag tracks whether we're currently inside a <thinking> tag
	// when parsing streaming content from models that wrap reasoning in XML tags.
	inThinkingTag bool
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

	// Extract text and file attachments from prompt content blocks.
	promptText, files := extractPromptContent(params.Prompt)
	if promptText == "" && len(files) == 0 {
		return acp.PromptResponse{}, acp.NewInvalidParams("empty prompt")
	}

	// If we have files but no text prompt, add a default prompt
	// This is required because the underlying LLM library needs a non-empty prompt
	// when there are no previous messages in the conversation.
	if promptText == "" && len(files) > 0 {
		promptText = "Please analyze the attached file."
	}

	log.Debug("acp: prompt", "session", sessionID, "prompt_len", len(promptText), "files", len(files))

	// Reset thinking tag state for this new prompt turn
	a.inThinkingTag = false

	// Create a cancellable context for this prompt turn.
	promptCtx, cancel := context.WithCancel(ctx)
	sess.setCancel(cancel)
	defer sess.clearCancel()

	// Subscribe to Kit events and stream them as ACP session updates.
	unsub := a.subscribeEvents(promptCtx, sess.kit, params.SessionId)
	defer unsub()

	// Run the prompt through Kit's full turn lifecycle.
	// Use PromptResultWithFiles when file attachments are present.
	var err error
	if len(files) > 0 {
		_, err = sess.kit.PromptResultWithFiles(promptCtx, promptText, files)
	} else {
		_, err = sess.kit.PromptResult(promptCtx, promptText)
	}
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
			// Handle models that wrap reasoning in <thinking> tags (Qwen, DeepSeek)
			// Parse the chunk and separate reasoning from regular text
			reasoning, text := a.parseThinkingTags(ev.Chunk)

			// Send reasoning update if we have reasoning content
			if reasoning != "" {
				u := acp.UpdateAgentThoughtText(reasoning)
				_ = a.conn.SessionUpdate(ctx, acp.SessionNotification{
					SessionId: sessionID,
					Update:    u,
				})
			}

			// Send text update if we have text content
			if text != "" {
				u := acp.UpdateAgentMessageText(text)
				update = &u
			}

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
					Filename:  "image.png",
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
					Filename:  "audio.wav",
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

// parseThinkingTags parses a text chunk for <thinking> or  tags and separates
// reasoning content from regular text. This handles models (Qwen, DeepSeek)
// that wrap reasoning in XML-style tags instead of using proper reasoning events.
// Returns (reasoningContent, textContent).
func (a *Agent) parseThinkingTags(chunk string) (reasoning string, text string) {
	// Handle empty chunk
	if chunk == "" {
		return "", ""
	}

	// Determine which tag format to use (long or short)
	openTag := thinkingTagOpen
	closeTag := thinkingTagClose

	if strings.Contains(chunk, shortThinkTagOpen) || strings.Contains(chunk, shortThinkTagClose) {
		openTag = shortThinkTagOpen
		closeTag = shortThinkTagClose
	} else if !strings.Contains(chunk, thinkingTagOpen) && !strings.Contains(chunk, thinkingTagClose) && !a.inThinkingTag {
		// No tags at all and not in thinking mode - return as text
		return "", chunk
	}

	// Check for opening tag
	if strings.Contains(chunk, openTag) {
		parts := strings.SplitN(chunk, openTag, 2)

		// Content before the opening tag is regular text
		if !a.inThinkingTag && parts[0] != "" {
			text = parts[0]
		}

		a.inThinkingTag = true

		// Content after the opening tag is reasoning
		if len(parts) > 1 {
			// Check if the same chunk contains the closing tag
			if strings.Contains(parts[1], closeTag) {
				innerParts := strings.SplitN(parts[1], closeTag, 2)
				reasoning = innerParts[0]
				a.inThinkingTag = false

				// Content after closing tag is regular text
				if len(innerParts) > 1 && innerParts[1] != "" {
					text += innerParts[1]
				}
			} else if parts[1] != "" {
				// No closing tag yet, all remaining content is reasoning
				reasoning = parts[1]
			}
		}
		return reasoning, text
	}

	// Check for closing tag
	if strings.Contains(chunk, closeTag) {
		parts := strings.SplitN(chunk, closeTag, 2)
		a.inThinkingTag = false

		// Content before closing tag is reasoning
		reasoning = parts[0]

		// Content after closing tag is regular text
		if len(parts) > 1 && parts[1] != "" {
			text = parts[1]
		}
		return reasoning, text
	}

	// No tags found - content goes to current mode
	if a.inThinkingTag {
		return chunk, ""
	}
	return "", chunk
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

// extractFilenameFromURI extracts a filename from a file URI or path.
func extractFilenameFromURI(uri string) string {
	// Handle file:// URIs
	uri = strings.TrimPrefix(uri, "file://")
	// Extract basename
	if idx := strings.LastIndex(uri, "/"); idx >= 0 {
		return uri[idx+1:]
	}
	return uri
}

// readResourceFromURI attempts to read file content from a file:// URI.
func readResourceFromURI(uri string) ([]byte, error) {
	if !strings.HasPrefix(uri, "file://") {
		return nil, fmt.Errorf("unsupported URI scheme: %s", uri)
	}
	path := uri[7:] // Remove file:// prefix
	return os.ReadFile(path)
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
