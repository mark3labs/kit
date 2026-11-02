package core

import (
	"context"
	"encoding/json"
	"fmt"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/jobs"
)

// JobService connects core tools to session-owned background work and scheduling.
// Start must detach work from turn cancellation, preserve context values, and
// supply a context controlled by the service lifetime and job cancellation.
// Yield registers a scheduler wait and returns immediately. The service, not
// the tool, must halt the current generation after a successful yield.
type JobService interface {
	Start(ctx context.Context, kind string, work func(context.Context) (string, error)) (jobs.Job, error)
	Get(id string) (jobs.Job, error)
	List() []jobs.Job
	Cancel(id string) error
	WaitAny(ctx context.Context, ids []string, all bool) ([]jobs.Job, error)
	Yield(ctx context.Context, ids []string, all bool) error
}

type jobServiceKey struct{}
type backgroundJobKey struct{}

// WithJobService stores the job service used by core tools in ctx.
func WithJobService(ctx context.Context, service JobService) context.Context {
	return context.WithValue(ctx, jobServiceKey{}, service)
}

func jobServiceFromContext(ctx context.Context) JobService {
	service, _ := ctx.Value(jobServiceKey{}).(JobService)
	return service
}

func startBackgroundJob(ctx context.Context, kind string, call fantasy.ToolCall, args any, execute func(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error)) (fantasy.ToolResponse, error) {
	service := jobServiceFromContext(ctx)
	if service == nil {
		return fantasy.NewTextErrorResponse("background job service is not available"), nil
	}
	input, err := json.Marshal(args)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("encode background arguments: %v", err)), nil
	}
	call.Input = string(input)
	// Mask turn-scoped streaming callbacks before the service copies values.
	ctx = ContextWithToolOutputCallback(ctx, nil)
	job, err := service.Start(ctx, kind, func(jobCtx context.Context) (string, error) {
		jobCtx = context.WithValue(ContextWithToolOutputCallback(jobCtx, nil), backgroundJobKey{}, true)
		response, err := execute(jobCtx, call)
		if err != nil {
			return response.Content, fmt.Errorf("execute background %s: %w", kind, err)
		}
		if response.IsError {
			return response.Content, fmt.Errorf("background %s failed: %s", kind, response.Content)
		}
		return response.Content, nil
	})
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("start background %s: %v", kind, err)), nil
	}
	return jobResponse(job)
}

func jobResponse(value any) (fantasy.ToolResponse, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("encode jobs response: %w", err)
	}
	return fantasy.NewTextResponse(string(data)), nil
}

type jobToolArgs struct {
	ID  string   `json:"id"`
	IDs []string `json:"ids"`
	All bool     `json:"all,omitempty"`
}

func newJobTool(name, description string, parameters map[string]any, required []string) fantasy.AgentTool {
	return &coreTool{
		info: fantasy.ToolInfo{Name: name, Description: description, Parameters: parameters, Required: required},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			var args jobToolArgs
			if err := parseArgs(call.Input, &args); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			service := jobServiceFromContext(ctx)
			if service == nil {
				return fantasy.NewTextErrorResponse("background job service is not available"), nil
			}
			switch name {
			case "jobs_list":
				return jobResponse(service.List())
			case "jobs_get", "jobs_cancel":
				if args.ID == "" {
					return fantasy.NewTextErrorResponse("id parameter is required"), nil
				}
				if name == "jobs_cancel" {
					if err := service.Cancel(args.ID); err != nil {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("cancel job: %v", err)), nil
					}
				}
				job, err := service.Get(args.ID)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("get job: %v", err)), nil
				}
				return jobResponse(job)
			case "jobs_wait":
				if len(args.IDs) == 0 {
					return fantasy.NewTextErrorResponse("ids parameter must contain at least one job ID"), nil
				}
				if err := service.Yield(ctx, args.IDs, args.All); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("wait for jobs: %v", err)), nil
				}
				return fantasy.NewTextResponse("Waiting for background jobs. The scheduler will resume when the wait is complete."), nil
			}
			return fantasy.NewTextErrorResponse("unknown job tool"), nil
		},
	}
}

// NewJobsListTool creates a tool that lists retained background jobs.
func NewJobsListTool(_ ...ToolOption) fantasy.AgentTool {
	return newJobTool("jobs_list", "List background jobs and their current status.", map[string]any{}, nil)
}

// NewJobsGetTool creates a tool that retrieves a background job and its output.
func NewJobsGetTool(_ ...ToolOption) fantasy.AgentTool {
	return newJobTool("jobs_get", "Get the status and output of a background job.", map[string]any{"id": map[string]any{"type": "string"}}, []string{"id"})
}

// NewJobsCancelTool creates a tool that cancels a background job.
func NewJobsCancelTool(_ ...ToolOption) fantasy.AgentTool {
	return newJobTool("jobs_cancel", "Cancel a background job.", map[string]any{"id": map[string]any{"type": "string"}}, []string{"id"})
}

// NewJobsWaitTool creates a tool that yields execution to the job scheduler.
func NewJobsWaitTool(_ ...ToolOption) fantasy.AgentTool {
	return newJobTool("jobs_wait", "Yield until any requested background job finishes, or all when all is true. This ends the current turn; the scheduler resumes execution.", map[string]any{
		"ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "minItems": 1},
		"all": map[string]any{"type": "boolean", "description": "Wait for all requested jobs (default false)."},
	}, []string{"ids"})
}
