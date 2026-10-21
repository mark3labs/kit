package extensions

import (
	"os"
	"path/filepath"
	"strings"
)

// extensionCandidate is one extension found by walkExtensionCandidates.
type extensionCandidate struct {
	absPath string // filesystem path (absolute when root is absolute)
	relPath string // slash-separated, relative to root
	isMain  bool   // true for a multi-file package (main.go)
}

// walkExtensionCandidates finds extensions using repository layout conventions.
// It returns candidates found before a walk error, along with that error.
func walkExtensionCandidates(root string) ([]extensionCandidate, error) {
	var results []extensionCandidate
	multiFileDirs := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(root, path)
		relPath = filepath.ToSlash(relPath)

		// Skip directories we know don't contain extensions
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".github", "node_modules", "vendor", "dist", "build":
				return filepath.SkipDir
			}

			// Skip internal code directories
			if strings.HasPrefix(relPath, "internal/") ||
				strings.HasPrefix(relPath, "cmd/") ||
				strings.HasPrefix(relPath, "pkg/") ||
				strings.HasPrefix(relPath, "test/") ||
				strings.HasPrefix(relPath, "tests/") {
				return filepath.SkipDir
			}

			// Root directory - scan it
			if relPath == "." {
				return nil
			}

			base := info.Name()
			isExtDir := base == "extensions" || base == "ext" ||
				strings.HasSuffix(base, "-extensions") || strings.HasSuffix(base, "-ext")

			// Allow walking into examples/ so we can reach examples/extensions/ etc,
			// but don't treat examples/ itself or non-extension subdirs as extension locations.
			if relPath == "examples" {
				return nil
			}

			if !isExtDir {
				mainPath := filepath.Join(path, "main.go")
				if _, err := os.Stat(mainPath); err == nil {
					if relPath == base { // Top-level directory
						if !multiFileDirs[relPath] {
							multiFileDirs[relPath] = true
							results = append(results, extensionCandidate{absPath: mainPath, relPath: relPath + "/main.go", isMain: true})
						}
						return filepath.SkipDir
					}
				}
				return filepath.SkipDir
			}

			// Check for main.go
			mainPath := filepath.Join(path, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				if !multiFileDirs[relPath] {
					multiFileDirs[relPath] = true
					results = append(results, extensionCandidate{absPath: mainPath, relPath: relPath + "/main.go", isMain: true})
				}
				return filepath.SkipDir
			}

			return nil
		}

		// It's a file
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		if info.Name() == "main.go" {
			return nil
		}

		parentDir := filepath.Dir(relPath)
		if parentDir == "." {
			// Root-level .go file - valid extension
			results = append(results, extensionCandidate{absPath: path, relPath: relPath})
			return nil
		}

		if !isValidExtDir(parentDir) {
			return nil
		}

		results = append(results, extensionCandidate{absPath: path, relPath: relPath})
		return nil
	})

	return results, err
}

// isValidExtDir reports whether parentDir is a valid extension location.
// parentDir must be slash-separated and relative to the repository root.
func isValidExtDir(parentDir string) bool {
	for _, dir := range []string{"examples/extensions", "examples/ext", "ext"} {
		if parentDir == dir || strings.HasPrefix(parentDir, dir+"/") {
			return true
		}
	}
	for _, suffix := range []string{"-extensions", "-ext"} {
		if strings.Contains(parentDir, suffix+"/") || strings.HasSuffix(parentDir, suffix) {
			return true
		}
	}
	return false
}
