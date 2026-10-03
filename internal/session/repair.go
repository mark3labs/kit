package session

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/mark3labs/kit/internal/message"
)

// Recovery of interrupted tool calls.
//
// Kit persists a step incrementally: the assistant message (with its tool
// calls) is one append, the tool-role message (with the results) the next. A
// process that dies between the two writes leaves the tail of the branch
// holding tool calls that never got a result. A transcript in that state is
// not resumable — providers reject a conversation that has a tool call with
// no matching result — and BuildContext would faithfully hand exactly such a
// conversation to the next request.
//
// On open, the tree manager walks the tail of the current branch (leaf back
// to the nearest user message or compaction, which is where a turn starts)
// and collects every tool call that no tool message in between answered. For
// each one it appends a synthetic tool result marked as an error, so the
// session opens in a state the next LLM request accepts. The repair is a
// plain append: nothing already stored is rewritten.
//
// Interrupted turns the app chose to abandon go through a different path
// (the UI re-parents the incomplete turn off-branch), so repair only ever
// sees the output of a process that died mid-step.

// interruptedResultText is the content of a synthetic tool result for a call
// whose process died before the result was recorded.
const interruptedResultText = "[interrupted] The process running this session stopped before the tool produced a result. Nothing was executed, or the result was lost. Re-run the tool if its work is still needed."

// repairInterruptedToolCalls finds tool calls left unanswered at the tail of
// the current branch and appends one tool message with a synthetic error
// result for each. It returns the number of calls repaired. Must be called
// before the append handle is used again; safe on in-memory sessions, where
// it is a no-op because they have no crash window.
func (tm *TreeManager) repairInterruptedToolCalls() (int, error) {
	tm.mu.Lock()
	locked := true
	defer func() {
		if locked {
			tm.mu.Unlock()
		}
	}()

	if tm.leafID == "" {
		return 0, nil
	}

	// Walk leaf -> root until a turn boundary. pending keeps the unanswered
	// call IDs in branch order (root -> leaf): new calls are prepended as
	// the walk moves backward, and answered ones are removed when their
	// result message is seen.
	type callRef struct{ id, name string }
	var pending []callRef
	answered := make(map[string]bool)

walk:
	for current := tm.leafID; current != ""; {
		entry, ok := tm.index[current]
		if !ok {
			break
		}
		me, ok := entry.(*MessageEntry)
		if !ok {
			// A compaction is the other turn boundary: everything before it
			// is out of the LLM context, so calls answered before it stay
			// answered and calls left pending there are NOT repaired — a
			// synthetic result after the compaction would reference a tool
			// call the next request does not contain, and providers reject
			// that too. Model changes, labels, extension data and the like
			// neither call tools nor answer them.
			if _, isCompaction := entry.(*CompactionEntry); isCompaction {
				break walk
			}
			current = tm.entryParentID(entry)
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			current = me.ParentID
			continue
		}
		switch msg.Role {
		case message.RoleTool:
			for _, r := range msg.ToolResults() {
				answered[r.ToolCallID] = true
			}
			current = me.ParentID
		case message.RoleAssistant:
			calls := msg.ToolCalls()
			// Prepend so pending stays in branch order.
			prepended := make([]callRef, 0, len(pending)+len(calls))
			for _, c := range calls {
				prepended = append(prepended, callRef{id: c.ID, name: c.Name})
			}
			pending = append(prepended, pending...)
			current = me.ParentID
		case message.RoleUser:
			// A user message starts a turn: nothing older can be awaiting a
			// result from the crashed turn.
			break walk
		default: // system and anything else: keep walking
			current = me.ParentID
		}
	}

	var orphaned []callRef
	for _, c := range pending {
		if !answered[c.id] {
			orphaned = append(orphaned, c)
		}
	}
	if len(orphaned) == 0 {
		return 0, nil
	}

	// One tool message carries every synthetic result, mirroring how kit
	// writes all results of one step as a single message.
	parts := make([]message.ContentPart, 0, len(orphaned))
	for _, c := range orphaned {
		parts = append(parts, message.ToolResult{
			ToolCallID: c.id,
			Name:       c.name,
			Content:    interruptedResultText,
			IsError:    true,
		})
	}
	resultMsg := message.Message{
		Role:  message.RoleTool,
		Parts: parts,
	}
	tm.mu.Unlock()
	locked = false
	ids, err := tm.AppendStep(context.Background(), resultMsg.ToLLMMessages())
	if err != nil {
		return 0, fmt.Errorf("persist interrupted tool repair: %w", err)
	}

	log.Warn("session: repaired tool calls left unanswered by a stopped process",
		"calls", len(orphaned), "entry", ids[0])
	return len(orphaned), nil
}
