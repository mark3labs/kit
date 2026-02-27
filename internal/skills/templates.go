package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PromptTemplate is a named text template with {{variable}} placeholders.
type PromptTemplate struct {
	// Name is the human-readable identifier for this template.
	Name string
	// Content is the raw template text with {{variable}} placeholders.
	Content string
	// Variables lists the placeholder names discovered in Content.
	Variables []string
}

// variableRe matches {{variable_name}} placeholders.
var variableRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// NewPromptTemplate creates a PromptTemplate, automatically extracting
// variable names from {{...}} placeholders in content.
func NewPromptTemplate(name, content string) *PromptTemplate {
	vars := extractVariables(content)
	return &PromptTemplate{
		Name:      name,
		Content:   content,
		Variables: vars,
	}
}

// LoadPromptTemplate reads a template from a file.  The template name is
// derived from the filename (without extension).
func LoadPromptTemplate(path string) (*PromptTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", path, err)
	}

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	return NewPromptTemplate(name, string(data)), nil
}

// Expand replaces all {{variable}} placeholders with values from the
// provided map.  Missing variables are left as-is (no error).
func (t *PromptTemplate) Expand(values map[string]string) string {
	result := t.Content
	for k, v := range values {
		result = strings.ReplaceAll(result, "{{"+k+"}}", v)
	}
	return result
}

// ExpandStrict replaces all {{variable}} placeholders and returns an error
// if any variable in the template has no corresponding value.
func (t *PromptTemplate) ExpandStrict(values map[string]string) (string, error) {
	var missing []string
	for _, v := range t.Variables {
		if _, ok := values[v]; !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return "", fmt.Errorf("missing template variables: %s", strings.Join(missing, ", "))
	}
	return t.Expand(values), nil
}

// extractVariables returns unique variable names from {{...}} placeholders.
func extractVariables(content string) []string {
	matches := variableRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var vars []string
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			vars = append(vars, name)
		}
	}
	return vars
}
