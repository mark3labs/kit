package prompts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_Integration(t *testing.T) {
	// Create a temp directory for testing
	tempDir := t.TempDir()
	
	// Create the .kit/prompts subdirectory structure
	promptsDir := filepath.Join(tempDir, ".kit", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts dir: %v", err)
	}
	
	// Create a test template file
	templateContent := `---
description: Test template for integration
---
Review $1 with focus on $2`
	
	testFile := filepath.Join(promptsDir, "test.md")
	if err := os.WriteFile(testFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Test loading from the temp directory
	tpls, diags, err := LoadAll(LoadOptions{
		HomeDir:         tempDir,
		IncludeDefaults: false, // Skip default locations for this test
	})
	
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	
	if len(diags) > 0 {
		t.Logf("Got %d diagnostics", len(diags))
	}
	
	if len(tpls) != 1 {
		t.Fatalf("Expected 1 template, got %d", len(tpls))
	}
	
	tpl := tpls[0]
	if tpl.Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", tpl.Name)
	}
	
	if tpl.Description != "Test template for integration" {
		t.Errorf("Expected description 'Test template for integration', got '%s'", tpl.Description)
	}
	
	// Test expansion
	expanded := tpl.Expand("code security")
	expected := "Review code with focus on security"
	if expanded != expected {
		t.Errorf("Expected '%s', got '%s'", expected, expanded)
	}
}

func TestParseTemplate_WithFrontmatter(t *testing.T) {
	// Create a temp file with frontmatter
	tempDir := t.TempDir()
	templateContent := `---
description: A test template
---
Create a $1 component with $2 features`
	
	testFile := filepath.Join(tempDir, "component.md")
	if err := os.WriteFile(testFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tpl, err := ParseTemplate(testFile)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}
	
	if tpl.Name != "component" {
		t.Errorf("Expected name 'component', got '%s'", tpl.Name)
	}
	
	if tpl.Description != "A test template" {
		t.Errorf("Expected description 'A test template', got '%s'", tpl.Description)
	}
	
	expectedContent := "Create a $1 component with $2 features"
	if tpl.Content != expectedContent {
		t.Errorf("Expected content '%s', got '%s'", expectedContent, tpl.Content)
	}
}

func TestParseTemplate_WithoutFrontmatter(t *testing.T) {
	// Create a temp file without frontmatter
	tempDir := t.TempDir()
	templateContent := `Simple template without frontmatter
Supports $1 and $2 placeholders`
	
	testFile := filepath.Join(tempDir, "simple.md")
	if err := os.WriteFile(testFile, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	tpl, err := ParseTemplate(testFile)
	if err != nil {
		t.Fatalf("ParseTemplate failed: %v", err)
	}
	
	if tpl.Name != "simple" {
		t.Errorf("Expected name 'simple', got '%s'", tpl.Name)
	}
	
	// Description should be empty since there's no frontmatter
	if tpl.Description != "" {
		t.Errorf("Expected empty description, got '%s'", tpl.Description)
	}
	
	// Content should include everything
	if tpl.Content != templateContent {
		t.Errorf("Content mismatch\nExpected:\n%s\nGot:\n%s", templateContent, tpl.Content)
	}
}
