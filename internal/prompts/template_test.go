package prompts

import (
	"testing"
)

func TestParseCommandArgs(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"hello", []string{"hello"}},
		{"hello world", []string{"hello", "world"}},
		{`"hello world"`, []string{"hello world"}},
		{`'hello world'`, []string{"hello world"}},
		{`hello "world foo" bar`, []string{"hello", "world foo", "bar"}},
		{`hello 'world foo' bar`, []string{"hello", "world foo", "bar"}},
		{`hello \"world\"`, []string{"hello", `"world"`}},
		{`hello \\world`, []string{"hello", `\world`}},
		{`  hello   world  `, []string{"hello", "world"}},
		{`Button "onClick handler" "disabled support"`, []string{"Button", "onClick handler", "disabled support"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseCommandArgs(tt.input)
			if len(got) != len(tt.expected) {
				t.Errorf("ParseCommandArgs(%q) = %v, want %v", tt.input, got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("ParseCommandArgs(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestSubstituteArgs(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		args     []string
		expected string
	}{
		{
			name:     "no placeholders",
			content:  "Hello world",
			args:     []string{},
			expected: "Hello world",
		},
		{
			name:     "positional $1",
			content:  "Hello $1",
			args:     []string{"world"},
			expected: "Hello world",
		},
		{
			name:     "positional $1 $2",
			content:  "$1 and $2",
			args:     []string{"first", "second"},
			expected: "first and second",
		},
		{
			name:     "missing arg",
			content:  "Hello $1 and $2",
			args:     []string{"world"},
			expected: "Hello world and ",
		},
		{
			name:     "$@ wildcard",
			content:  "Args: $@",
			args:     []string{"a", "b", "c"},
			expected: "Args: a b c",
		},
		{
			name:     "$ARGUMENTS wildcard",
			content:  "Args: $ARGUMENTS",
			args:     []string{"a", "b", "c"},
			expected: "Args: a b c",
		},
		{
			name:     "${@} all args",
			content:  "Args: ${@}",
			args:     []string{"a", "b", "c"},
			expected: "Args: a b c",
		},
		{
			name:     "${@:2} slice from index 2",
			content:  "Rest: ${@:2}",
			args:     []string{"a", "b", "c", "d"},
			expected: "Rest: b c d",
		},
		{
			name:     "${@:1:2} slice with length",
			content:  "First two: ${@:1:2}",
			args:     []string{"a", "b", "c", "d"},
			expected: "First two: a b",
		},
		{
			name:     "${@:0} from start",
			content:  "All: ${@:0}",
			args:     []string{"a", "b", "c"},
			expected: "All: a b c",
		},
		{
			name:     "${@:3:1} single arg",
			content:  "Third: ${@:3:1}",
			args:     []string{"a", "b", "c", "d"},
			expected: "Third: c",
		},
		{
			name:     "combined placeholders",
			content:  "Create $1 with features: $ARGUMENTS",
			args:     []string{"Button", "onClick", "disabled"},
			expected: "Create Button with features: Button onClick disabled",
		},
		{
			name:     "slice beyond bounds",
			content:  "${@:10}",
			args:     []string{"a", "b"},
			expected: "",
		},
		{
			name:     "empty args with wildcard",
			content:  "Args: $@",
			args:     []string{},
			expected: "Args: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SubstituteArgs(tt.content, tt.args)
			if got != tt.expected {
				t.Errorf("SubstituteArgs(%q, %v) = %q, want %q", tt.content, tt.args, got, tt.expected)
			}
		})
	}
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantDesc string
		wantErr  bool
	}{
		{
			name:     "simple description",
			content:  "description: Review code\n",
			wantDesc: "Review code",
		},
		{
			name:     "empty",
			content:  "",
			wantDesc: "",
		},
		{
			name:     "invalid yaml",
			content:  "description: [unclosed",
			wantDesc: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, err := ParseFrontmatter(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFrontmatter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if fm.Description != tt.wantDesc {
				t.Errorf("ParseFrontmatter() Description = %q, want %q", fm.Description, tt.wantDesc)
			}
		})
	}
}

func TestPromptTemplateExpand(t *testing.T) {
	tpl := &PromptTemplate{
		Name:        "component",
		Description: "Create a component",
		Content:     "Create a React component named $1 with features: $ARGUMENTS",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "Button",
			expected: "Create a React component named Button with features: Button",
		},
		{
			input:    `Button "onClick handler"`,
			expected: "Create a React component named Button with features: Button onClick handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tpl.Expand(tt.input)
			if got != tt.expected {
				t.Errorf("Expand(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestHasArgPlaceholders(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"no placeholders", "Just a plain prompt with no args", false},
		{"$1 placeholder", "Create a $1 component", true},
		{"$@ placeholder", "Run with args: $@", true},
		{"$ARGUMENTS placeholder", "Features: $ARGUMENTS", true},
		{"${1} placeholder", "Name: ${1}", true},
		{"${ARGUMENTS} placeholder", "All: ${ARGUMENTS}", true},
		{"${@:1} placeholder", "Rest: ${@:1}", true},
		{"${@:1:2} placeholder", "Slice: ${@:1:2}", true},
		{"dollar in text", "Cost is one hundred dollars", false},
		{"empty content", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := &PromptTemplate{Content: tt.content}
			if got := tpl.HasArgPlaceholders(); got != tt.want {
				t.Errorf("HasArgPlaceholders() = %v, want %v", got, tt.want)
			}
		})
	}
}
