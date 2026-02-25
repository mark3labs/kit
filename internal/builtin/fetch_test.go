package builtin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewFetchServer(t *testing.T) {
	server, err := NewFetchServer()
	if err != nil {
		t.Fatalf("Failed to create fetch server: %v", err)
	}

	if server == nil {
		t.Fatal("Expected server to be non-nil")
	}
}

func TestFetchServerRegistry(t *testing.T) {
	registry := NewRegistry()

	// Test that fetch server is registered
	servers := registry.ListServers()
	found := slices.Contains(servers, "fetch")

	if !found {
		t.Error("fetch server not found in registry")
	}

	// Test creating fetch server through registry
	wrapper, err := registry.CreateServer("fetch", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("Failed to create fetch server through registry: %v", err)
	}

	if wrapper == nil {
		t.Fatal("Expected wrapper to be non-nil")
	}

	if wrapper.GetServer() == nil {
		t.Fatal("Expected wrapped server to be non-nil")
	}
}

func TestFetchHTML(t *testing.T) {
	// Create a test HTTP server
	testHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
    <script>console.log('test');</script>
    <style>body { color: red; }</style>
</head>
<body>
    <h1>Hello World</h1>
    <p>This is a test paragraph.</p>
    <script>alert('should be removed');</script>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testHTML))
	}))
	defer server.Close()

	// Test HTML format
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    server.URL,
				"format": "html",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that we got HTML content
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty HTML content")
		}
		// Should contain the original HTML
		if !strings.Contains(textContent.Text, "<h1>Hello World</h1>") {
			t.Error("Expected HTML content to contain original markup")
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestFetchText(t *testing.T) {
	// Create a test HTTP server
	testHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
    <script>console.log('test');</script>
    <style>body { color: red; }</style>
</head>
<body>
    <h1>Hello World</h1>
    <p>This is a test paragraph.</p>
    <script>alert('should be removed');</script>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testHTML))
	}))
	defer server.Close()

	// Test text format
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    server.URL,
				"format": "text",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that we got text content
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty text content")
		}
		// Should contain the text but not HTML tags
		if !strings.Contains(textContent.Text, "Hello World") {
			t.Error("Expected text content to contain 'Hello World'")
		}
		if strings.Contains(textContent.Text, "<h1>") {
			t.Error("Expected text content to not contain HTML tags")
		}
		if strings.Contains(textContent.Text, "console.log") {
			t.Error("Expected text content to not contain script content")
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestFetchMarkdown(t *testing.T) {
	// Create a test HTTP server
	testHTML := `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Hello World</h1>
    <p>This is a test paragraph.</p>
    <ul>
        <li>Item 1</li>
        <li>Item 2</li>
    </ul>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testHTML))
	}))
	defer server.Close()

	// Test markdown format
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    server.URL,
				"format": "markdown",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that we got markdown content
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty markdown content")
		}
		// Should contain markdown formatting
		if !strings.Contains(textContent.Text, "# Hello World") {
			t.Error("Expected markdown content to contain '# Hello World'")
		}
	} else {
		t.Error("Expected text content")
	}
}

func TestFetchInvalidURL(t *testing.T) {
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    "not-a-valid-url",
				"format": "text",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return an error result
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

func TestFetchInvalidFormat(t *testing.T) {
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    "https://example.com",
				"format": "invalid",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should return an error result
	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text == "" {
			t.Error("Expected non-empty error message")
		}
	}
}

func TestFetchPlainText(t *testing.T) {
	// Create a test HTTP server that returns plain text
	testText := "This is plain text content.\nWith multiple lines.\nAnd some more text."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testText))
	}))
	defer server.Close()

	// Test text format with plain text content
	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "fetch",
			Arguments: map[string]any{
				"url":    server.URL,
				"format": "text",
			},
		},
	}

	ctx := context.Background()
	result, err := executeFetch(ctx, request)

	if err != nil {
		t.Fatalf("Failed to execute fetch: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result to be non-nil")
	}

	if len(result.Content) == 0 {
		t.Fatal("Expected result to have content")
	}

	// Check that we got the original text content
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		if textContent.Text != testText {
			t.Errorf("Expected '%s', got '%s'", testText, textContent.Text)
		}
	} else {
		t.Error("Expected text content")
	}
}
