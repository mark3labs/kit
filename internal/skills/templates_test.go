package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// NewPromptTemplate
// ---------------------------------------------------------------------------

func TestNewPromptTemplate_ExtractsVariables(t *testing.T) {
	tpl := NewPromptTemplate("test", "Hello {{name}}, you are {{role}}.")
	if len(tpl.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(tpl.Variables))
	}
	if tpl.Variables[0] != "name" || tpl.Variables[1] != "role" {
		t.Errorf("Variables = %v, want [name role]", tpl.Variables)
	}
}

func TestNewPromptTemplate_DeduplicatesVariables(t *testing.T) {
	tpl := NewPromptTemplate("test", "{{x}} and {{x}} and {{y}}")
	if len(tpl.Variables) != 2 {
		t.Fatalf("expected 2 unique variables, got %d", len(tpl.Variables))
	}
}

func TestNewPromptTemplate_NoVariables(t *testing.T) {
	tpl := NewPromptTemplate("plain", "No variables here.")
	if len(tpl.Variables) != 0 {
		t.Errorf("expected 0 variables, got %d", len(tpl.Variables))
	}
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

func TestExpand_AllVariablesProvided(t *testing.T) {
	tpl := NewPromptTemplate("test", "Hello {{name}}, welcome to {{place}}.")
	result := tpl.Expand(map[string]string{
		"name":  "Alice",
		"place": "Wonderland",
	})
	want := "Hello Alice, welcome to Wonderland."
	if result != want {
		t.Errorf("Expand = %q, want %q", result, want)
	}
}

func TestExpand_MissingVariable_LeftAsIs(t *testing.T) {
	tpl := NewPromptTemplate("test", "Hello {{name}}, your {{role}}.")
	result := tpl.Expand(map[string]string{
		"name": "Bob",
	})
	want := "Hello Bob, your {{role}}."
	if result != want {
		t.Errorf("Expand = %q, want %q", result, want)
	}
}

func TestExpand_EmptyValues(t *testing.T) {
	tpl := NewPromptTemplate("test", "Value: {{val}}")
	result := tpl.Expand(map[string]string{})
	if result != "Value: {{val}}" {
		t.Errorf("Expand = %q, want unchanged", result)
	}
}

// ---------------------------------------------------------------------------
// ExpandStrict
// ---------------------------------------------------------------------------

func TestExpandStrict_AllProvided(t *testing.T) {
	tpl := NewPromptTemplate("test", "{{greeting}} {{target}}")
	result, err := tpl.ExpandStrict(map[string]string{
		"greeting": "Hi",
		"target":   "World",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hi World" {
		t.Errorf("ExpandStrict = %q, want %q", result, "Hi World")
	}
}

func TestExpandStrict_MissingVariable_Error(t *testing.T) {
	tpl := NewPromptTemplate("test", "{{a}} {{b}} {{c}}")
	_, err := tpl.ExpandStrict(map[string]string{"a": "1"})
	if err == nil {
		t.Error("expected error for missing variables")
	}
}

// ---------------------------------------------------------------------------
// LoadPromptTemplate
// ---------------------------------------------------------------------------

func TestLoadPromptTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "greeting.txt")
	content := "Hello {{name}}, you work on {{project}}."
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tpl, err := LoadPromptTemplate(path)
	if err != nil {
		t.Fatal(err)
	}
	if tpl.Name != "greeting" {
		t.Errorf("Name = %q, want %q", tpl.Name, "greeting")
	}
	if len(tpl.Variables) != 2 {
		t.Errorf("expected 2 variables, got %d", len(tpl.Variables))
	}
}

func TestLoadPromptTemplate_NonexistentFile(t *testing.T) {
	_, err := LoadPromptTemplate("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
