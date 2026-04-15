package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessFileAttachments_TextFile(t *testing.T) {
	// Create a temp text file
	dir := t.TempDir()
	textFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(textFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	text := "@" + textFile + " check this out"
	result := ProcessFileAttachments(text, dir)

	if len(result.FileParts) != 0 {
		t.Errorf("expected 0 FileParts for text file, got %d", len(result.FileParts))
	}
	if result.ProcessedText == text {
		t.Error("expected text file to be XML-wrapped, but got original text unchanged")
	}
	// Should contain XML wrapping
	if !contains(result.ProcessedText, "<file path=") {
		t.Error("expected XML <file> wrapping in processed text")
	}
	if !contains(result.ProcessedText, "hello world") {
		t.Error("expected file content in processed text")
	}
}

func TestProcessFileAttachments_BinaryFile(t *testing.T) {
	// Create a minimal PNG file (binary)
	dir := t.TempDir()
	pngFile := filepath.Join(dir, "image.png")
	// Minimal valid PNG header
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 1x1
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, // 8bit RGB
		0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, 0x54, // IDAT chunk
		0x08, 0xD7, 0x63, 0xF8, 0xCF, 0xC0, 0x00, 0x00,
		0x00, 0x02, 0x00, 0x01, 0xE2, 0x21, 0xBC, 0x33,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND chunk
		0xAE, 0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(pngFile, pngData, 0644); err != nil {
		t.Fatal(err)
	}

	text := "@" + pngFile + " what is this image?"
	result := ProcessFileAttachments(text, dir)

	if len(result.FileParts) != 1 {
		t.Fatalf("expected 1 FilePart for binary file, got %d", len(result.FileParts))
	}
	if result.FileParts[0].MediaType != "image/png" {
		t.Errorf("expected media type image/png, got %s", result.FileParts[0].MediaType)
	}
	if result.FileParts[0].Filename != "image.png" {
		t.Errorf("expected filename image.png, got %s", result.FileParts[0].Filename)
	}
	// The @token should be removed from the text
	if contains(result.ProcessedText, "@") && contains(result.ProcessedText, pngFile) {
		t.Error("expected @token to be removed from processed text for binary file")
	}
	if contains(result.ProcessedText, "what is this image?") {
		// Good, the prompt text should remain
	} else {
		t.Error("expected prompt text to remain in processed text")
	}
}

func TestProcessFileAttachments_MCPResource(t *testing.T) {
	// Test @mcp:server:uri token processing with a mock reader
	text := "@mcp:test-server:docs://readme tell me about this"
	reader := func(serverName, uri string) (string, []byte, string, bool, error) {
		if serverName != "test-server" || uri != "docs://readme" {
			t.Errorf("unexpected server/uri: %s/%s", serverName, uri)
		}
		return "Hello from MCP resource", nil, "text/plain", false, nil
	}

	result := ProcessFileAttachments(text, "/tmp", reader)

	if len(result.FileParts) != 0 {
		t.Errorf("expected 0 FileParts for text MCP resource, got %d", len(result.FileParts))
	}
	if !contains(result.ProcessedText, "<resource uri=\"docs://readme\" server=\"test-server\">") {
		t.Error("expected <resource> XML wrapping in processed text")
	}
	if !contains(result.ProcessedText, "Hello from MCP resource") {
		t.Error("expected MCP resource content in processed text")
	}
}

func TestProcessFileAttachments_MCPResource_Binary(t *testing.T) {
	// Test @mcp:server:uri token processing for a binary resource
	text := "@mcp:test-server:images://logo describe this"
	reader := func(serverName, uri string) (string, []byte, string, bool, error) {
		if serverName != "test-server" || uri != "images://logo" {
			t.Errorf("unexpected server/uri: %s/%s", serverName, uri)
		}
		return "", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png", true, nil
	}

	result := ProcessFileAttachments(text, "/tmp", reader)

	if len(result.FileParts) != 1 {
		t.Fatalf("expected 1 FilePart for binary MCP resource, got %d", len(result.FileParts))
	}
	if result.FileParts[0].MediaType != "image/png" {
		t.Errorf("expected media type image/png, got %s", result.FileParts[0].MediaType)
	}
	if result.FileParts[0].Filename != "logo" {
		t.Errorf("expected filename 'logo', got %s", result.FileParts[0].Filename)
	}
	// The @token should be removed from the text
	if contains(result.ProcessedText, "@mcp:") {
		t.Error("expected @mcp: token to be removed from processed text for binary resource")
	}
}

func TestProcessFileAttachments_NoReader(t *testing.T) {
	// Without an MCP reader, @mcp: tokens should be left as-is
	text := "@mcp:server:resource this is a test"
	result := ProcessFileAttachments(text, "/tmp")

	if len(result.FileParts) != 0 {
		t.Errorf("expected 0 FileParts, got %d", len(result.FileParts))
	}
	// The @mcp: token should remain unchanged since no reader was provided
	if result.ProcessedText != text {
		t.Errorf("expected text unchanged without reader, got: %s", result.ProcessedText)
	}
}

func TestDetectMediaType(t *testing.T) {
	tests := []struct {
		ext      string
		content  []byte
		expected string
	}{
		{".go", nil, "text/plain"}, // .go falls back to content sniffing → text/plain
		{".png", []byte{0x89, 0x50, 0x4E, 0x47}, "image/png"},
		{".jpg", []byte{0xFF, 0xD8, 0xFF}, "image/jpeg"},
		{".pdf", []byte{0x25, 0x50, 0x44, 0x46}, "application/pdf"},
		{".txt", []byte("hello"), "text/plain"},
		{".wav", nil, "audio/wav"},
		{".webp", nil, "image/webp"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := detectMediaType("test"+tt.ext, tt.content)
			if got != tt.expected {
				t.Errorf("detectMediaType(%q) = %q, want %q", tt.ext, got, tt.expected)
			}
		})
	}
}

func TestIsBinaryMediaType(t *testing.T) {
	tests := []struct {
		mimeType string
		expected bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"audio/wav", true},
		{"video/mp4", true},
		{"application/pdf", true},
		{"text/plain", false},
		{"text/go", false},
		{"application/json", false},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			got := isBinaryMediaType(tt.mimeType)
			if got != tt.expected {
				t.Errorf("isBinaryMediaType(%q) = %v, want %v", tt.mimeType, got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
