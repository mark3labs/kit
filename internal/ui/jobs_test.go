package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/core"
	kit "github.com/mark3labs/kit/pkg/kit"
)

type jobsTestController struct {
	stubAppController
	jobs      []kit.Job
	cancelled string
}

func (c *jobsTestController) Jobs() []kit.Job { return c.jobs }
func (c *jobsTestController) GetJob(id string) (kit.Job, error) {
	for _, job := range c.jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return kit.Job{}, fmt.Errorf("unknown job %s", id)
}
func (c *jobsTestController) CancelJob(id string) error {
	if _, err := c.GetJob(id); err != nil {
		return err
	}
	c.cancelled = id
	return nil
}

func TestJobCommands(t *testing.T) {
	ctrl := &jobsTestController{jobs: []kit.Job{{ID: "job-1", Kind: "shell", Status: kit.JobRunning, Output: "test output"}}}
	m, _, _ := newTestAppModel(ctrl)
	for _, name := range []string{"/jobs", "/cancel-job"} {
		if commands.GetCommandByName(name) == nil {
			t.Fatalf("command missing: %s", name)
		}
	}
	m = sendMsg(m, core.SubmitMsg{Text: "/jobs"})
	if text := lastKillMessage(m); !strings.Contains(text, "job-1") || !strings.Contains(text, "1 running") {
		t.Fatalf("list: %q", text)
	}
	m = sendMsg(m, core.SubmitMsg{Text: "/jobs job-1"})
	if text := lastKillMessage(m); !strings.Contains(text, "test output") {
		t.Fatalf("output: %q", text)
	}
	m = sendMsg(m, core.SubmitMsg{Text: "/cancel-job job-1"})
	if ctrl.cancelled != "job-1" {
		t.Fatal("job not cancelled")
	}
	m = sendMsg(m, core.SubmitMsg{Text: "/cancel-job"})
	if text := lastKillMessage(m); !strings.Contains(text, "Usage:") {
		t.Fatalf("usage: %q", text)
	}
	m = sendMsg(m, core.SubmitMsg{Text: "/jobs unknown"})
	if text := lastKillMessage(m); !strings.Contains(text, "unknown job") {
		t.Fatalf("error: %q", text)
	}
	m = sendMsg(m, app.JobsUpdatedEvent{})
	if !strings.Contains(m.renderJobStatus(), "1 running") {
		t.Fatal("missing status count")
	}
	ctrl.jobs[0].Status = kit.JobCompleted
	if m.renderJobStatus() != "" {
		t.Fatal("completed job counted as running")
	}
}

// TestJobsUpdatedInvalidatesModalLayout ensures job status changes invalidate
// cached heights before a prompt or overlay can consume the event.
func TestJobsUpdatedInvalidatesModalLayout(t *testing.T) {
	for _, mode := range []string{"input", "prompt", "overlay"} {
		t.Run(mode, func(t *testing.T) {
			ctrl := &jobsTestController{}
			m, _, _ := newTestAppModel(ctrl)
			switch mode {
			case "prompt":
				m = sendMsg(m, app.PromptRequestEvent{
					PromptType: "confirm", Message: "Continue?",
					ResponseCh: make(chan app.PromptResponse, 1),
				})
				if m.state != statePrompt || m.prompt == nil {
					t.Fatal("prompt did not open")
				}
			case "overlay":
				m = sendMsg(m, app.OverlayRequestEvent{
					Title: "Jobs", Content: "Job details",
					ResponseCh: make(chan app.OverlayResponse, 1),
				})
				if m.state != stateOverlay || m.overlay == nil {
					t.Fatal("overlay did not open")
				}
			}
			state := m.state
			for _, jobs := range [][]kit.Job{
				{{ID: "job-1", Status: kit.JobRunning}},
				nil,
			} {
				ctrl.jobs = jobs
				m.layoutDirty = false
				updated, _ := m.Update(app.JobsUpdatedEvent{})
				m = updated.(*AppModel)
				if !m.layoutDirty {
					t.Fatal("job update left cached layout clean")
				}
				if m.state != state {
					t.Fatal("job update changed modal state")
				}
			}
		})
	}
}
