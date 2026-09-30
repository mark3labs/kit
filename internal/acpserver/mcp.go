package acpserver

import (
	"fmt"
	"strings"

	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// mcpServerConfig converts an MCP server entry from session/new,
// session/load or session/resume into a Kit MCP server config. It returns
// the server name and the config.
//
// The spec requires every agent to support the stdio transport. Kit also
// supports HTTP (streamable) and SSE, so Initialize advertises both.
func mcpServerConfig(srv acp.McpServer) (string, kit.MCPServerConfig, error) {
	switch {
	case srv.Stdio != nil:
		s := srv.Stdio
		if s.Name == "" || s.Command == "" {
			return "", kit.MCPServerConfig{}, fmt.Errorf("stdio MCP server needs a name and a command")
		}
		cfg := kit.MCPServerConfig{
			Type:    "local",
			Command: append([]string{s.Command}, s.Args...),
		}
		if len(s.Env) > 0 {
			cfg.Environment = make(map[string]string, len(s.Env))
			for _, e := range s.Env {
				cfg.Environment[e.Name] = e.Value
			}
		}
		return s.Name, cfg, nil

	case srv.Http != nil:
		s := srv.Http
		if s.Name == "" || s.Url == "" {
			return "", kit.MCPServerConfig{}, fmt.Errorf("http MCP server needs a name and a url")
		}
		return s.Name, kit.MCPServerConfig{
			Type:    "remote",
			URL:     s.Url,
			Headers: httpHeaders(s.Headers),
		}, nil

	case srv.Sse != nil:
		s := srv.Sse
		if s.Name == "" || s.Url == "" {
			return "", kit.MCPServerConfig{}, fmt.Errorf("sse MCP server needs a name and a url")
		}
		return s.Name, kit.MCPServerConfig{
			Transport: "sse",
			URL:       s.Url,
			Headers:   httpHeaders(s.Headers),
		}, nil

	case srv.Acp != nil:
		// MCP-over-ACP is not advertised (mcpCapabilities.acp is false).
		return "", kit.MCPServerConfig{}, fmt.Errorf("MCP server %q uses the unsupported acp transport", srv.Acp.Name)
	}
	return "", kit.MCPServerConfig{}, fmt.Errorf("MCP server entry has no known transport")
}

// httpHeaders converts ACP headers to Kit's "Name: Value" form.
func httpHeaders(headers []acp.HttpHeader) []string {
	if len(headers) == 0 {
		return nil
	}
	out := make([]string, 0, len(headers))
	for _, h := range headers {
		name := strings.TrimSpace(h.Name)
		if name == "" {
			continue
		}
		out = append(out, name+": "+h.Value)
	}
	return out
}
