package acpserver

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	"github.com/mark3labs/kit/internal/models"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// Config option IDs Kit exposes through session config options.
const (
	configIDModel         = "model"
	configIDThinkingLevel = "thinking_level"
)

// configOptions returns the full list of session config options with their
// current values. The spec requires this list in the session/new, session/load
// and session/resume responses (optional there) and in every
// session/set_config_option response (required there), so it must never be
// nil.
func configOptions(k *kit.Kit) []acp.SessionConfigOption {
	opts := []acp.SessionConfigOption{modelConfigOption(k)}
	if opt, ok := thinkingConfigOption(k); ok {
		opts = append(opts, opt)
	}
	return opts
}

// modelConfigOption lists the models whose provider has credentials, grouped
// by provider. The current model is always present so the current value is
// always a valid option.
func modelConfigOption(k *kit.Kit) acp.SessionConfigOption {
	current := k.GetModelString()
	registry := models.GetGlobalRegistry()

	var groups acp.SessionConfigSelectOptionsGrouped
	seenCurrent := false
	providers := registry.GetLLMProviders()
	sort.Strings(providers)
	for _, provider := range providers {
		if registry.ValidateEnvironment(provider, "") != nil && !strings.HasPrefix(current, provider+"/") {
			continue
		}
		modelsMap, err := registry.GetModelsForProvider(provider)
		if err != nil || len(modelsMap) == 0 {
			continue
		}
		ids := make([]string, 0, len(modelsMap))
		for id := range modelsMap {
			// The custom provider's "custom" stub is only a fallback for
			// --provider-url and is not a real model.
			if provider == "custom" && id == "custom" && len(modelsMap) > 1 {
				continue
			}
			ids = append(ids, id)
		}
		sort.Strings(ids)
		group := acp.SessionConfigSelectGroup{
			Group: acp.SessionConfigGroupId(provider),
			Name:  provider,
		}
		for _, id := range ids {
			value := provider + "/" + id
			name := modelsMap[id].Name
			if name == "" {
				name = id
			}
			if value == current {
				seenCurrent = true
			}
			group.Options = append(group.Options, acp.SessionConfigSelectOption{
				Name:  name,
				Value: acp.SessionConfigValueId(value),
			})
		}
		if len(group.Options) > 0 {
			groups = append(groups, group)
		}
	}

	// A model not in the registry (for example a custom endpoint) is still
	// the current value, so list it.
	if !seenCurrent && current != "" {
		provider, _, _ := strings.Cut(current, "/")
		groups = append(groups, acp.SessionConfigSelectGroup{
			Group: acp.SessionConfigGroupId(provider + ":current"),
			Name:  provider,
			Options: []acp.SessionConfigSelectOption{{
				Name:  current,
				Value: acp.SessionConfigValueId(current),
			}},
		})
	}

	category := acp.SessionConfigOptionCategoryModel
	desc := "The language model for this session"
	return acp.SessionConfigOption{Select: &acp.SessionConfigOptionSelect{
		Id:           configIDModel,
		Name:         "Model",
		Description:  &desc,
		Category:     &category,
		CurrentValue: acp.SessionConfigValueId(current),
		Options:      acp.SessionConfigSelectOptions{Grouped: &groups},
	}}
}

// thinkingConfigOption returns the reasoning effort option. It is only
// offered for reasoning models; for other models it has no effect.
func thinkingConfigOption(k *kit.Kit) (acp.SessionConfigOption, bool) {
	if !k.IsReasoningModel() {
		return acp.SessionConfigOption{}, false
	}
	levels := k.SupportedThinkingLevels()
	if len(levels) < 2 {
		return acp.SessionConfigOption{}, false
	}
	current := currentThinkingLevel(k, levels)

	options := make(acp.SessionConfigSelectOptionsUngrouped, 0, len(levels))
	for _, l := range levels {
		options = append(options, acp.SessionConfigSelectOption{
			Name:  thinkingLevelName(l),
			Value: acp.SessionConfigValueId(l),
		})
	}
	category := acp.SessionConfigOptionCategoryThoughtLevel
	desc := "How much the model reasons before it answers"
	return acp.SessionConfigOption{Select: &acp.SessionConfigOptionSelect{
		Id:           configIDThinkingLevel,
		Name:         "Thinking",
		Description:  &desc,
		Category:     &category,
		CurrentValue: acp.SessionConfigValueId(current),
		Options:      acp.SessionConfigSelectOptions{Ungrouped: &options},
	}}, true
}

// currentThinkingLevel returns the configured level, mapped onto a level the
// model supports, so the current value is always one of the options.
func currentThinkingLevel(k *kit.Kit, supported []string) string {
	level := string(models.ParseThinkingLevel(k.GetThinkingLevel()))
	if slices.Contains(supported, level) {
		return level
	}
	if provider, modelID, err := kit.ParseModelString(k.GetModelString()); err == nil {
		if s := kit.SuggestThinkingLevel(provider, modelID, level); slices.Contains(supported, s) {
			return s
		}
	}
	return supported[0]
}

// thinkingLevelName returns a display name for a thinking level.
func thinkingLevelName(level string) string {
	if level == "" {
		return level
	}
	return strings.ToUpper(level[:1]) + level[1:]
}

// applyConfigOption sets one config option on the session. Unknown option
// IDs and values that are not in the option list are rejected with
// InvalidParams.
func applyConfigOption(ctx context.Context, k *kit.Kit, configID, value string) error {
	switch configID {
	case configIDModel:
		if value == "" {
			return acp.NewInvalidParams("model value is required")
		}
		if value == k.GetModelString() {
			return nil
		}
		// Kit can use models that are not in the registry (custom
		// endpoints, local servers), so a value outside the option list is
		// tried too. A model Kit cannot build is an invalid parameter.
		if err := k.SetModel(ctx, value); err != nil {
			return acp.NewInvalidParams(fmt.Sprintf("set model %s: %v", value, err))
		}
		return nil

	case configIDThinkingLevel:
		opt, ok := thinkingConfigOption(k)
		if !ok {
			return acp.NewInvalidParams("the current model does not support thinking levels")
		}
		if !optionHasValue(opt, value) {
			return acp.NewInvalidParams(fmt.Sprintf("unsupported thinking level: %s", value))
		}
		if err := k.SetThinkingLevel(ctx, value); err != nil {
			return fmt.Errorf("set thinking level: %w", err)
		}
		return nil
	}
	return acp.NewInvalidParams(fmt.Sprintf("unknown config option: %s", configID))
}

// optionHasValue reports whether value is one of the select option values.
func optionHasValue(opt acp.SessionConfigOption, value string) bool {
	if opt.Select == nil {
		return false
	}
	o := opt.Select.Options
	if o.Ungrouped != nil {
		for _, v := range *o.Ungrouped {
			if string(v.Value) == value {
				return true
			}
		}
	}
	if o.Grouped != nil {
		for _, g := range *o.Grouped {
			for _, v := range g.Options {
				if string(v.Value) == value {
					return true
				}
			}
		}
	}
	return false
}
