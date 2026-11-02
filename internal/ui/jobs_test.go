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
