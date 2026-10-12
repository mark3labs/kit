# Experimental model modes

Kit reads named modes from models.dev `experimental.modes`. Mode availability depends on the provider adapter, not only the catalog.

## CLI and TUI

Use `--model-mode ultrafast` at startup, or set `model_mode: ultrafast` in configuration.

In the TUI:

- `/mode` opens the selector.
- `/mode ultrafast` selects a named mode.
- `/mode default` removes mode overrides.

The selector marks modes as experimental, shows estimated input/output prices per million tokens, and disables unsupported modes with a reason. The status bar shows the active named mode. Model changes reset the mode. Reasoning changes retain it.

## SDK

Set `Options.ModelMode` at startup, or use `WithModelMode` with the functional options API.

```go
modes := k.ModelModes() // Includes unavailable catalog modes and reasons.
supported := k.SupportedModelModes()
_ = modes
_ = supported
err := k.SetModelMode(ctx, "ultrafast")
// An empty string restores the model default.
```

`CurrentModelMode` returns the selected mode. Runtime mode changes are rejected during an active turn. Unknown or unsupported modes return an error. Mode and thinking level are separate settings.

## Current adapter limits

The pinned provider library supports OpenAI Responses service-tier settings. Kit permits `ultrafast` and `auto`, and permits `priority` only for models accepted by the library's current model guard. GPT-6 priority modes are unavailable because that guard would discard the setting. OpenAI `reasoning.mode: pro` is unavailable.

Anthropic body overrides use the library's `ExtraBody` field. Modes that require custom headers are currently unavailable. Other providers and custom provider factories do not yet support request overrides for catalog modes.

No JSON request-body transport rewriting is used. Request tests verify that OpenAI ultrafast is serialized for both generation and streaming. These tests verify request construction, not account entitlement or server acceptance.

Mode prices are estimates. The TUI uses effective mode prices for new usage after a successful mode change; prior usage keeps its existing cost totals.
