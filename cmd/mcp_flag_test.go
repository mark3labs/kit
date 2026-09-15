package cmd

import (
	"reflect"
	"testing"

	"github.com/mark3labs/kit/internal/config"
)

func TestSplitCommandLine(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{name: "simple", in: "lightpanda mcp", want: []string{"lightpanda", "mcp"}},
		{name: "extra whitespace", in: "  npx   -y  pkg\t", want: []string{"npx", "-y", "pkg"}},
		{name: "double quotes", in: `cmd "a b" c`, want: []string{"cmd", "a b", "c"}},
		{name: "single quotes", in: `cmd 'a b' c`, want: []string{"cmd", "a b", "c"}},
		{name: "backslash escape", in: `cmd a\ b`, want: []string{"cmd", "a b"}},
		{name: "backslash in single quotes is literal", in: `cmd 'a\b'`, want: []string{"cmd", `a\b`}},
		{name: "escape inside double quotes", in: `cmd "say \"hi\""`, want: []string{"cmd", `say "hi"`}},
		{name: "empty quoted arg", in: `cmd ""`, want: []string{"cmd", ""}},
		{name: "adjacent quotes join", in: `cmd a"b c"d`, want: []string{"cmd", "ab cd"}},
		{name: "empty", in: "", want: nil},
		{name: "unterminated double quote", in: `cmd "a`, wantErr: true},
		{name: "unterminated single quote", in: `cmd 'a`, wantErr: true},
		{name: "trailing backslash", in: `cmd a\`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitCommandLine(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitCommandLine(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitCommandLine(%q) unexpected error: %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitCommandLine(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseMCPFlag(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		wantName string
		want     config.MCPServerConfig
		wantErr  bool
	}{
		{
			name:     "stdio command",
			in:       "browser=lightpanda mcp",
			wantName: "browser",
			want:     config.MCPServerConfig{Type: "local", Command: []string{"lightpanda", "mcp"}},
		},
		{
			name:     "stdio command with quoted arg",
			in:       `fs=npx -y @modelcontextprotocol/server-filesystem "/my dir"`,
			wantName: "fs",
			want:     config.MCPServerConfig{Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-filesystem", "/my dir"}},
		},
		{
			name:     "https url",
			in:       "docs=https://mcp.example.com/mcp",
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp"},
		},
		{
			name:     "http url",
			in:       "local=http://127.0.0.1:8080/mcp",
			wantName: "local",
			want:     config.MCPServerConfig{Type: "remote", URL: "http://127.0.0.1:8080/mcp"},
		},
		{
			name:     "url with query keeps extra equals",
			in:       "docs=https://mcp.example.com/mcp?token=abc=def",
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp?token=abc=def"},
		},
		{
			name:     "surrounding whitespace trimmed",
			in:       "  browser = lightpanda mcp ",
			wantName: "browser",
			want:     config.MCPServerConfig{Type: "local", Command: []string{"lightpanda", "mcp"}},
		},
		{
			name:     "url with one header",
			in:       `docs=https://mcp.example.com/mcp -H "Authorization: Bearer abc"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"Authorization: Bearer abc"}, NoOAuth: true},
		},
		{
			name:     "url with several headers",
			in:       `docs=https://mcp.example.com/mcp -H "X-Api-Key: secret" --header "X-Tenant: acme"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Api-Key: secret", "X-Tenant: acme"}, NoOAuth: true},
		},
		{
			name:     "header with equals form",
			in:       `docs=https://mcp.example.com/mcp -H="X-Api-Key: secret" --header="X-Tenant: acme"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Api-Key: secret", "X-Tenant: acme"}, NoOAuth: true},
		},
		{
			name:     "header spacing normalized",
			in:       `docs=https://mcp.example.com/mcp -H "X-Api-Key:secret"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Api-Key: secret"}, NoOAuth: true},
		},
		{
			name:     "header value keeps colons",
			in:       `docs=https://mcp.example.com/mcp -H "X-Trace: a:b:c"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Trace: a:b:c"}, NoOAuth: true},
		},
		{
			name:     "header env default used",
			in:       `docs=https://mcp.example.com/mcp -H "X-Api-Key: ${env://KIT_TEST_UNSET_KEY:-fallback}"`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Api-Key: fallback"}, NoOAuth: true},
		},
		{
			name:     "headers with explicit oauth keep oauth on",
			in:       `docs=https://mcp.example.com/mcp -H "X-Tenant: acme" --oauth`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", Headers: []string{"X-Tenant: acme"}},
		},
		{
			name:     "no-oauth without headers",
			in:       `docs=https://mcp.example.com/mcp --no-oauth`,
			wantName: "docs",
			want:     config.MCPServerConfig{Type: "remote", URL: "https://mcp.example.com/mcp", NoOAuth: true},
		},
		{name: "header without colon", in: `docs=https://mcp.example.com/mcp -H nocolon`, wantErr: true},
		{name: "header without name", in: `docs=https://mcp.example.com/mcp -H ": value"`, wantErr: true},
		{name: "header name with space", in: `docs=https://mcp.example.com/mcp -H "Bad Key: value"`, wantErr: true},
		{name: "header flag without value", in: `docs=https://mcp.example.com/mcp -H`, wantErr: true},
		{name: "unknown flag after url", in: `docs=https://mcp.example.com/mcp --bogus x`, wantErr: true},
		{name: "bare argument after url", in: `docs=https://mcp.example.com/mcp extra`, wantErr: true},
		{name: "missing env var in header", in: `docs=https://mcp.example.com/mcp -H "X-Api-Key: ${env://KIT_TEST_UNSET_KEY}"`, wantErr: true},
		{name: "missing equals", in: "lightpanda mcp", wantErr: true},
		{name: "empty name", in: "=lightpanda mcp", wantErr: true},
		{name: "empty spec", in: "browser=", wantErr: true},
		{name: "name with space", in: "my browser=lightpanda mcp", wantErr: true},
		{name: "name with slash", in: "a/b=lightpanda mcp", wantErr: true},
		{name: "bad quoting", in: `browser=lightpanda "mcp`, wantErr: true},
		{name: "only quotes", in: `browser=""`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, got, err := parseMCPFlag(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMCPFlag(%q) = %q, %#v, want error", tt.in, gotName, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMCPFlag(%q) unexpected error: %v", tt.in, err)
			}
			if gotName != tt.wantName {
				t.Errorf("parseMCPFlag(%q) name = %q, want %q", tt.in, gotName, tt.wantName)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseMCPFlag(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestApplyMCPFlags(t *testing.T) {
	t.Run("no flags is a no-op", func(t *testing.T) {
		cfg := &config.Config{}
		if err := applyMCPFlags(cfg, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MCPServers != nil {
			t.Errorf("MCPServers = %#v, want nil", cfg.MCPServers)
		}
	})

	t.Run("adds servers to nil map", func(t *testing.T) {
		cfg := &config.Config{}
		err := applyMCPFlags(cfg, []string{"browser=lightpanda mcp", "docs=https://mcp.example.com/mcp"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.MCPServers) != 2 {
			t.Fatalf("got %d servers, want 2", len(cfg.MCPServers))
		}
		if got := cfg.MCPServers["browser"].Command; !reflect.DeepEqual(got, []string{"lightpanda", "mcp"}) {
			t.Errorf("browser command = %#v", got)
		}
		if got := cfg.MCPServers["docs"].URL; got != "https://mcp.example.com/mcp" {
			t.Errorf("docs url = %q", got)
		}
	})

	t.Run("flag replaces config file server with same name", func(t *testing.T) {
		cfg := &config.Config{MCPServers: map[string]config.MCPServerConfig{
			"browser": {Type: "local", Command: []string{"old", "server"}},
			"other":   {Type: "remote", URL: "https://other.example.com"},
		}}
		if err := applyMCPFlags(cfg, []string{"browser=lightpanda mcp"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := cfg.MCPServers["browser"].Command; !reflect.DeepEqual(got, []string{"lightpanda", "mcp"}) {
			t.Errorf("browser command = %#v, want flag value", got)
		}
		if _, ok := cfg.MCPServers["other"]; !ok {
			t.Errorf("config-file server 'other' was dropped")
		}
	})

	t.Run("last duplicate flag wins", func(t *testing.T) {
		cfg := &config.Config{}
		if err := applyMCPFlags(cfg, []string{"b=first", "b=second"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := cfg.MCPServers["b"].Command; !reflect.DeepEqual(got, []string{"second"}) {
			t.Errorf("command = %#v, want [second]", got)
		}
	})

	t.Run("remote headers reach the server config", func(t *testing.T) {
		t.Setenv("KIT_TEST_MCP_KEY", "s3cret")
		cfg := &config.Config{}
		err := applyMCPFlags(cfg, []string{`docs=https://mcp.example.com/mcp -H "Authorization: Bearer ${env://KIT_TEST_MCP_KEY}"`})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"Authorization: Bearer s3cret"}
		if got := cfg.MCPServers["docs"].Headers; !reflect.DeepEqual(got, want) {
			t.Errorf("docs headers = %#v, want %#v", got, want)
		}
		if got := cfg.MCPServers["docs"].URL; got != "https://mcp.example.com/mcp" {
			t.Errorf("docs url = %q", got)
		}
	})

	t.Run("bad flag returns error", func(t *testing.T) {
		cfg := &config.Config{}
		if err := applyMCPFlags(cfg, []string{"nope"}); err == nil {
			t.Fatal("expected error for value without '='")
		}
	})
}
