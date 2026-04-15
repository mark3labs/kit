package fileutil

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mark3labs/kit/internal/fences"
)

// FilePart represents a binary file attachment (image, audio, etc.) extracted
// from an @file reference. Callers convert this to kit.LLMFilePart before
// sending to the LLM. Defined here to avoid a circular dependency on pkg/kit.
type FilePart struct {
	// Filename is the basename of the file (e.g. "photo.png").
	Filename string
	// Data is the raw file bytes.
	Data []byte
	// MediaType is the MIME type (e.g. "image/png", "audio/wav").
	MediaType string
}

// MCPResourceReader is a callback function that reads an MCP resource by
// server name and URI. Returns text content, binary data, MIME type, and error.
// Used by ProcessFileAttachments to resolve @mcp:server:uri tokens.
type MCPResourceReader func(serverName, uri string) (text string, blobData []byte, mimeType string, isBlob bool, err error)

// FileAttachmentResult is the result of processing @file references in user
// input. Text files are inlined as XML in ProcessedText; binary files (images,
// audio, video, PDFs) are returned as FileParts for multimodal submission.
type FileAttachmentResult struct {
	// ProcessedText is the user's text with @file tokens replaced:
	// text files become XML-wrapped content, binary file tokens are removed.
	ProcessedText string
	// FileParts contains binary file attachments extracted from @file
	// references. Empty when all referenced files are text.
	FileParts []FilePart
}

// fileTokenPattern matches @file references in user text. Supports:
//   - @"path with spaces.txt" (quoted)
//   - @path/to/file.txt      (unquoted, no spaces)
var fileTokenPattern = regexp.MustCompile(`@"[^"]+"|@[^\s]+`)

// ProcessFileAttachments scans the user's input text for @file references,
// reads each referenced file, and returns a result containing the processed
// text and any binary file attachments. Text files are XML-wrapped inline;
// binary files (images, audio, etc.) are extracted as FileParts for multimodal
// submission. Non-file @ tokens (like email addresses) are left unchanged.
//
// MCP resources are supported via @mcp:server:uri tokens. The optional
// mcpReader callback is used to resolve them; pass nil to skip MCP resources.
func ProcessFileAttachments(text string, cwd string, mcpReader ...MCPResourceReader) FileAttachmentResult {
	var reader MCPResourceReader
	if len(mcpReader) > 0 {
		reader = mcpReader[0]
	}
	var allParts []FilePart
	processed := fences.ReplaceOutside(text, func(segment string) string {
		result, parts := processFileTokens(segment, cwd, reader)
		allParts = append(allParts, parts...)
		return result
	})
	return FileAttachmentResult{
		ProcessedText: processed,
		FileParts:     allParts,
	}
}

// processFileTokens handles @file replacement in a single text segment
// that is known to be outside fenced code blocks. Returns the processed
// text and any binary file parts extracted.
func processFileTokens(text string, cwd string, mcpReader MCPResourceReader) (string, []FilePart) {
	tokens := fileTokenPattern.FindAllString(text, -1)
	if len(tokens) == 0 {
		return text, nil
	}

	var parts []FilePart
	result := text
	for _, token := range tokens {
		path := tokenToPath(token)
		if path == "" {
			continue
		}

		// Check for MCP resource reference: @mcp:server:uri
		if strings.HasPrefix(path, "mcp:") {
			if mcpReader == nil {
				continue
			}
			mcpRef := path[4:] // strip "mcp:"
			// Split into server:uri (first colon separates server from URI)
			serverName, uri, ok := strings.Cut(mcpRef, ":")
			if !ok || serverName == "" || uri == "" {
				continue // invalid format
			}

			textContent, blobData, mimeType, isBlob, err := mcpReader(serverName, uri)
			if err != nil {
				continue // skip on error, leave token as-is
			}

			if isBlob {
				// Binary MCP resource → extract as FilePart.
				filename := filepath.Base(uri)
				if filename == "." || filename == "/" {
					filename = serverName + "_resource"
				}
				parts = append(parts, FilePart{
					Filename:  filename,
					Data:      blobData,
					MediaType: mimeType,
				})
				result = strings.Replace(result, token, "", 1)
			} else {
				// Text MCP resource → inline as XML.
				wrapped := fmt.Sprintf("<resource uri=\"%s\" server=\"%s\">\n%s\n</resource>", uri, serverName, textContent)
				result = strings.Replace(result, token, wrapped, 1)
			}
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

		mediaType := detectMediaType(absPath, content)

		if isBinaryMediaType(mediaType) {
			// Binary file → extract as a FilePart for multimodal submission.
			// Remove the @token from the text.
			parts = append(parts, FilePart{
				Filename:  filepath.Base(absPath),
				Data:      content,
				MediaType: mediaType,
			})
			result = strings.Replace(result, token, "", 1)
		} else {
			// Text file → inline as XML-wrapped content.
			wrapped := wrapFileContent(absPath, content)
			result = strings.Replace(result, token, wrapped, 1)
		}
	}

	// Clean up any extra whitespace left by removed binary tokens.
	result = strings.TrimSpace(result)

	return result, parts
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

// detectMediaType determines the MIME type of a file using extension-based
// lookup first (more reliable for known types), then falls back to content
// sniffing via net/http.DetectContentType.
func detectMediaType(path string, content []byte) string {
	// Extension-based detection is more reliable for well-known types.
	ext := strings.ToLower(filepath.Ext(path))
	if mt := mime.TypeByExtension(ext); mt != "" {
		// mime.TypeByExtension returns types like "image/png; charset=utf-8"
		// — strip parameters.
		if base, _, ok := strings.Cut(mt, ";"); ok {
			return strings.TrimSpace(base)
		}
		return mt
	}

	// Known extensions that mime package may miss.
	switch ext {
	case ".webp":
		return "image/webp"
	case ".avif":
		return "image/avif"
	case ".heic", ".heif":
		return "image/heif"
	case ".opus":
		return "audio/opus"
	case ".flac":
		return "audio/flac"
	case ".m4a":
		return "audio/mp4"
	case ".wasm":
		return "application/wasm"
	}

	// Content sniffing fallback.
	if len(content) > 0 {
		detected := http.DetectContentType(content)
		if detected != "" && detected != "application/octet-stream" {
			if base, _, ok := strings.Cut(detected, ";"); ok {
				return strings.TrimSpace(base)
			}
			return detected
		}
	}

	// Default: treat as plain text so it gets XML-wrapped.
	return "text/plain"
}

// isBinaryMediaType returns true if the MIME type represents a binary file
// that should be sent as a multimodal FilePart rather than XML-wrapped text.
func isBinaryMediaType(mediaType string) bool {
	// Image types — always binary.
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	// Audio types — always binary.
	if strings.HasPrefix(mediaType, "audio/") {
		return true
	}
	// Video types — always binary.
	if strings.HasPrefix(mediaType, "video/") {
		return true
	}
	// Specific application types that are binary.
	switch mediaType {
	case "application/pdf",
		"application/zip",
		"application/gzip",
		"application/x-tar",
		"application/octet-stream",
		"application/wasm",
		"application/x-executable",
		"application/vnd.ms-excel",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-powerpoint",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"application/msword",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return true
	}
	return false
}
