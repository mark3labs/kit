// Run with configured provider credentials:
//
// go run ./examples/sdk/background-jobs
// go run ./examples/sdk/background-jobs -managed
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"

	"github.com/mark3labs/kit/pkg/kit"
)

// run runs and prints a shell job, optionally asks the model to summarize it, and closes Kit.
func run(ctx context.Context, managed bool) (err error) {
	host, err := kit.New(ctx, &kit.Options{
		Quiet:        true,
		NoSession:    true,
		NoExtensions: true,
		NoSkills:     true,
		CoreToolList: []string{"shell", "jobs_list", "jobs_get", "jobs_wait", "jobs_cancel"},
	})
	if err != nil {
		return fmt.Errorf("create Kit: %w", err)
	}
	defer func() {
		if closeErr := host.Close(); closeErr != nil {
			err = fmt.Errorf("close Kit: %w", closeErr)
		}
	}()

	// Admitted work survives cancellation of the start context. WaitJob waits
	// for a terminal snapshot; its context cancels only the wait, not the job.
	job, err := host.StartShellJob(ctx, "printf 'Hello from a background job\\n'")
	if err != nil {
		return fmt.Errorf("start shell job: %w", err)
	}
	finished, err := host.WaitJob(ctx, job.ID)
	if err != nil {
		return fmt.Errorf("wait for shell job: %w", err)
	}
	fmt.Printf("Job %s: %s\n%s\n", finished.ID, finished.Status, finished.Output)
	if finished.Status != kit.JobCompleted {
		return fmt.Errorf("shell job failed: %s", finished.Error)
	}

	if managed {
		// Managed mode sends completed output as untrusted data and runs all
		// required continuations. This step calls the configured model.
		result, err := host.PromptWithJobs(ctx, "Summarize the result of the background shell job.")
		if err != nil {
			return fmt.Errorf("run managed prompt: %w", err)
		}
		fmt.Println(result.Response)
	}
	return nil
}

// main parses managed mode and runs the example with a two-minute deadline, exiting on error.
func main() {
	managed := flag.Bool("managed", false, "run a model prompt and drain job continuations")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := run(ctx, *managed); err != nil {
		log.Error("background jobs example failed", "error", err)
		os.Exit(1)
	}
}
