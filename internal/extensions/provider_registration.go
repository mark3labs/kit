package extensions

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode"
)

func providerObject(value string) bool {
	if value == "" {
		return true
	}
	var object map[string]json.RawMessage
	return json.Unmarshal([]byte(value), &object) == nil && object != nil
}

func freezeCapabilities(c ProviderCapabilities) ProviderCapabilities {
	c.InputModalities = slices.Clone(c.InputModalities)
	c.ReasoningLevels = slices.Clone(c.ReasoningLevels)
	return c
}

func validProviderName(s string) bool {
	if len(s) == 0 || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, c := range s {
		valid := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.'
		if !valid {
			return false
		}
	}
	return true
}

func validateCapabilities(c ProviderCapabilities) error {
	for _, modality := range c.InputModalities {
		switch modality {
		case "text", "image", "audio", "video", "file":
		default:
			return fmt.Errorf("invalid input modality")
		}
	}
	if !c.Reasoning && len(c.ReasoningLevels) > 0 {
		return fmt.Errorf("reasoning levels require reasoning support")
	}
	for _, level := range c.ReasoningLevels {
		if strings.TrimSpace(level) == "" || strings.ContainsFunc(level, unicode.IsControl) || level == "off" {
			return fmt.Errorf("invalid reasoning level")
		}
	}
	return nil
}

func freezeProvider(def ProviderDef) (ProviderDef, error) {
	def.ID = strings.ToLower(strings.TrimSpace(def.ID))
	if !validProviderName(def.ID) {
		return def, fmt.Errorf("invalid ASCII provider identifier")
	}
	if strings.TrimSpace(def.Name) == "" || strings.ContainsFunc(def.Name, unicode.IsControl) {
		return def, fmt.Errorf("provider name is empty")
	}
	if def.Create == nil {
		return def, fmt.Errorf("create callback is required")
	}
	if !providerObject(def.SettingsJSON) {
		return def, fmt.Errorf("settings must be a JSON object")
	}
	if err := validateCapabilities(def.UnknownModelDefaults); err != nil {
		return def, err
	}
	switch def.Auth.Mode {
	case "", ProviderAuthNone, ProviderAuthAPIKey, ProviderAuthExternal:
	default:
		return def, fmt.Errorf("invalid auth mode")
	}
	if def.Auth.Mode == "" {
		def.Auth.Mode = ProviderAuthNone
	}
	def.Auth.Environment = slices.Clone(def.Auth.Environment)
	def.UnknownModelDefaults = freezeCapabilities(def.UnknownModelDefaults)
	def.Models = slices.Clone(def.Models)
	seen := map[string]bool{}
	for i, model := range def.Models {
		model.ID = strings.TrimSpace(model.ID)
		if model.ID == "" || strings.ContainsFunc(model.ID, unicode.IsControl) {
			return def, fmt.Errorf("invalid model identifier at index %d", i)
		}
		if seen[model.ID] {
			return def, fmt.Errorf("duplicate model %q", model.ID)
		}
		seen[model.ID] = true
		if strings.TrimSpace(model.Name) == "" || strings.ContainsFunc(model.Name, unicode.IsControl) {
			return def, fmt.Errorf("model %q has an empty name", model.ID)
		}
		if model.ContextLimit < 0 || model.OutputLimit < 0 || (model.ContextLimit > 0 && model.OutputLimit > model.ContextLimit) {
			return def, fmt.Errorf("model %q has invalid limits", model.ID)
		}
		for _, price := range []ProviderPrice{model.InputPrice, model.OutputPrice, model.CacheReadPrice, model.CacheWritePrice} {
			if price.Value < 0 || math.IsNaN(price.Value) || math.IsInf(price.Value, 0) {
				return def, fmt.Errorf("model %q has invalid pricing", model.ID)
			}
		}
		if !providerObject(model.SettingsJSON) {
			return def, fmt.Errorf("model %q settings must be a JSON object", model.ID)
		}
		if err := validateCapabilities(model.Capabilities); err != nil {
			return def, fmt.Errorf("model %q: %w", model.ID, err)
		}
		model.Capabilities = freezeCapabilities(model.Capabilities)
		def.Models[i] = model
	}
	return def, nil
}

func (ext *LoadedExtension) registerProvider(def ProviderDef) {
	frozen, err := freezeProvider(def)
	if err == nil {
		for _, prior := range ext.Providers {
			if prior.ID == frozen.ID {
				err = fmt.Errorf("duplicate provider registration")
				break
			}
		}
	}
	if err != nil {
		// Do not print invalid raw IDs or configuration supplied by the extension.
		ext.RegistrationErrors = append(ext.RegistrationErrors, fmt.Errorf("extension %s provider registration: %w", ext.Path, err))
		return
	}
	if ext.Owner == nil {
		ext.Owner = &ProviderOwner{}
	}
	frozen.owner = ext.Owner
	frozen.authCache = &ProviderAuthCache{}
	ext.Providers = append(ext.Providers, frozen)
}
