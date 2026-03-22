package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// PromptTemplate is a named prompt template with shell-style argument placeholders.
// It supports Pi-style $1, $2, $@, $ARGUMENTS, ${@:N}, ${@:N:L} syntax.
type PromptTemplate struct {
	// Name is the human-readable identifier for this template.
	Name string
	// Description summarises what this template provides.
	Description string
	// Content is the raw template text with placeholders.
	Content string
	// Source indicates where the template was loaded from (e.g., "default", "user").
	Source string
	// FilePath is the absolute filesystem path the template was loaded from.
	FilePath string
}

// ParseTemplate reads a template from a file. The template name is derived
// from the filename (without extension). If the file contains YAML frontmatter,
// the description is extracted from it.
func ParseTemplate(path string) (*PromptTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", path, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	content := string(data)
	tpl := &PromptTemplate{
		FilePath: abs,
		Content:  content,
	}

	// Parse frontmatter if present
	if strings.HasPrefix(strings.TrimSpace(content), frontmatterSep) {
		trimmed := strings.TrimSpace(content)
		rest := trimmed[len(frontmatterSep):]
		frontmatter, body, found := strings.Cut(rest, "\n"+frontmatterSep)
		if found {
			body = strings.TrimPrefix(body, "\n")
			fm, err := ParseFrontmatter(frontmatter)
			if err == nil {
				tpl.Description = fm.Description
			}
			tpl.Content = strings.TrimSpace(body)
		}
	}

	// Derive name from filename
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	tpl.Name = strings.TrimSuffix(base, ext)

	return tpl, nil
}

// ParseCommandArgs splits a command line into arguments respecting quotes.
// It handles single quotes, double quotes, and backslash escaping.
func ParseCommandArgs(input string) []string {
	var args []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' && !inSingleQuote {
			// Backslash escapes next char, but not in single quotes
			escaped = true
			continue
		}

		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}

		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if r == ' ' && !inSingleQuote && !inDoubleQuote {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
		_ = i // silence unused warning when we need position later
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// argPlaceholder matches shell-style argument placeholders:
//   - $1, $2, etc. - positional arguments
//   - $@ - all arguments
//   - $ARGUMENTS - all arguments (alias for $@)
//   - ${@:N} - arguments from N onwards
//   - ${@:N:L} - L arguments starting from N
var argPlaceholder = regexp.MustCompile(`\$\{(\d+)\}|\$\{(\d+):(\d+)\}|\$\{ARGUMENTS\}|\$\{@(:\d+)?(:\d+)?\}|\$(\d+)|\$@|\$ARGUMENTS`)

// SubstituteArgs replaces argument placeholders in content with values from args.
// Supported placeholders:
//   - $N, ${N} - the Nth argument (1-indexed)
//   - $@, $ARGUMENTS, ${ARGUMENTS} - all arguments joined with spaces
//   - ${@:N} - arguments from index N onwards (0-indexed)
//   - ${@:N:L} - L arguments starting from index N (0-indexed)
func SubstituteArgs(content string, args []string) string {
	return argPlaceholder.ReplaceAllStringFunc(content, func(match string) string {
		// Check for ${N} or ${N:M} format
		if strings.HasPrefix(match, "${") && strings.Contains(match, "}") {
			inner := match[2 : len(match)-1] // Remove ${ and }

			// Check for ${ARGUMENTS}
			if inner == "ARGUMENTS" {
				return strings.Join(args, " ")
			}

			// Check for ${@...} format
			if strings.HasPrefix(inner, "@") {
				return expandAtArgs(inner, args)
			}

			// Check for ${N:M} format (positional with length)
			if colonIdx := strings.Index(inner, ":"); colonIdx > 0 {
				startStr := inner[:colonIdx]
				rest := inner[colonIdx+1:]

				start, err := strconv.Atoi(startStr)
				if err != nil || start < 1 {
					return match
				}

				// Check if there's a second colon for length ${N:M:L}
				if secondColonIdx := strings.Index(rest, ":"); secondColonIdx >= 0 {
					lengthStr := rest[secondColonIdx+1:]
					length, err := strconv.Atoi(lengthStr)
					if err != nil || length < 0 {
						return match
					}
					return joinArgsRange(args, start-1, length)
				}

				// Single colon ${N:M} - M is length
				length, err := strconv.Atoi(rest)
				if err != nil || length < 0 {
					return match
				}
				return joinArgsRange(args, start-1, length)
			}

			// Simple ${N} format
			n, err := strconv.Atoi(inner)
			if err != nil || n < 1 {
				return match
			}
			if n <= len(args) {
				return args[n-1]
			}
			return ""
		}

		// Check for $N format (without braces)
		if strings.HasPrefix(match, "$") && !strings.HasPrefix(match, "${") {
			suffix := match[1:]

			// $@ or $ARGUMENTS
			if suffix == "@" || suffix == "ARGUMENTS" {
				return strings.Join(args, " ")
			}

			// $N
			n, err := strconv.Atoi(suffix)
			if err != nil || n < 1 {
				return match
			}
			if n <= len(args) {
				return args[n-1]
			}
			return ""
		}

		return match
	})
}

// expandAtArgs handles ${@...} patterns (1-indexed like bash)
func expandAtArgs(inner string, args []string) string {
	// Remove the @ prefix
	rest := inner[1:]

	if rest == "" {
		// ${@} - all arguments
		return strings.Join(args, " ")
	}

	// Must start with :
	if !strings.HasPrefix(rest, ":") {
		return "${" + inner + "}"
	}
	rest = rest[1:]

	// Parse start index
	colonIdx := strings.Index(rest, ":")
	var startStr, lengthStr string

	if colonIdx >= 0 {
		startStr = rest[:colonIdx]
		lengthStr = rest[colonIdx+1:]
	} else {
		startStr = rest
	}

	start, err := strconv.Atoi(startStr)
	if err != nil || start < 0 {
		return "${" + inner + "}"
	}

	// Convert from 1-indexed to 0-indexed (bash convention)
	// Treat 0 as 1 (bash convention: args start at 1)
	if start > 0 {
		start--
	}

	if lengthStr != "" {
		length, err := strconv.Atoi(lengthStr)
		if err != nil || length < 0 {
			return "${" + inner + "}"
		}
		return joinArgsRange(args, start, length)
	}

	// ${@:N} - from N to end
	if start >= len(args) {
		return ""
	}
	return strings.Join(args[start:], " ")
}

// joinArgsRange joins args from start index, taking up to length elements
func joinArgsRange(args []string, start, length int) string {
	if start >= len(args) || length <= 0 {
		return ""
	}
	end := start + length
	if end > len(args) {
		end = len(args)
	}
	return strings.Join(args[start:end], " ")
}

// Expand substitutes arguments into the template content and returns the result.
// It first parses args from the input string, then substitutes them into the template.
func (t *PromptTemplate) Expand(argsInput string) string {
	args := ParseCommandArgs(argsInput)
	return SubstituteArgs(t.Content, args)
}

// ExpandWithArgs substitutes the provided arguments into the template content.
func (t *PromptTemplate) ExpandWithArgs(args []string) string {
	return SubstituteArgs(t.Content, args)
}
