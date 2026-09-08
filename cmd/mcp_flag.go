package cmd

import (
	"fmt"
	"strings"

	"github.com/mark3labs/kit/internal/config"
)

// parseMCPFlag turns one --mcp value into a named MCP server config.
//
// The value has the form "name=spec". When spec starts with "http://" or
// "https://" the server is remote (streamable HTTP). Otherwise spec is a
// command line that starts a local stdio server. The command line is split
// on whitespace; single quotes, double quotes and backslash escapes keep
// spaces inside one argument.
//
//	--mcp 'browser=lightpanda mcp'
//	--mcp 'docs=https://mcp.example.com/mcp'
//	--mcp 'fs=npx -y @modelcontextprotocol/server-filesystem "/my dir"'
func parseMCPFlag(value string) (string, config.MCPServerConfig, error) {
	name, spec, found := strings.Cut(value, "=")
	name = strings.TrimSpace(name)
	spec = strings.TrimSpace(spec)
	if !found || name == "" || spec == "" {
		return "", config.MCPServerConfig{}, fmt.Errorf("invalid --mcp value %q: expected name=command or name=url", value)
	}
	if strings.ContainsAny(name, " \t/") {
		return "", config.MCPServerConfig{}, fmt.Errorf("invalid --mcp server name %q: must not contain spaces or '/'", name)
	}

	if strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
		return name, config.MCPServerConfig{Type: "remote", URL: spec}, nil
	}

	command, err := splitCommandLine(spec)
	if err != nil {
		return "", config.MCPServerConfig{}, fmt.Errorf("invalid --mcp command for %q: %w", name, err)
	}
	if len(command) == 0 || command[0] == "" {
		return "", config.MCPServerConfig{}, fmt.Errorf("invalid --mcp value %q: command is empty", value)
	}
	return name, config.MCPServerConfig{Type: "local", Command: command}, nil
}

// applyMCPFlags adds every --mcp server to cfg. A flag server replaces a
// config-file server with the same name. The merged config is validated
// again so a bad flag fails at startup and not at connect time.
func applyMCPFlags(cfg *config.Config, values []string) error {
	if len(values) == 0 {
		return nil
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]config.MCPServerConfig)
	}
	for _, value := range values {
		name, server, err := parseMCPFlag(value)
		if err != nil {
			return err
		}
		cfg.MCPServers[name] = server
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid --mcp config: %w", err)
	}
	return nil
}

// splitCommandLine splits s into arguments the way a POSIX shell splits a
// simple command: whitespace separates arguments, single quotes keep text
// literal, double quotes keep text but honour backslash escapes, and a
// backslash outside quotes escapes the next rune. No variable expansion or
// globbing is done.
func splitCommandLine(s string) ([]string, error) {
	var (
		args    []string
		current strings.Builder
		inArg   bool
		quote   rune // 0, '\'' or '"'
		escaped bool
	)

	for _, r := range s {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
			inArg = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			inArg = true
		case r == ' ' || r == '\t' || r == '\n':
			if inArg {
				args = append(args, current.String())
				current.Reset()
				inArg = false
			}
		default:
			current.WriteRune(r)
			inArg = true
		}
	}

	if escaped {
		return nil, fmt.Errorf("trailing backslash")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated %c quote", quote)
	}
	if inArg {
		args = append(args, current.String())
	}
	return args, nil
}
