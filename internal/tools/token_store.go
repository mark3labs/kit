package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mark3labs/mcp-go/client/transport"
)

// Compile-time check that FileTokenStore implements transport.TokenStore.
var _ transport.TokenStore = (*FileTokenStore)(nil)

// FileTokenStore is a file-backed implementation of transport.TokenStore that
// persists OAuth tokens as JSON on disk. Tokens are stored in a shared JSON file
// keyed by server URL, allowing multiple MCP servers to maintain independent tokens.
//
// The token file is located at $XDG_CONFIG_HOME/.kit/mcp_tokens.json, falling back
// to ~/.config/.kit/mcp_tokens.json when XDG_CONFIG_HOME is not set.
//
// FileTokenStore is safe for concurrent use.
type FileTokenStore struct {
	serverKey string
	filePath  string
	mu        sync.RWMutex
}

// NewFileTokenStore creates a new FileTokenStore for the given server URL.
// The serverKey is used as the map key in the shared token file, and should
// typically be the MCP server's base URL.
//
// Returns an error if the token file path cannot be resolved.
func NewFileTokenStore(serverKey string) (*FileTokenStore, error) {
	filePath, err := resolveTokenFilePath()
	if err != nil {
		return nil, fmt.Errorf("resolving token file path: %w", err)
	}

	return &FileTokenStore{
		serverKey: serverKey,
		filePath:  filePath,
	}, nil
}

// GetToken returns the stored token for this store's server key.
// Returns transport.ErrNoToken if no token exists for the server key or if
// the token file does not yet exist.
// Returns context.Canceled or context.DeadlineExceeded if the context is done.
func (s *FileTokenStore) GetToken(ctx context.Context) (*transport.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	tokens, err := readTokenFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, transport.ErrNoToken
		}
		return nil, fmt.Errorf("reading token file: %w", err)
	}

	token, ok := tokens[s.serverKey]
	if !ok {
		return nil, transport.ErrNoToken
	}

	return token, nil
}

// SaveToken persists the given token for this store's server key.
// If the token file or its parent directories do not exist, they are created.
// Existing tokens for other server keys are preserved.
// Returns context.Canceled or context.DeadlineExceeded if the context is done.
func (s *FileTokenStore) SaveToken(ctx context.Context, token *transport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tokens, err := readTokenFile(s.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading token file: %w", err)
	}
	if tokens == nil {
		tokens = make(map[string]*transport.Token)
	}

	tokens[s.serverKey] = token

	if err := writeTokenFile(s.filePath, tokens); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}

	return nil
}

// resolveTokenFilePath determines the path to the token file using
// XDG_CONFIG_HOME if set, otherwise falling back to ~/.config/.kit/.
func resolveTokenFilePath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("determining user home directory: %w", err)
		}
		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, ".kit", "mcp_tokens.json"), nil
}

// readTokenFile reads and unmarshals the token file into a server-keyed map.
// Returns os.ErrNotExist (via os.IsNotExist) if the file does not exist.
func readTokenFile(path string) (map[string]*transport.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tokens map[string]*transport.Token
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, fmt.Errorf("unmarshaling token file: %w", err)
	}

	return tokens, nil
}

// writeTokenFile marshals the token map and writes it to disk, creating
// parent directories as needed. The file is written with 0600 permissions
// to protect sensitive token data.
func writeTokenFile(path string, tokens map[string]*transport.Token) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating token directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling tokens: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing token file %s: %w", path, err)
	}

	return nil
}
