package ui

import (
	"fmt"
	"strings"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// jobController is optional so alternate app controllers can omit job support.
type jobController interface {
	Jobs() []kit.Job
	GetJob(string) (kit.Job, error)
	CancelJob(string) error
}

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
		job, err := ctrl.GetJob(args[0])
		if err != nil {
			m.printSystemMessage(err.Error())
			return
		}
		m.printSystemMessage(fmt.Sprintf("%s  %s  %s\n%s\n%s", job.ID, job.Kind, job.Status, job.Output, job.Error))
		return
	}
	jobs := ctrl.Jobs()
	if len(jobs) == 0 {
		m.printSystemMessage("No background jobs.")
		return
	}
	var text strings.Builder
	running := 0
	for _, job := range jobs {
		if job.Status == kit.JobRunning {
			running++
		}
		fmt.Fprintf(&text, "%s  %s  %s\n", job.ID, job.Kind, job.Status)
	}
	fmt.Fprintf(&text, "%d running. Use /jobs <job-id> for output or /cancel-job <job-id> to stop work.", running)
	m.printSystemMessage(text.String())
}

func (m *AppModel) handleCancelJobCommand(args []string) {
	ctrl, ok := m.appCtrl.(jobController)
	if !ok {
		m.printSystemMessage("Background jobs are not available.")
		return
	}
	if len(args) != 1 {
		m.printSystemMessage("Usage: /cancel-job <job-id>")
		return
	}
	if err := ctrl.CancelJob(args[0]); err != nil {
		m.printSystemMessage(err.Error())
		return
	}
	m.printSystemMessage("Cancellation requested for job " + args[0] + ".")
}

func (m *AppModel) renderJobStatus() string {
	ctrl, ok := m.appCtrl.(jobController)
	if !ok {
		return ""
	}
	count := 0
	for _, job := range ctrl.Jobs() {
		if job.Status == kit.JobRunning {
			count++
		}
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf("Jobs: %d running · /jobs · /cancel-job <id>", count)
}

// renderBackgroundStatus is shared by layout measurement and View.
func (m *AppModel) renderBackgroundStatus() string {
	view := m.renderSubagentStatusBar()
	if m.width <= 0 {
		return view
	}
	if jobs := m.renderJobStatus(); jobs != "" {
		if view != "" {
			view += "\n"
		}
		view += truncateLine(jobs, m.width)
	}
	return view
}
