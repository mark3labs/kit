# kit-telegram

A Kit extension that relays all Kit agent runs to Telegram and lets approved Telegram users reply back into Kit.

## What it does

- Relays **all Kit runs** to one Telegram chat while connected
- Edits one Telegram progress message in place during a run
- Lets approved Telegram users send normal text replies back into Kit
- Shows `Telegram Connected` or `Telegram Disconnected` in the status bar
- Shows a small spinner animation as `⠋ Telegram Connecting` only while the relay is still connecting
- On startup with an already validated enabled config, sends a short Telegram connection message to confirm the relay is up

## Requirements

- `kit` installed and working
- A Telegram bot token from `@BotFather`
- Either:
  - A Telegram chat where you can message the bot, or
  - A numeric Telegram chat id you want to enter manually
- For group chats, one or more allowed Telegram user ids

## Quickstart

### 1. Install the extension

```bash
kit install github.com/mark3labs/kit/examples/extensions/kit-telegram
```

Or run directly:
```bash
kit -e path/to/kit-telegram/main.go
```

### 2. Start Kit and connect Telegram

```bash
kit
```

Inside Kit, run:

```
/telegram connect
```

You will be prompted for:

- Bot token from `@BotFather`
- Whether to auto-detect the chat by messaging the bot or enter the chat id manually
- Allowed user ids when needed

### 3. Verify the relay

```
/telegram test
```

Reply in Telegram with the code from the test message.

## Commands

| Command | Description |
|---------|-------------|
| `/telegram` | Human-friendly overview and subcommand list |
| `/telegram status` | Raw deterministic relay state |
| `/telegram test` | Verify outbound and inbound relay |
| `/telegram toggle` | Enable or disable relay without deleting credentials |
| `/telegram logout` | Remove saved credentials and disconnect relay |
| `/telegram connect` | Run the setup flow again |
| `/telegram clear` | Clear Telegram status and working messages from the TUI |

## Remote commands (from Telegram)

| Command | Description |
|---------|-------------|
| `/telegram` | Sends the overview back to Telegram |
| `/telegram status` | Sends the deterministic state report to Telegram |
| `/telegram test` | Sends a reply-code test message from Telegram |
| `/telegram toggle` | Flips the enabled flag |
| `/telegram logout yes` | Logs out (requires `yes` confirmation) |
| `/telegram clear` | Clears the TUI footer and working messages |

## Key APIs Used

- `RegisterCommand` — Slash command with subcommands and tab completion
- `OnSessionStart` / `OnSessionShutdown` — Lifecycle management
- `OnAgentStart` / `OnAgentEnd` — Run tracking and progress rendering
- `OnToolCall` / `OnToolResult` — Action tracking
- `OnMessageEnd` — Capture assistant responses
- `OnInput` — Mirror local messages to Telegram
- `SetStatus` / `RemoveStatus` — Status bar indicators
- `SetWidget` / `RemoveWidget` — Working message display
- `PromptInput` / `PromptSelect` / `PromptConfirm` — Interactive setup flow
- `SendMessage` — Inject Telegram replies as Kit prompts

## Architecture

Single Go file interpreted by Yaegi at runtime. Core components:

- **Telegram Bot API client** — HTTP calls via `net/http` for getMe, getChat, getChatMember, getUpdates (long-polling), sendMessage, editMessageText
- **Config persistence** — JSON file at `.kit/kit-telegram.json` with atomic writes
- **Long-polling goroutine** — Background polling for Telegram updates with warmup poll, retry, and client-side timeouts
- **Message queue** — In-memory FIFO queue for Telegram prompt input with edit-before-dispatch support
- **Progress rendering** — `⏳ elapsed · step N` with action lines, edited in place
- **Final rendering** — `✅/❌ elapsed` with response text, split into chunks for long output

## Debug mode

Set environment variable `KIT_TELEGRAM_DEBUG=1` to enable verbose debug logging.
