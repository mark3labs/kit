package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// FileSuggestion represents a single file or directory suggestion for the @
// autocomplete popup.
type FileSuggestion struct {
	// RelPath is the path relative to the search base (e.g. "cmd/kit/main.go").
	RelPath string
	// IsDir is true when the entry is a directory.
	IsDir bool
	// Score is the fuzzy match score (higher is better).
	Score int
}

// maxFileSuggestions is the maximum number of file suggestions returned.
const maxFileSuggestions = 20

// ExtractAtPrefix checks the current line for an @-file trigger at cursorCol.
// It returns:
//   - hasAt: true if a valid @ trigger was found
//   - prefix: the text after @ (possibly empty) that the user has typed so far
//   - startIdx: byte offset of the @ character in the line
//
// The @ must appear at the start of the line or after whitespace. Quoted paths
// are supported: @"path with spaces" — the returned prefix strips quotes.
func ExtractAtPrefix(line string, cursorCol int) (hasAt bool, prefix string, startIdx int) {
	if cursorCol > len(line) {
		cursorCol = len(line)
	}

	// Walk backwards from cursorCol to find the @ character.
	text := line[:cursorCol]

	// Find the last @ that is preceded by whitespace or is at position 0.
	atIdx := -1
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == '@' {
			// Must be at start of line or preceded by whitespace.
			if i == 0 || text[i-1] == ' ' || text[i-1] == '\t' {
				atIdx = i
				break
			}
		}
		// Stop scanning if we hit a space — the @ we want must be in the
		// current "word".
		if text[i] == ' ' || text[i] == '\t' {
			break
		}
	}

	if atIdx < 0 {
		return false, "", 0
	}

	raw := text[atIdx+1:]

	// Handle quoted paths: @"some path" — strip leading quote.
	if after, found := strings.CutPrefix(raw, `"`); found {
		raw = strings.TrimSuffix(after, `"`)
	}

	return true, raw, atIdx
}

// GetFileSuggestions returns file/directory suggestions matching the given
// prefix. It tries `git ls-files` first (fast, respects .gitignore), then
// falls back to a simple directory walk.
//
// If prefix contains a path separator the search is scoped to that
// subdirectory. For example, prefix "cmd/k" searches inside "cmd/" for
// entries matching "k".
func GetFileSuggestions(prefix string, cwd string) []FileSuggestion {
	// Resolve the base directory and filter query from the prefix.
	baseDir, query := splitPrefixPath(prefix)

	searchDir := cwd
	if baseDir != "" {
		candidate := resolveSearchDir(baseDir, cwd)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			searchDir = candidate
		} else {
			return nil // invalid base directory
		}
	}

	files := listFiles(searchDir, cwd)
	if len(files) == 0 {
		return nil
	}

	// Prepend baseDir so results display as "cmd/main.go" not just "main.go".
	if baseDir != "" {
		for i := range files {
			files[i].RelPath = baseDir + files[i].RelPath
		}
	}

	return fuzzyFilterFiles(files, prefix, query)
}

// splitPrefixPath separates a prefix like "cmd/kit/m" into
// baseDir="cmd/kit/" and query="m". If there is no separator the
// baseDir is empty and query is the full prefix.
func splitPrefixPath(prefix string) (baseDir, query string) {
	// Handle ~ expansion display (we keep it in the prefix for display
	// but resolve it when actually searching).
	idx := strings.LastIndex(prefix, "/")
	if idx < 0 {
		return "", prefix
	}
	return prefix[:idx+1], prefix[idx+1:]
}

// resolveSearchDir converts a baseDir from the prefix into an absolute path.
// Supports ~/, ../, and absolute paths.
func resolveSearchDir(baseDir, cwd string) string {
	// Expand ~/
	if strings.HasPrefix(baseDir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, baseDir[2:])
		}
	}

	// Absolute paths
	if filepath.IsAbs(baseDir) {
		return filepath.Clean(baseDir)
	}

	// Relative to cwd
	return filepath.Join(cwd, baseDir)
}

// listFiles returns files and directories within searchDir, relative to that
// directory. Uses `git ls-files` when inside a git repo for speed and
// .gitignore awareness, otherwise falls back to os.ReadDir.
func listFiles(searchDir, cwd string) []FileSuggestion {
	// Try git ls-files first (fast, respects .gitignore).
	if files := listFilesGit(searchDir, cwd); files != nil {
		return files
	}
	return listFilesReadDir(searchDir)
}

// listFilesGit uses `git ls-files` and `git ls-files --others --exclude-standard`
// to list tracked and untracked-but-not-ignored files.
func listFilesGit(searchDir, cwd string) []FileSuggestion {
	// Check if we're in a git repo.
	check := exec.Command("git", "rev-parse", "--show-toplevel")
	check.Dir = cwd
	if err := check.Run(); err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var results []FileSuggestion

	// Tracked files.
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = searchDir
	out, err := cmd.Output()
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			// Normalize separators.
			line = filepath.ToSlash(line)
			addFileEntries(&results, seen, line, searchDir)
		}
	}

	// Untracked, non-ignored files.
	cmd2 := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd2.Dir = searchDir
	out2, err := cmd2.Output()
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(string(out2)), "\n") {
			if line == "" {
				continue
			}
			line = filepath.ToSlash(line)
			addFileEntries(&results, seen, line, searchDir)
		}
	}

	if len(results) == 0 {
		return nil
	}
	return results
}

