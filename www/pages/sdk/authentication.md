---
title: SDK Authentication
description: Stored provider credentials and host-controlled login in Go.
ogImage: /og-image.png
---

# SDK authentication

## Stored credentials

`SetProviderAPIKey(providerID, apiKey)` stores a key in `$XDG_CONFIG_HOME/.kit/credentials.json` (0600 for a newly created file; existing permissions are preserved), or `~/.config/.kit/credentials.json`. Handle its error. Stored keys take precedence over provider environment variables. Explicit `Options.ProviderAPIKey` / `WithProviderAPIKey` overrides stored API keys and provider environment variables. Exception: stored OpenAI OAuth credentials take precedence at the default OpenAI destination or a trusted Codex endpoint (https://chatgpt.com/backend-api/codex); configuration can also supply provider settings. Do not log keys, tokens or the credential store.

`GetProviderAPIKey` reads only the stored key (empty when absent), not environment fallback. `HasProviderCredentials` reports stored API-key or OAuth presence, not remote validity. `RemoveProviderCredentials` deletes all stored credentials for that provider. Anthropic and OpenAI use dedicated slots; Copilot is OAuth-only and rejects API-key storage. `NewCredentialManager` exposes the store for advanced use; see its [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#CredentialManager) for provider-specific helpers.

## Startup without credentials

By default `New` fails fast. `IsMissingCredentialsError(err)` detects a wrapped `MissingCredentialsError`. `WithAllowMissingCredentials` / `Options.AllowMissingCredentials` tolerates provider-creation failures more broadly: missing credentials, failed refresh, unsupported providers and similar failures. It does not tolerate a malformed model string. Check `host.ProviderError()`; prompts fail with that error until `SetModel` successfully creates a provider. After storing credentials, call `SetModel` or create a new instance. This option does not authenticate or silently retry prompts.

## Host-controlled OpenAI login

`LoginProvider(ctx, "openai", LoginOptions{...})` supports only ChatGPT/Codex subscription authentication, not general OpenAI API OAuth. Anthropic and Copilot login are not supported by this function. The required `OnAuthorization(context.Context, LoginInstructions) error` callback receives a URL after the listener is ready. The host displays it or opens a browser; the SDK does neither. Honor cancellation and return callback errors.

The flow is bounded by five minutes or the caller's earlier deadline. It binds loopback `127.0.0.1:1455` and `[::1]:1455`, with redirect `http://localhost:1455/auth/callback`. At least one listener must bind, and the browser must reach it. Remote/headless hosts need loopback port forwarding before login. There is no manual code-entry fallback. Keep the credential store writable for token refresh. Existing instances need provider recreation through `SetModel` (or a new Kit).

The compile-checked [login example](https://github.com/mark3labs/kit/blob/master/pkg/kit/provider_login_example_test.go) shows a three-minute caller timeout and context-aware URL display. Normal tests compile but do not execute it. `LoginOptions.CredentialManager` selects a custom store; `LoginOptions.UserAgent` is independent of any instance identity.

Provider login is distinct from MCP OAuth. MCP servers use `MCPAuthHandler` and separate token storage: see [MCP OAuth](/sdk/options#mcp-oauth-authorization) and the [public API index](/sdk/api-reference).
