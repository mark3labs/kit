package acpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// planToolName is the ACP-only tool the model uses to publish its plan.
// Codex uses the same name for the same purpose.
const planToolName = "update_plan"

// planInput is the input of the update_plan tool.
type planInput struct {
	Entries []planEntryInput `json:"entries" description:"The complete plan. Send every step each time, not only the changed ones."`
}

type planEntryInput struct {
	Content  string `json:"content" description:"What this step does, in one short sentence"`
	Status   string `json:"status" enum:"pending,in_progress,completed" description:"Step status. Keep at most one step in_progress."`
	Priority string `json:"priority,omitempty" enum:"high,medium,low" description:"Step priority (default medium)"`
}

const planToolDescription = `Show your plan for the current task to the user, as a list of steps with a status each.

Use it for tasks with three or more distinct steps. Send the complete plan every time: the new list replaces the old one. Mark a step in_progress when you start it and completed as soon as it is done. Do not use it for simple one-step requests.`

// planTool returns the update_plan tool of a session. Each call sends an ACP
// plan update, which the client shows as the agent's plan.
func (a *Agent) planTool(sess *acpSession) kit.Tool {
	return kit.NewTool(planToolName, planToolDescription,
		func(ctx context.Context, in planInput) (kit.ToolOutput, error) {
			entries, err := planEntries(in)
			if err != nil {
				return kit.ToolOutput{Content: err.Error(), IsError: true}, nil
			}
			if err := a.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: sess.id(),
				Update:    acp.UpdatePlan(entries...),
			}); err != nil {
				return kit.ToolOutput{Content: fmt.Sprintf("could not show the plan: %v", err), IsError: true}, nil
			}
			return kit.ToolOutput{Content: fmt.Sprintf("Plan updated (%d steps).", len(entries))}, nil
		})
}

// planEntries validates the tool input and converts it to ACP plan entries.
func planEntries(in planInput) ([]acp.PlanEntry, error) {
	entries := make([]acp.PlanEntry, 0, len(in.Entries))
	for i, e := range in.Entries {
		content := strings.TrimSpace(e.Content)
		if content == "" {
			return nil, fmt.Errorf("step %d has no content", i+1)
		}
		status := acp.PlanEntryStatus(e.Status)
		switch status {
		case acp.PlanEntryStatusPending, acp.PlanEntryStatusInProgress, acp.PlanEntryStatusCompleted:
		case "":
			status = acp.PlanEntryStatusPending
		default:
			return nil, fmt.Errorf("step %d has an unknown status %q (use pending, in_progress or completed)", i+1, e.Status)
		}
		priority := acp.PlanEntryPriority(e.Priority)
		switch priority {
		case acp.PlanEntryPriorityHigh, acp.PlanEntryPriorityMedium, acp.PlanEntryPriorityLow:
		default:
			priority = acp.PlanEntryPriorityMedium
		}
		entries = append(entries, acp.PlanEntry{Content: content, Status: status, Priority: priority})
	}
	return entries, nil
}

// planUpdateFromArgs rebuilds the plan update of a stored update_plan call,
// for history replay. It returns false when the arguments are not valid.
func planUpdateFromArgs(args string) (acp.SessionUpdate, bool) {
	var in planInput
	if err := json.Unmarshal([]byte(args), &in); err != nil {
		return acp.SessionUpdate{}, false
	}
	entries, err := planEntries(in)
	if err != nil {
		return acp.SessionUpdate{}, false
	}
	return acp.UpdatePlan(entries...), true
}
