package kit

import (
	"fmt"
	"slices"
	"time"

	"github.com/spf13/viper"

	"github.com/mark3labs/kit/internal/codemode"
	"github.com/mark3labs/kit/internal/core"
)

// CodeModeToolName is the name of the code mode tool.
const CodeModeToolName = core.CodeModeToolName

// Code mode exposure levels. They control where a tool is visible when code
// mode is on. See [CodeModeOptions.Exposure].
const (
	// CodeModeExposureDirect: the model sees the tool and scripts can call
	// it. The default.
	CodeModeExposureDirect = string(codemode.ExposureDirect)
	// CodeModeExposureScriptOnly: only scripts can call the tool; the code
	// mode catalog lists its full signature.
	CodeModeExposureScriptOnly = string(codemode.ExposureCodeMode)
	// CodeModeExposureDeferred: only scripts can call the tool, and the
	// catalog does not list it; scripts find it with searchTools().
	CodeModeExposureDeferred = string(codemode.ExposureDeferred)
	// CodeModeExposureModelOnly: the model sees the tool, scripts cannot
	// call it.
	CodeModeExposureModelOnly = string(codemode.ExposureModelOnly)
)

// CodeModeOptions configures code mode: one tool ("codemode") that runs a
// JavaScript program in a sandbox. The program calls the agent's other
// tools, combines and filters their results, and returns a small answer, so
// intermediate results do not use context.
//
// Zero fields fall back to the "codemode" config section, then to the
// built-in defaults.
type CodeModeOptions struct {
	// Enabled turns code mode on. Naming "codemode" in CoreToolList also
	// turns it on.
	Enabled bool
	// Timeout bounds one script run. Default 30s.
	Timeout time.Duration
	// MaxToolCalls bounds the tool calls one script can make. Default 50.
	MaxToolCalls int
	// MaxOutputBytes bounds the result text the model receives. Longer
	// output is saved to a temporary file. Default 100000.
	MaxOutputBytes int
	// MaxConcurrency bounds parallel tool calls in one script. Default 8.
	MaxConcurrency int
	// MemoryLimitMB bounds approximate heap growth during a script. A
	// negative value disables the check. Default 256.
	MemoryLimitMB int
	// CatalogBudget bounds the bytes of the tool list in the code mode tool
	// description. Default 12000.
	CatalogBudget int
	// MCPExposure is the exposure of MCP tools that no Exposure rule
	// matches. Default CodeModeExposureDirect.
	MCPExposure string
	// Exposure maps tool-name globs (e.g. "github__*") to an exposure
	// level. The most specific pattern wins: exact names before globs,
	// longer patterns before shorter ones.
	Exposure map[string]string
}

// resolveCodeMode merges the "codemode" config section with opts. It
// returns whether code mode is enabled and the tool configuration.
func resolveCodeMode(opts *Options, v *viper.Viper) (bool, core.CodeModeConfig, error) {
	o := CodeModeOptions{
		Enabled:        v.GetBool("codemode.enabled"),
		Timeout:        time.Duration(v.GetInt("codemode.timeout")) * time.Second,
		MaxToolCalls:   v.GetInt("codemode.max-tool-calls"),
		MaxOutputBytes: v.GetInt("codemode.max-output-bytes"),
		MaxConcurrency: v.GetInt("codemode.max-concurrency"),
		MemoryLimitMB:  v.GetInt("codemode.memory-limit-mb"),
		CatalogBudget:  v.GetInt("codemode.catalog-budget"),
		MCPExposure:    v.GetString("codemode.mcp-exposure"),
		Exposure:       v.GetStringMapString("codemode.exposure"),
	}
	if c := opts.CodeMode; c != nil {
		o.Enabled = o.Enabled || c.Enabled
		if c.Timeout > 0 {
			o.Timeout = c.Timeout
		}
		if c.MaxToolCalls > 0 {
			o.MaxToolCalls = c.MaxToolCalls
		}
		if c.MaxOutputBytes > 0 {
			o.MaxOutputBytes = c.MaxOutputBytes
		}
		if c.MaxConcurrency > 0 {
			o.MaxConcurrency = c.MaxConcurrency
		}
		if c.MemoryLimitMB != 0 {
			o.MemoryLimitMB = c.MemoryLimitMB
		}
		if c.CatalogBudget > 0 {
			o.CatalogBudget = c.CatalogBudget
		}
		if c.MCPExposure != "" {
			o.MCPExposure = c.MCPExposure
		}
		if len(c.Exposure) > 0 {
			merged := make(map[string]string, len(o.Exposure)+len(c.Exposure))
			for k, val := range o.Exposure {
				merged[k] = val
			}
			for k, val := range c.Exposure {
				merged[k] = val
			}
			o.Exposure = merged
		}
	}
	cfg, err := o.toolConfig()
	if err != nil {
		return false, core.CodeModeConfig{}, err
	}
	return o.Enabled, cfg, nil
}

func (o CodeModeOptions) toolConfig() (core.CodeModeConfig, error) {
	rules, err := codemode.ParseRules(o.Exposure)
	if err != nil {
		return core.CodeModeConfig{}, fmt.Errorf("codemode: %w", err)
	}
	var mcpDefault codemode.Exposure
	if o.MCPExposure != "" {
		if mcpDefault, err = codemode.ParseExposure(o.MCPExposure); err != nil {
			return core.CodeModeConfig{}, fmt.Errorf("codemode.mcp-exposure: %w", err)
		}
	}
	var mem int64
	switch {
	case o.MemoryLimitMB < 0:
		mem = -1
	case o.MemoryLimitMB > 0:
		mem = int64(o.MemoryLimitMB) << 20
	}
	return core.CodeModeConfig{
		Limits: codemode.Limits{
			Timeout:        o.Timeout,
			MaxToolCalls:   o.MaxToolCalls,
			MaxOutputBytes: o.MaxOutputBytes,
			MaxConcurrency: o.MaxConcurrency,
			MemoryLimit:    mem,
		},
		Policy:        codemode.Policy{Rules: rules, MCPDefault: mcpDefault},
		CatalogBudget: o.CatalogBudget,
	}, nil
}

// withCodeModeTool adds the code mode tool to a resolved core tool list
// when code mode is enabled. It is added even when core tools are disabled,
// so code mode can expose MCP tools on their own.
func withCodeModeTool(toolList []string, enabled bool) []string {
	if !enabled || slices.Contains(toolList, CodeModeToolName) {
		return toolList
	}
	return append(toolList, CodeModeToolName)
}

// NewCodeModeTool creates the code mode tool. Add it to Options.Tools or
// Options.ExtraTools to use code mode with a custom tool set; Kit hands it
// the live tool set automatically.
func NewCodeModeTool(opts CodeModeOptions, toolOpts ...ToolOption) (Tool, error) {
	cfg, err := opts.toolConfig()
	if err != nil {
		return nil, err
	}
	return core.NewCodeModeTool(append(toolOpts, core.WithCodeMode(cfg))...), nil
}
