//go:build ignore

package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"kit/ext"
)

// Init asks a decision model whether each shell command is destructive and
// asks the user for confirmation when the model says it probably is.
//
// The extension does not hard-code a model. It uses the decision-model
// setting of Kit, so set one of:
//
//	decision-model: openrouter/typesafe/jev-1.14   # in .kit.yml
//	kit --decision-model openrouter/typesafe/jev-1.14
//	KIT_DECISION_MODEL=openrouter/typesafe/jev-1.14 kit
//
// When no model is set, or the decision call fails, the extension asks the
// user for every shell command. A decision model can be wrong, so do not
// use this gate as the only protection for untrusted input.
//
// Usage: kit -e examples/extensions/decide-gate.go
func Init(api ext.API) {
	// Ask the user when P(destructive) is at or above this value.
	const threshold = 0.5

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		if ctx.GetDecisionModel() == "" {
			ctx.PrintError("decide-gate: no decision-model is set; every shell command needs confirmation")
		}
	})

	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName != "shell" && tc.ToolName != "bash" {
			return nil
		}
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal([]byte(tc.Input), &input); err != nil || input.Command == "" {
			return nil
		}

		reason := ""
		res, err := ctx.Decide(ext.DecisionRequest{
			State: ext.DecisionState{Text: input.Command},
			Questions: []ext.DecisionQuestion{
				ext.BooleanQuestion("destructive",
					"Can this shell command delete, overwrite or corrupt data, or change system configuration?"),
			},
		})
		if err == nil {
			a, aerr := res.Boolean("destructive")
			if aerr == nil && a.Probability < threshold {
				return nil
			}
			if aerr != nil {
				reason = "no answer: " + aerr.Error()
			} else {
				reason = fmt.Sprintf("P(destructive) = %.2f", a.Probability)
			}
		} else if errors.Is(err, ext.ErrNoDecisionModel) {
			reason = "no decision-model is set"
		} else {
			reason = "decision failed: " + err.Error()
		}

		result := ctx.PromptConfirm(ext.PromptConfirmConfig{
			Message: "Command: " + input.Command + "\n(" + reason + ")\n\nAllow execution?",
		})
		if result.Cancelled || !result.Value {
			return &ext.ToolCallResult{
				Block:  true,
				Reason: "User denied execution of command: " + input.Command,
			}
		}
		return nil
	})
}
