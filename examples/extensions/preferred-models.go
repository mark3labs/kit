//go:build ignore

package main

import (
	"fmt"
	"strconv"
	"strings"

	"kit/ext"
)

// preferredModels reads the option preferred-models: provider/model strings,
// separated by commas or spaces.
func preferredModels(ctx ext.Context) []string {
	return strings.Fields(strings.ReplaceAll(ctx.GetOption("preferred-models"), ",", " "))
}

// currentModel returns the model in use as provider/model. After a model
// switch, ctx.Model is provider/model. At startup, ctx.Model is only the
// model ID. Then the configured model is in use, and the model registry
// gives its provider. If the registry does not know the model, the result
// is only the model ID.
func currentModel(ctx ext.Context) string {
	if strings.Contains(ctx.Model, "/") {
		return ctx.Model
	}
	if caps, errMsg := ctx.GetModelCapabilities(""); errMsg == "" && caps.ModelID == ctx.Model {
		return caps.Provider + "/" + caps.ModelID
	}
	return ctx.Model
}

// pickModel returns the model that arg selects. arg is the number of the
// model in the list, starting at 1, or a part of a model name. A part of a
// name selects the first model that contains it. pickModel returns "" if
// nothing matches.
func pickModel(list []string, arg string) string {
	if n, err := strconv.Atoi(arg); err == nil {
		if n >= 1 && n <= len(list) {
			return list[n-1]
		}
		return ""
	}
	for _, m := range list {
		if strings.Contains(strings.ToLower(m), strings.ToLower(arg)) {
			return m
		}
	}
	return ""
}

// switchTo switches to model and returns the message for the transcript.
func switchTo(ctx ext.Context, model string) (string, error) {
	if model == currentModel(ctx) {
		return "Already using " + model, nil
	}
	if err := ctx.SetModel(model); err != nil {
		return "", err
	}
	return "Switched to " + model, nil
}

// Init adds the command /preferred. It switches between the models in the
// option preferred-models, which lists provider/model strings separated by
// commas or spaces. Write the list as one string; a YAML list is not read.
//
//	options:
//	  preferred-models: anthropic/claude-opus-5-5, amazon-bedrock/eu.anthropic.claude-opus-5-5
//
// or for one run: KIT_OPT_PREFERRED_MODELS="ollama/llama3 ollama/qwen3" kit
//
//	/preferred          select a model from the list
//	/preferred 2        switch to the second model in the list
//	/preferred bedrock  switch to the first model whose name contains "bedrock"
//
// Unlike /model, the list contains only the models that you put in it.
//
// Usage: kit -e examples/extensions/preferred-models.go
func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "preferred-models",
		Description: "Models for /preferred, as provider/model, separated by commas",
	})

	api.RegisterCommand(ext.CommandDef{
		Name:        "preferred",
		Description: "Switch to one of your preferred models",
		Execute: func(args string, ctx ext.Context) (string, error) {
			list := preferredModels(ctx)
			if len(list) == 0 {
				return "No preferred models. List them in the option preferred-models, for example:\n  options:\n    preferred-models: anthropic/claude-opus-5-5, anthropic/claude-sonnet-5-5", nil
			}
			if arg := strings.TrimSpace(args); arg != "" {
				model := pickModel(list, arg)
				if model == "" {
					return "", fmt.Errorf("no preferred model matches %q", arg)
				}
				return switchTo(ctx, model)
			}
			current := currentModel(ctx)
			options := make([]string, len(list))
			for i, m := range list {
				options[i] = fmt.Sprintf("%d  %s", i+1, m)
				if m == current {
					options[i] += "  (current)"
				}
			}
			res := ctx.PromptSelect(ext.PromptSelectConfig{
				Message: "Switch to a preferred model",
				Options: options,
			})
			if res.Cancelled || res.Index < 0 || res.Index >= len(list) {
				return "", nil
			}
			return switchTo(ctx, list[res.Index])
		},
		Complete: func(prefix string, ctx ext.Context) []string {
			var out []string
			for _, m := range preferredModels(ctx) {
				if strings.HasPrefix(m, prefix) {
					out = append(out, m)
				}
			}
			return out
		},
	})
}
