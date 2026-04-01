package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// fileTokenPattern matches @file references in user text. Supports:
//   - @"path with spaces.txt" (quoted)
//   - @path/to/file.txt      (unquoted, no spaces)
var fileTokenPattern = regexp.MustCompile(`@"[^"]+"|@[^\s]+`)

// ProcessFileAttachments scans the user's input text for @file references,
// reads each referenced file, and returns the text with @tokens replaced by
// XML-wrapped file content. Non-file @ tokens (like email addresses) are left
// unchanged.
//
// Returns the original text unchanged if no valid @file references are found.
func ProcessFileAttachments(text string, cwd string) string {
	tokens := fileTokenPattern.FindAllString(text, -1)
	if len(tokens) == 0 {
		return text
	}

	result := text
	for _, token := range tokens {
		path := tokenToPath(token)
		if path == "" {
			continue
		}

		absPath, err := resolvePath(path, cwd)
		if err != nil {
			// Not a valid file reference — leave the token as-is.
			// This handles cases like email addresses (@user) gracefully.
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			continue
		}

		// Skip directories — we only attach file content.
		if info.IsDir() {
			continue
		}

		// Skip empty files.
		if info.Size() == 0 {
			continue
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		// Build the XML-wrapped replacement.
		wrapped := wrapFileContent(absPath, content)
		result = strings.Replace(result, token, wrapped, 1)
	}

	return result
}

// tokenToPath strips the @ prefix and optional quotes from a token,
// returning the raw file path. Returns "" for invalid tokens.
func tokenToPath(token string) string {
	if !strings.HasPrefix(token, "@") {
		return ""
	}
	path := token[1:]

	// Strip quotes.
	if strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		path = path[1 : len(path)-1]
	}

	// Reject obviously non-file tokens (e.g. bare @ or @-flags).
	if path == "" || strings.HasPrefix(path, "-") {
		return ""
	}

	return path
}

// resolvePath resolves a potentially relative file path to an absolute path.
// Supports ~/ expansion and relative paths. No CWD restriction — the user
// can reference any file they have read access to.
func resolvePath(path string, cwd string) (string, error) {
	// Expand ~/
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}

	// Resolve relative to cwd.
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	// Clean and resolve symlinks for consistent paths.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Resolve symlinks so the displayed path is canonical.
	resolved, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// EvalSymlinks fails if the file doesn't exist — fall back to
		// the cleaned absolute path and let the caller's Stat handle it.
		return absPath, nil
	}

	return resolved, nil
}

// wrapFileContent wraps file content in XML tags for LLM consumption.
func wrapFileContent(absPath string, content []byte) string {
	return fmt.Sprintf("<file path=\"%s\">\n%s\n</file>", absPath, string(content))
}
