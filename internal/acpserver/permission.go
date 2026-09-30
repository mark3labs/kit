package acpserver

import (
	"fmt"
	"slices"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// Tool approval modes. A client changes the mode of a session through the
// "approval" config option; `kit acp --approval` sets the default.
const (
	// approvalAsk asks the client before every tool that can change
	// something: edits, commands, subagents, MCP and extension tools.
	approvalAsk = "ask"
	// approvalAutoEdit allows file edits without asking, and asks before
	// commands and other tools.
	approvalAutoEdit = "auto_edit"
	// approvalAuto runs every tool without asking.
	approvalAuto = "auto"
)

// ApprovalModes lists the valid approval modes, in the order clients show
// them.
var ApprovalModes = []string{approvalAsk, approvalAutoEdit, approvalAuto}

// validApproval reports whether mode is a known approval mode.
func validApproval(mode string) bool {
	return slices.Contains(ApprovalModes, mode)
}

// Permission option IDs sent in session/request_permission.
const (
	optAllowOnce    = "allow_once"
	optAllowAlways  = "allow_always"
	optRejectOnce   = "reject_once"
	optRejectAlways = "reject_always"
)

// readOnlyTools never need approval: they cannot change anything.
var readOnlyTools = map[string]bool{
	"read":           true,
	"ls":             true,
	"grep":           true,
	"find":           true,
	planToolName:     true,
	"activate_skill": true,
}

// needsApproval reports whether a tool call must be approved by the user in
// the given mode.
func needsApproval(mode, toolName string) bool {
	if readOnlyTools[toolName] {
		return false
	}
	switch mode {
	case approvalAuto:
		return false
	case approvalAutoEdit:
		return toolName != "edit" && toolName != "write"
	}
	return true
}

// installPermissionHook makes every tool call of the session go through the
// approval mode, and asks the client with session/request_permission when
// the mode requires it. A rejected call does not run; the model gets the
// rejection as the tool result.
func (a *Agent) installPermissionHook(sess *acpSession) {
	sess.kit.OnBeforeToolCall(kit.HookPriorityHigh, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
		if h.ToolName == planToolName {
			// Shown as a plan, not as a tool call.
			return nil
		}
		id := sess.toolIDs.lookup(h.ToolCallID)
		if reason := a.checkPermission(sess, id, h.ToolName, h.ToolArgs); reason != "" {
			return &kit.BeforeToolCallResult{Block: true, Reason: reason}
		}
		// The tool runs now: tell the client.
		a.sendUpdate(sess, acp.UpdateToolCall(id, acp.WithUpdateStatus(acp.ToolCallStatusInProgress)))
		return nil
	})
}

// checkPermission returns "" when the tool call may run, or the reason it
// may not.
func (a *Agent) checkPermission(sess *acpSession, id acp.ToolCallId, toolName, toolArgs string) string {
	sess.mu.Lock()
	mode := sess.approval
	allowed := sess.alwaysAllow[toolName]
	rejected := sess.alwaysReject[toolName]
	sess.mu.Unlock()

	if rejected {
		return fmt.Sprintf("the user does not allow the %s tool in this session", toolName)
	}
	if allowed || !needsApproval(mode, toolName) {
		return ""
	}

	args := parseToolArgs(toolArgs)
	title := toolTitle(toolName, args)
	kind := acpToolKind(toolName)
	status := acp.ToolCallStatusPending
	update := acp.ToolCallUpdate{
		ToolCallId: id,
		Title:      &title,
		Kind:       &kind,
		Status:     &status,
		Locations:  toolLocations(toolName, args, sess.cwd),
	}
	if args != nil {
		update.RawInput = args
	}

	ctx := sess.context()
	resp, err := a.conn.RequestPermission(ctx, acp.RequestPermissionRequest{
		SessionId: sess.id(),
		ToolCall:  update,
		Options: []acp.PermissionOption{
			{OptionId: optAllowOnce, Name: "Allow", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: optAllowAlways, Name: fmt.Sprintf("Always allow %s", toolName), Kind: acp.PermissionOptionKindAllowAlways},
			{OptionId: optRejectOnce, Name: "Reject", Kind: acp.PermissionOptionKindRejectOnce},
			{OptionId: optRejectAlways, Name: fmt.Sprintf("Always reject %s", toolName), Kind: acp.PermissionOptionKindRejectAlways},
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			return "the turn was cancelled"
		}
		log.Warn("acp: permission request failed", "tool", toolName, "error", err)
		return fmt.Sprintf("permission request failed: %v", err)
	}

	if resp.Outcome.Selected == nil {
		// The spec's "cancelled" outcome: the user cancelled the turn.
		return "the turn was cancelled"
	}
	switch string(resp.Outcome.Selected.OptionId) {
	case optAllowOnce:
		return ""
	case optAllowAlways:
		sess.mu.Lock()
		sess.alwaysAllow[toolName] = true
		sess.mu.Unlock()
		return ""
	case optRejectAlways:
		sess.mu.Lock()
		sess.alwaysReject[toolName] = true
		sess.mu.Unlock()
	}
	return "the user rejected this tool call"
}

// sendUpdate sends one session update for the session. Errors are logged:
// a lost update must not stop the turn.
func (a *Agent) sendUpdate(sess *acpSession, update acp.SessionUpdate) {
	if err := a.conn.SessionUpdate(sess.context(), acp.SessionNotification{
		SessionId: sess.id(),
		Update:    update,
	}); err != nil {
		log.Debug("acp: session update failed", "session", sess.sessionID, "error", err)
	}
}
