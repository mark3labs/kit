package codemode

import (
	"fmt"
	"path"
	"strings"
)

// Exposure controls where a tool is visible when code mode is on.
type Exposure string

const (
	// ExposureDirect: the model sees the tool and can call it directly. A
	// script can also call it. This is the default.
	ExposureDirect Exposure = "direct"
	// ExposureCodeMode: the model does not see the tool as a direct tool.
	// Only a script can call it. The code mode catalog lists its full
	// signature.
	ExposureCodeMode Exposure = "codemode"
	// ExposureDeferred: like ExposureCodeMode, but the catalog does not
	// list the tool. A script finds it with searchTools().
	ExposureDeferred Exposure = "deferred"
	// ExposureModelOnly: the model sees the tool, a script cannot call it.
	ExposureModelOnly Exposure = "model-only"
)

// ParseExposure validates s and returns the matching Exposure.
func ParseExposure(s string) (Exposure, error) {
	switch e := Exposure(strings.ToLower(strings.TrimSpace(s))); e {
	case ExposureDirect, ExposureCodeMode, ExposureDeferred, ExposureModelOnly:
		return e, nil
	case "":
		return ExposureDirect, nil
	}
	return "", fmt.Errorf("unknown code mode exposure %q (want direct, codemode, deferred or model-only)", s)
}

// VisibleToModel reports whether the model sees a tool with this exposure as
// a direct tool.
func (e Exposure) VisibleToModel() bool {
	return e == ExposureDirect || e == ExposureModelOnly || e == ""
}

// ExposureRule maps a tool-name glob (path.Match syntax, e.g. "github__*")
// to an exposure.
type ExposureRule struct {
	Pattern  string
	Exposure Exposure
}

// Policy decides the exposure of each tool. Rules are evaluated in order;
// the first rule whose pattern matches wins. Tools that match no rule get
// MCPDefault when they are MCP tools (their name contains the namespace
// separator), else ExposureDirect.
type Policy struct {
	Rules      []ExposureRule
	MCPDefault Exposure
}

// Resolve returns the exposure of the named tool. Matching ignores case,
// because configuration loaders may lowercase map keys.
func (p Policy) Resolve(name string) Exposure {
	lower := strings.ToLower(name)
	for _, r := range p.Rules {
		if ok, err := path.Match(strings.ToLower(r.Pattern), lower); err == nil && ok {
			return r.Exposure
		}
	}
	if p.MCPDefault != "" && strings.Contains(name, NamespaceSeparator) {
		return p.MCPDefault
	}
	return ExposureDirect
}

// ParseRules converts a pattern → exposure map into ordered rules. Exact
// names sort before globs, and longer patterns before shorter ones, so the
// most specific rule wins regardless of map order.
func ParseRules(m map[string]string) ([]ExposureRule, error) {
	rules := make([]ExposureRule, 0, len(m))
	for pat, raw := range m {
		e, err := ParseExposure(raw)
		if err != nil {
			return nil, fmt.Errorf("exposure for %q: %w", pat, err)
		}
		if _, err := path.Match(pat, ""); err != nil {
			return nil, fmt.Errorf("invalid exposure pattern %q: %w", pat, err)
		}
		rules = append(rules, ExposureRule{Pattern: pat, Exposure: e})
	}
	isGlob := func(s string) bool { return strings.ContainsAny(s, "*?[") }
	sortRules(rules, func(a, b ExposureRule) bool {
		ga, gb := isGlob(a.Pattern), isGlob(b.Pattern)
		if ga != gb {
			return !ga
		}
		if len(a.Pattern) != len(b.Pattern) {
			return len(a.Pattern) > len(b.Pattern)
		}
		return a.Pattern < b.Pattern
	})
	return rules, nil
}

func sortRules(r []ExposureRule, less func(a, b ExposureRule) bool) {
	for i := 1; i < len(r); i++ {
		for j := i; j > 0 && less(r[j], r[j-1]); j-- {
			r[j], r[j-1] = r[j-1], r[j]
		}
	}
}
