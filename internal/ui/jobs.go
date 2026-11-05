package ui

import (
	"fmt"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// jobController is optional so alternate app controllers can omit job support.
type jobController interface {
	Jobs() []kit.Job
	GetJob(string) (kit.Job, error)
	CancelJob(string) error
}

// handleCancelJobCommand requests cancellation of one job and reports usage, availability, or controller errors.
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

// renderJobStatus returns the running-job count and command hints, or empty text when no jobs run.
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
