package models

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Resolver combines an immutable instance catalog with the process catalog.
// It does not retain extension implementation types or publish global entries.
type Resolver struct {
	entries         map[string]map[string]ModelInfo
	unknown         map[string]bool
	owners          map[string]string
	unknownMetadata map[string]ModelInfo
}

// NewResolver snapshots instance metadata. Unknown lookups do not populate lists.
func NewResolver(entries map[string]map[string]ModelInfo, unknown map[string]bool) *Resolver {
	result := &Resolver{entries: make(map[string]map[string]ModelInfo, len(entries)), unknown: maps.Clone(unknown)}
	for provider, items := range entries {
		result.entries[provider] = map[string]ModelInfo{}
		for id, info := range items {
			result.entries[provider][id] = cloneModelInfo(info)
		}
	}
	return result
}
func cloneModelInfo(info ModelInfo) ModelInfo {
	if info.Cost.CacheRead != nil {
		value := *info.Cost.CacheRead
		info.Cost.CacheRead = &value
	}
	if info.Cost.CacheWrite != nil {
		value := *info.Cost.CacheWrite
		info.Cost.CacheWrite = &value
	}
	info.InputModalities = slices.Clone(info.InputModalities)
	info.ReasoningLevels = slices.Clone(info.ReasoningLevels)
	return info
}

// SetOwner records a source identity before the resolver is published.
func (r *Resolver) SetOwner(provider, path string) {
	if r.owners == nil {
		r.owners = map[string]string{}
	}
	r.owners[provider] = path
}

// SetUnknownMetadata supplies explicit provider-level unknown-model defaults.
func (r *Resolver) SetUnknownMetadata(provider string, info ModelInfo) {
	if r.unknownMetadata == nil {
		r.unknownMetadata = map[string]ModelInfo{}
	}
	r.unknownMetadata[provider] = cloneModelInfo(info)
}

// Owner returns the registered source identity.
func (r *Resolver) Owner(provider string) string {
	if r == nil {
		return ""
	}
	return r.owners[provider]
}

// Owns reports whether this generation declares the provider.
func (r *Resolver) Owns(provider string) bool {
	if r == nil {
		return false
	}
	_, ok := r.entries[strings.ToLower(provider)]
	return ok
}

// Lookup returns a metadata copy, falling back only for non-instance providers.
func (r *Resolver) Lookup(provider, id string) *ModelInfo {
	if r != nil {
		if items, ok := r.entries[strings.ToLower(provider)]; ok {
			if info, ok := items[id]; ok {
				copy := cloneModelInfo(info)
				return &copy
			}
			if r.unknown[strings.ToLower(provider)] {
				info := cloneModelInfo(r.unknownMetadata[strings.ToLower(provider)])
				info.ID = id
				info.Name = id
				info.CapabilitiesKnown = true
				info.Streaming = true
				info.ReasoningIsGraded = true
				if len(info.InputModalities) == 0 {
					info.InputModalities = []string{"text"}
				}
				return &info
			}
			return nil
		}
	}
	return GetGlobalRegistry().LookupModel(provider, id)
}

// Validate checks instance declarations without accepting removed providers.
func (r *Resolver) Validate(model string) error {
	provider, id, err := ParseModelString(model)
	if err != nil {
		return err
	}
	if r.Owns(provider) {
		if r.Lookup(provider, id) == nil {
			return fmt.Errorf("extension provider %s does not declare model %q", provider, id)
		}
		return nil
	}
	return GetGlobalRegistry().ValidateModelString(model)
}

// List returns a copy of declared instance entries only.
func (r *Resolver) List() map[string]map[string]ModelInfo {
	result := map[string]map[string]ModelInfo{}
	if r != nil {
		for provider, entries := range r.entries {
			result[provider] = map[string]ModelInfo{}
			for id, info := range entries {
				result[provider][id] = cloneModelInfo(info)
			}
		}
	}
	return result
}
