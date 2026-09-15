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
// A remote spec can carry HTTP headers after the URL with -H or --header.
// Each header is one "Key: Value" string, and its value supports the same
// ${env://VAR} substitution as the config file. Headers switch OAuth off for
// that server, because the headers are the credential; --oauth keeps OAuth on
// and --no-oauth switches it off without headers.
//
//	--mcp 'browser=lightpanda mcp'
//	--mcp 'docs=https://mcp.example.com/mcp'
//	--mcp 'docs=https://mcp.example.com/mcp -H "Authorization: Bearer ${env://API_KEY}"'
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
		server, err := parseRemoteMCPSpec(name, spec)
		if err != nil {
			return "", config.MCPServerConfig{}, err
		}
		return name, server, nil
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

// parseRemoteMCPSpec turns a remote spec into a server config. The spec is
// the URL, optionally followed by -H/--header, --oauth or --no-oauth
// arguments.
func parseRemoteMCPSpec(name, spec string) (config.MCPServerConfig, error) {
	tokens, err := splitCommandLine(spec)
	if err != nil {
		return config.MCPServerConfig{}, fmt.Errorf("invalid --mcp url for %q: %w", name, err)
	}
	if len(tokens) == 0 || tokens[0] == "" {
		return config.MCPServerConfig{}, fmt.Errorf("invalid --mcp value for %q: url is empty", name)
	}

	headers, oauth, err := parseRemoteMCPArgs(tokens[1:])
	if err != nil {
		return config.MCPServerConfig{}, fmt.Errorf("invalid --mcp value for %q: %w", name, err)
	}

	server := config.MCPServerConfig{Type: "remote", URL: tokens[0], Headers: headers}

	// Headers are the credential for this server, so OAuth stays off unless
	// the user asks for it. Without that rule Kit would try dynamic client
	// registration first and fail before the headers are ever sent.
	switch {
	case oauth != nil:
		server.NoOAuth = !*oauth
	case len(headers) > 0:
		server.NoOAuth = true
	}

	return server, nil
}

// parseRemoteMCPArgs reads the arguments that follow a remote URL: header
// flags ("-H VALUE", "-H=VALUE", "--header VALUE", "--header=VALUE") and the
// OAuth switches --oauth / --no-oauth. The returned oauth value is nil when
// neither switch is present.
func parseRemoteMCPArgs(args []string) ([]string, *bool, error) {
	var (
		headers []string
		oauth   *bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var raw string
		switch {
		case arg == "--oauth":
			enabled := true
			oauth = &enabled
			continue
		case arg == "--no-oauth":
			enabled := false
			oauth = &enabled
			continue
		case arg == "-H" || arg == "--header":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("%s needs a \"Key: Value\" argument", arg)
			}
			i++
			raw = args[i]
		case strings.HasPrefix(arg, "-H="):
			raw = strings.TrimPrefix(arg, "-H=")
		case strings.HasPrefix(arg, "--header="):
			raw = strings.TrimPrefix(arg, "--header=")
		default:
			return nil, nil, fmt.Errorf("unexpected argument %q after the url: only -H/--header, --oauth and --no-oauth are supported", arg)
		}

		header, err := normalizeMCPHeader(raw)
		if err != nil {
			return nil, nil, err
		}
		headers = append(headers, header)
	}
	return headers, oauth, nil
}

// normalizeMCPHeader validates one "Key: Value" header and expands
// ${env://VAR} references in its value.
func normalizeMCPHeader(raw string) (string, error) {
	key, value, found := strings.Cut(raw, ":")
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if !found || key == "" {
		return "", fmt.Errorf("invalid header %q: expected \"Key: Value\"", raw)
	}
	if strings.ContainsAny(key, " \t") {
		return "", fmt.Errorf("invalid header name %q: must not contain spaces", key)
	}

	substituter := &config.EnvSubstituter{}
	expanded, err := substituter.SubstituteEnvVars(value)
	if err != nil {
		return "", fmt.Errorf("invalid header %q: %w", raw, err)
	}

	return key + ": " + expanded, nil
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
