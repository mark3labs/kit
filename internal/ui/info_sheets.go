package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// The info commands show their content in a sheetModal. Each body is built
// on render, from live state, so a sheet stays current while it is open.

// infoMinColW is the minimum column width of the key/value sections in the
// info sheets.
const infoMinColW = 44

// infoGrid lays out key/value sections with no cap on description lines:
// values such as file paths and job output must show in full.
func infoGrid(secs []helpSection, width int, s helpStyles) []string {
	return layoutHelpGrid(secs, width, infoMinColW, 0, s)
}

// infoEmpty is the body of a sheet with nothing to list.
func infoEmpty(text string, width int, s helpStyles) []string {
	return sheetText(text, width, s.note, s)
}

// plural returns "1 tool" or "3 tools".
func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// groupDigits formats n with thousands separators.
func groupDigits(n int) string {
	str := strconv.Itoa(n)
	neg := strings.HasPrefix(str, "-")
	str = strings.TrimPrefix(str, "-")
	var b strings.Builder
	for i, r := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// openToolsSheet shows the available tools.
func (m *AppModel) openToolsSheet() {
	m.openSheet("◆ Tools", plural(len(m.toolNames), "tool")+" available to the model.", sheetInfoMaxW,
		func(width int, s helpStyles) []string {
			if len(m.toolNames) == 0 {
				return infoEmpty("No tools are currently available.", width, s)
			}
			return sheetNameGrid(m.toolNames, width, s)
		})
}

// openServersSheet shows the configured MCP servers.
func (m *AppModel) openServersSheet() {
	m.openSheet("◆ MCP Servers", plural(len(m.serverNames), "server")+" configured.", sheetInfoMaxW,
		func(width int, s helpStyles) []string {
			if len(m.serverNames) == 0 {
				return infoEmpty("No MCP servers are currently configured.", width, s)
			}
			return sheetNameGrid(m.serverNames, width, s)
		})
}

// openUsageSheet shows token usage and cost for the last turn and the
// session.
func (m *AppModel) openUsageSheet() {
	if m.usageTracker == nil {
		m.printSystemMessage("Usage tracking is not available for this model.")
		return
	}
	m.openSheet("◆ Usage", "Token usage and cost. Use /reset-usage to start the count again.", sheetInfoMaxW,
		func(width int, s helpStyles) []string {
			if m.usageTracker == nil {
				return infoEmpty("Usage tracking is not available for this model.", width, s)
			}
			sess := m.usageTracker.GetSessionStats()
			secs := []helpSection{}
			if turn := m.usageTracker.GetTurnStats(); turn != nil {
				secs = append(secs, helpSection{Title: "Last turn", Entries: []helpEntry{
					{Key: "Input", Desc: groupDigits(turn.InputTokens) + " tokens"},
					{Key: "Output", Desc: groupDigits(turn.OutputTokens) + " tokens"},
					{Key: "Cache read", Desc: groupDigits(turn.CacheReadTokens) + " tokens"},
					{Key: "Cache write", Desc: groupDigits(turn.CacheWriteTokens) + " tokens"},
					{Key: "Cost", Desc: fmt.Sprintf("$%.6f", turn.TotalCost)},
				}})
			}
			secs = append(secs, helpSection{Title: "Session", Entries: []helpEntry{
				{Key: "Input", Desc: groupDigits(sess.TotalInputTokens) + " tokens"},
				{Key: "Output", Desc: groupDigits(sess.TotalOutputTokens) + " tokens"},
				{Key: "Cache read", Desc: groupDigits(sess.TotalCacheReadTokens) + " tokens"},
				{Key: "Cache write", Desc: groupDigits(sess.TotalCacheWriteTokens) + " tokens"},
				{Key: "Cost", Desc: fmt.Sprintf("$%.6f", sess.TotalCost)},
				{Key: "Requests", Desc: groupDigits(sess.RequestCount)},
			}})
			return infoGrid(secs, width, s)
		})
}

// handleSessionInfoCommand shows the session details.
func (m *AppModel) handleSessionInfoCommand() tea.Cmd {
	if _, ok := m.appCtrl.SessionSnapshot(); !ok {
		m.printSystemMessage("No tree session active.")
		return nil
	}
	m.openSheet("◆ Session", "", sheetInfoMaxW, func(width int, s helpStyles) []string {
		snap, ok := m.appCtrl.SessionSnapshot()
		if !ok {
			return infoEmpty("No tree session active.", width, s)
		}
		entries := []helpEntry{}
		if snap.Name != "" {
			entries = append(entries, helpEntry{Key: "Name", Desc: snap.Name})
		}
		entries = append(entries,
			helpEntry{Key: "ID", Desc: snap.ID},
			helpEntry{Key: "File", Desc: snap.FilePath},
			helpEntry{Key: "Working dir", Desc: snap.Cwd},
			helpEntry{Key: "Created", Desc: snap.Created.Format(time.RFC3339)},
			helpEntry{Key: "Entries", Desc: groupDigits(snap.EntryCount)},
			helpEntry{Key: "Messages", Desc: groupDigits(snap.MessageCount)},
			helpEntry{Key: "Current leaf", Desc: snap.LeafID},
		)
		// One section, one column: the values are long paths and IDs.
		return layoutHelpGrid([]helpSection{{Title: "Details", Entries: entries}}, width, width, 0, s)
	})
	return nil
}

// handleShortcutsCommand shows the keyboard shortcuts that extensions
// registered, grouped by extension.
func (m *AppModel) handleShortcutsCommand() tea.Cmd {
	m.openSheet("◆ Extension Shortcuts", "Keys that loaded extensions registered. /help lists the built-in keys.", sheetInfoMaxW,
		func(width int, s helpStyles) []string {
			if m.getShortcutList == nil {
				return infoEmpty("No extensions loaded, so no shortcuts are registered.", width, s)
			}
			secs := m.shortcutSections()
			if len(secs) == 0 {
				return infoEmpty("No extension shortcuts are registered.", width, s)
			}
			return infoGrid(secs, width, s)
		})
	return nil
}

// handleJobsCommand shows the retained jobs, or one job and its output.
// Unsupported access, invalid arguments and lookup errors go to the
// scrollback, since there is no sheet to show.
func (m *AppModel) handleJobsCommand(args []string) {
	ctrl, ok := m.appCtrl.(jobController)
	if !ok {
		m.printSystemMessage("Background jobs are not available.")
		return
	}
	if len(args) > 1 {
		m.printSystemMessage("Usage: /jobs [job-id]")
		return
	}
	if len(args) == 1 {
		id := args[0]
		if _, err := ctrl.GetJob(id); err != nil {
			m.printSystemMessage(err.Error())
			return
		}
		m.openSheet("◆ Job "+id, "Use /cancel-job "+id+" to stop it.", sheetInfoMaxW,
			func(width int, s helpStyles) []string {
				job, err := ctrl.GetJob(id)
				if err != nil {
					return infoEmpty(err.Error(), width, s)
				}
				return jobBody(job, width, s)
			})
		return
	}
	m.openSheet("◆ Background Jobs", "Use /jobs <job-id> for output or /cancel-job <job-id> to stop work.", sheetInfoMaxW,
		func(width int, s helpStyles) []string {
			jobs := ctrl.Jobs()
			if len(jobs) == 0 {
				return infoEmpty("No background jobs.", width, s)
			}
			running := 0
			entries := make([]helpEntry, 0, len(jobs))
			for _, job := range jobs {
				if job.Status == kit.JobRunning {
					running++
				}
				entries = append(entries, helpEntry{
					Key:  job.ID,
					Desc: fmt.Sprintf("%s  %s", job.Kind, job.Status),
					Note: jobTime(job),
				})
			}
			title := fmt.Sprintf("%s · %d running", plural(len(jobs), "job"), running)
			return layoutHelpGrid([]helpSection{{Title: title, Entries: entries}}, width, width, 0, s)
		})
}

// jobTime returns when a job started, and when it finished if it has.
func jobTime(job kit.Job) string {
	if job.StartedAt.IsZero() {
		return ""
	}
	t := "started " + job.StartedAt.Format(time.Kitchen)
	if !job.FinishedAt.IsZero() {
		t += ", took " + job.FinishedAt.Sub(job.StartedAt).Round(time.Second).String()
	}
	return t
}

// jobBody renders one job: its details, its output and its error.
func jobBody(job kit.Job, width int, s helpStyles) []string {
	entries := []helpEntry{
		{Key: "Kind", Desc: job.Kind},
		{Key: "Status", Desc: string(job.Status)},
	}
	if !job.StartedAt.IsZero() {
		entries = append(entries, helpEntry{Key: "Started", Desc: job.StartedAt.Format(time.RFC3339)})
	}
	if !job.FinishedAt.IsZero() {
		entries = append(entries, helpEntry{Key: "Finished", Desc: job.FinishedAt.Format(time.RFC3339)})
	}
	body := layoutHelpGrid([]helpSection{{Title: "Details", Entries: entries}}, width, width, 0, s)

	blank := s.bg.Render(strings.Repeat(" ", width))
	body = append(body, blank, helpGroupHeading("Output", width, s), blank)
	if out := sheetText(job.Output, width, s.desc, s); len(out) > 0 {
		body = append(body, out...)
	} else {
		body = append(body, infoEmpty("(no output yet)", width, s)...)
	}
	if job.Error != "" {
		body = append(body, blank, helpGroupHeading("Error", width, s), blank)
		body = append(body, sheetText(job.Error, width, s.desc, s)...)
	}
	return body
}
