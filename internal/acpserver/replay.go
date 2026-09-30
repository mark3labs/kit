package acpserver

import (
	"context"
	"encoding/base64"
	"fmt"

	acp "github.com/coder/acp-go-sdk"

	"github.com/mark3labs/kit/internal/message"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// replayHistory streams the conversation of the current session branch to
// the client as session/update notifications. session/load MUST do this
// before it responds.
func (a *Agent) replayHistory(ctx context.Context, sess *acpSession, sessionID acp.SessionId) error {
	for _, update := range historyUpdates(sess.kit.GetStructuredMessages(), sess.cwd, sess.toolIDs) {
		if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: sessionID,
			Update:    update,
		}); err != nil {
			return fmt.Errorf("replay history: %w", err)
		}
	}
	return nil
}

// historyUpdates converts stored messages into the session updates a client
// would have seen live: user and agent message chunks, thought chunks, and a
// tool_call / tool_call_update pair for each tool invocation. Tool call IDs
// go through ids, so calls made after the replay stay unique.
func historyUpdates(msgs []kit.StructuredMessage, cwd string, ids *toolIDMapper) []acp.SessionUpdate {
	var updates []acp.SessionUpdate
	for _, msg := range msgs {
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case kit.TextContent:
				if p.Text == "" {
					continue
				}
				switch msg.Role {
				case kit.RoleUser:
					updates = append(updates, acp.UpdateUserMessageText(p.Text))
				case kit.RoleAssistant:
					updates = append(updates, acp.UpdateAgentMessageText(p.Text))
				}

			case message.ImageContent:
				if msg.Role != kit.RoleUser || len(p.Data) == 0 {
					continue
				}
				updates = append(updates, acp.UpdateUserMessage(
					acp.ImageBlock(base64.StdEncoding.EncodeToString(p.Data), p.MediaType),
				))

			case kit.ReasoningContent:
				if p.Thinking != "" {
					updates = append(updates, acp.UpdateAgentThoughtText(p.Thinking))
				}

			case kit.ToolCall:
				args := parseToolArgs(p.Input)
				opts := []acp.ToolCallStartOpt{
					acp.WithStartKind(acpToolKind(p.Name)),
					acp.WithStartStatus(acp.ToolCallStatusInProgress),
				}
				if args != nil {
					opts = append(opts, acp.WithStartRawInput(args))
				}
				if locs := toolLocations(p.Name, args, cwd); len(locs) > 0 {
					opts = append(opts, acp.WithStartLocations(locs))
				}
				updates = append(updates, acp.StartToolCall(
					ids.start(p.ID), toolTitle(p.Name, args), opts...,
				))

			case kit.ToolResult:
				status := acp.ToolCallStatusCompleted
				if p.IsError {
					status = acp.ToolCallStatusFailed
				}
				updates = append(updates, acp.UpdateToolCall(ids.lookup(p.ToolCallID),
					acp.WithUpdateStatus(status),
					acp.WithUpdateContent(toolResultContent(p.Content, nil)),
				))
			}
		}
	}
	return updates
}
