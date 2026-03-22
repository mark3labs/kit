package prompts

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// frontmatterSep is the YAML frontmatter delimiter.
const frontmatterSep = "---"

// Frontmatter represents the YAML frontmatter in a prompt template file.
type Frontmatter struct {
	// Description summarises what this template provides.
	Description string `yaml:"description"`
}

// ParseFrontmatter parses YAML frontmatter content into a Frontmatter struct.
func ParseFrontmatter(content string) (*Frontmatter, error) {
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(content), &fm); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	return &fm, nil
}
