package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
)

// LoadOptions configures how templates are discovered and loaded.
type LoadOptions struct {
	// Cwd is the current working directory for project-local discovery.
	// If empty, the current working directory is used.
	Cwd string
	// HomeDir is the user's home directory. If empty, os.UserHomeDir() is used.
	HomeDir string
	// ExtraPaths are additional explicit paths to search for templates.
	ExtraPaths []string
	// ConfigPaths are paths from configuration files to search.
	ConfigPaths []string
	// IncludeDefaults determines whether to include built-in default templates.
	IncludeDefaults bool
}

// Diagnostic reports a template collision or loading issue.
type Diagnostic struct {
	// Name is the template name that had a collision.
	Name string
	// KeptPath is the path of the template that was kept (higher precedence).
	KeptPath string
	// DroppedPath is the path of the template that was dropped.
	DroppedPath string
	// Reason explains why the collision occurred.
	Reason string
}

// LoadAll discovers and loads all prompt templates from standard locations
// and any extra paths. Templates are loaded in order of precedence (lowest
// to highest), with later templates overriding earlier ones of the same name.
//
// Discovery paths searched in order:
//   1. Default templates (if IncludeDefaults)
//   2. ~/.kit/prompts/ (global user templates)
//   3. .kit/prompts/ (project-local templates)
//   4. ConfigPaths (from configuration)
//   5. ExtraPaths (explicit paths, highest precedence)
func LoadAll(opts LoadOptions) ([]*PromptTemplate, []Diagnostic, error) {
	if opts.Cwd == "" {
		opts.Cwd, _ = os.Getwd()
	}

	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("getting home directory: %w", err)
		}
		opts.HomeDir = home
	}

	var all []*PromptTemplate
	var diagnostics []Diagnostic
	seen := make(map[string]*PromptTemplate) // name -> template

	// Helper to add templates with deduplication tracking
	addTemplates := func(templates []*PromptTemplate, source string) {
		for _, tpl := range templates {
			if existing, ok := seen[tpl.Name]; ok {
				// Collision: report diagnostic, keep existing (lower precedence wins)
				diagnostics = append(diagnostics, Diagnostic{
					Name:        tpl.Name,
					KeptPath:    existing.FilePath,
					DroppedPath: tpl.FilePath,
					Reason:      fmt.Sprintf("template from %s overridden by %s", source, existing.Source),
				})
				log.Debug("template collision",
					"name", tpl.Name,
					"dropped", tpl.FilePath,
					"kept", existing.FilePath)
			} else {
				tpl.Source = source
				seen[tpl.Name] = tpl
				all = append(all, tpl)
			}
		}
	}

	// 1. Default templates (lowest precedence)
	if opts.IncludeDefaults {
		defaults := loadDefaultTemplates()
		addTemplates(defaults, "default")
	}

	// 2. Global user templates: ~/.kit/prompts/
	globalDir := filepath.Join(opts.HomeDir, ".kit", "prompts")
	if templates, err := LoadFromDir(globalDir); err == nil {
		addTemplates(templates, "global")
	}

	// 3. Project-local templates: .kit/prompts/
	localDir := filepath.Join(opts.Cwd, ".kit", "prompts")
	if templates, err := LoadFromDir(localDir); err == nil {
		addTemplates(templates, "local")
	}

	// 4. Config paths
	for _, path := range opts.ConfigPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if templates, err := LoadFromDir(path); err == nil {
				addTemplates(templates, "config")
			}
		} else if strings.HasSuffix(path, ".md") {
			if tpl, err := ParseTemplate(path); err == nil {
				addTemplates([]*PromptTemplate{tpl}, "config")
			}
		}
	}

	// 5. Extra paths (highest precedence)
	for _, path := range opts.ExtraPaths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.IsDir() {
			if templates, err := LoadFromDir(path); err == nil {
				addTemplates(templates, "explicit")
			}
		} else if strings.HasSuffix(path, ".md") {
			if tpl, err := ParseTemplate(path); err == nil {
				addTemplates([]*PromptTemplate{tpl}, "explicit")
			}
		}
	}

	return all, diagnostics, nil
}

// LoadFromDir scans a directory for .md files and loads them as templates.
// It looks for *.md files directly in the directory.
// Files that fail to parse are logged and skipped.
func LoadFromDir(dir string) ([]*PromptTemplate, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, nil // directory doesn't exist — not an error
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading prompts directory %s: %w", dir, err)
	}

	var templates []*PromptTemplate
	var errs []string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		full := filepath.Join(dir, name)
		tpl, err := ParseTemplate(full)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		templates = append(templates, tpl)
	}

	if len(errs) > 0 {
		return templates, fmt.Errorf("some templates failed to load: %s", strings.Join(errs, "; "))
	}
	return templates, nil
}

// Deduplicate removes duplicate templates by name, keeping the first occurrence.
// It returns the deduplicated list and diagnostics for any collisions.
// This is a standalone function for when you need to deduplicate an existing list.
func Deduplicate(templates []*PromptTemplate) ([]*PromptTemplate, []Diagnostic) {
	seen := make(map[string]*PromptTemplate)
	var result []*PromptTemplate
	var diagnostics []Diagnostic

	for _, tpl := range templates {
		if existing, ok := seen[tpl.Name]; ok {
			diagnostics = append(diagnostics, Diagnostic{
				Name:        tpl.Name,
				KeptPath:    existing.FilePath,
				DroppedPath: tpl.FilePath,
				Reason:      "duplicate template name (first-match-wins)",
			})
		} else {
			seen[tpl.Name] = tpl
			result = append(result, tpl)
		}
	}

	return result, diagnostics
}

// loadDefaultTemplates returns the built-in default templates.
// These are embedded templates that ship with Kit.
func loadDefaultTemplates() []*PromptTemplate {
	// Default templates can be added here as needed
	// For now, return an empty slice - users can define their own templates
	return nil
}