// addFileEntries adds the file and any intermediate directory entries to
// results if not already seen. Paths are stored with forward slashes.
func addFileEntries(results *[]FileSuggestion, seen map[string]bool, relPath string, searchDir string) {
	// Add intermediate directories as suggestions (first component only).
	parts := strings.SplitN(relPath, "/", 2)
	if len(parts) > 1 {
		dir := parts[0] + "/"
		if !seen[dir] {
			seen[dir] = true
			*results = append(*results, FileSuggestion{RelPath: dir, IsDir: true})
		}
	}

	// Add the file itself.
	if !seen[relPath] {
		seen[relPath] = true
		*results = append(*results, FileSuggestion{RelPath: relPath, IsDir: false})
	}
}

// listFilesReadDir is the fallback when git is not available. Lists immediate
// children of dir via os.ReadDir, skipping hidden dirs and common noise.
func listFilesReadDir(dir string) []FileSuggestion {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	skip := map[string]bool{
		".git": true, "node_modules": true, ".kit": true,
		"__pycache__": true, ".venv": true, "vendor": true,
	}

	var results []FileSuggestion
	for _, e := range entries {
		name := e.Name()
		if skip[name] {
			continue
		}
		// Skip hidden files/dirs (except common config files).
		if strings.HasPrefix(name, ".") && name != ".env" && name != ".gitignore" {
			continue
		}
		if e.IsDir() {
			results = append(results, FileSuggestion{RelPath: name + "/", IsDir: true})
		} else {
			results = append(results, FileSuggestion{RelPath: name, IsDir: false})
		}
	}
	return results
}

// fuzzyFilterFiles scores and filters file suggestions against the query,
// returning the top maxFileSuggestions results sorted by score descending.
// Directories are boosted slightly so they appear near the top.
func fuzzyFilterFiles(files []FileSuggestion, fullPrefix, query string) []FileSuggestion {
	if query == "" && fullPrefix == "" {
		// No filter — return all (capped).
		if len(files) > maxFileSuggestions {
			files = files[:maxFileSuggestions]
		}
		return files
	}

	// When there's a base dir but no query (e.g. "cmd/"), show everything
	// in that directory.
	if query == "" {
		var filtered []FileSuggestion
		for i := range files {
			if strings.HasPrefix(files[i].RelPath, fullPrefix) {
				// Only show direct children of the base directory.
				rest := files[i].RelPath[len(fullPrefix):]
				if rest == "" {
					continue
				}
				filtered = append(filtered, files[i])
			}
		}
		if len(filtered) > maxFileSuggestions {
			filtered = filtered[:maxFileSuggestions]
		}
		return filtered
	}

	var scored []FileSuggestion
	queryLower := strings.ToLower(query)

	for i := range files {
		path := files[i].RelPath
		// When we have a fullPrefix with a dir component, only consider
		// files under that directory.
		if fullPrefix != query && !strings.HasPrefix(path, fullPrefix[:len(fullPrefix)-len(query)]) {
			continue
		}

		score := scoreFilePath(queryLower, path)
		if score <= 0 {
			continue
		}

		// Boost directories so they appear near the top for navigation.
		if files[i].IsDir {
			score += 10
		}

		files[i].Score = score
		scored = append(scored, files[i])
	}

	// Sort by score descending.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > maxFileSuggestions {
		scored = scored[:maxFileSuggestions]
	}
	return scored
}

// scoreFilePath scores a file path against a fuzzy query. Higher is better.
// Returns 0 if there is no match.
func scoreFilePath(query, path string) int {
	pathLower := strings.ToLower(path)
	baseName := filepath.Base(strings.TrimSuffix(path, "/"))
	baseNameLower := strings.ToLower(baseName)

	// Exact basename match.
	if baseNameLower == query {
		return 1000
	}

	// Basename starts with query.
	if strings.HasPrefix(baseNameLower, query) {
		return 800 - len(baseName) + len(query)
	}

	// Basename contains query as substring.
	if strings.Contains(baseNameLower, query) {
		return 500 - len(baseName) + len(query)
	}

	// Full path contains query as substring.
	if strings.Contains(pathLower, query) {
		return 300 - len(path) + len(query)
	}

	// Fuzzy character match on basename.
	if score := fuzzyCharacterMatch(query, baseNameLower); score > 0 {
		return score
	}

	// Fuzzy character match on full path.
	if score := fuzzyCharacterMatch(query, pathLower); score > 0 {
		return score - 50
	}

	return 0
}
