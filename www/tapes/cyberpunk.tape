# KIT cyberpunk demo — records www/public/cyberpunk.gif + .webm
#
# Record with:  ./www/tapes/record.sh cyberpunk
#
# Everything the demo needs (model, persona, thinking level, ephemeral mode)
# is declared in tapes/demo/.kit.yml, which Kit picks up from the working
# directory. The only thing typed on camera is `kit`, then the conversation.
#
# Nothing here asks the agent to write code: LLM codegen latency is
# unpredictable (10s -> 2m+ observed for the same class of prompt), which makes
# fixed Sleep timings unrecordable. The replies are short and synchronised with
# Wait instead of guessed sleeps.

Output ../public/cyberpunk.gif
Output ../public/cyberpunk.webm

# --- Terminal setup -------------------------------------------------
Set Shell       "nu"
Set FontFamily  "Operator-caskabold"
Set FontSize    16
Set Width       1200
# Tall enough that a full exchange (question + reply) stays on screen at once,
# which the Wait sync below depends on.
Set Height      820
Set Padding     20
Set TypingSpeed 55ms
Set Framerate   24

# Synthwave palette, matched to Kit's built-in `synthwave` theme
# (internal/ui/style/themes.go) so the TUI and the terminal agree.
Set Theme { "name": "synthwave", "background": "#262335", "foreground": "#ffffff", "selection": "#463465", "cursor": "#36f9f6", "black": "#262335", "red": "#fe4450", "green": "#72f1b8", "yellow": "#fede5d", "blue": "#36f9f6", "magenta": "#ff7edb", "cyan": "#36f9f6", "white": "#ffffff", "brightBlack": "#848bbd", "brightRed": "#fe4450", "brightGreen": "#72f1b8", "brightYellow": "#fede5d", "brightBlue": "#36f9f6", "brightMagenta": "#ff7edb", "brightCyan": "#36f9f6", "brightWhite": "#ffffff" }

# record.sh runs tapes from www/tapes/ and builds a fresh kit into output/,
# but nushell rebuilds $env.PATH from its own config, discarding the exported
# PATH — without this prepend the tape silently records whatever `kit` is
# installed system-wide instead of the build under test.
# The right prompt is blanked because it renders a clock, which would make
# every re-recording differ.
Hide
Sleep 3s
Type "$env.PATH = ($env.PATH | prepend ('../../output' | path expand))" Enter
Type "$env.PROMPT_COMMAND_RIGHT = {|| '' }" Enter
Type "cd demo" Enter
Type "clear" Enter
Sleep 1s
Show

# --- Jack in ---------------------------------------------------------
# Opens in Kit's default `kitt` theme (record.sh pins it), so the switch below
# reads as a real before/after rather than depending on local preferences.
Sleep 800ms
Type "kit"
Sleep 600ms
Enter
Sleep 4s

# Live theme switch — a real Kit feature, and it recolours the whole TUI.
Type "/theme synthwave"
Sleep 900ms
Enter
Sleep 3s

# --- Exchange 1 ------------------------------------------------------
# Sync on the sentinel the system prompt guarantees, rather than guessing a
# Sleep: that holds even if the model is slow. The trailing Sleep is the
# actual read time for the viewer.
#
# Each Wait is anchored to its own question text, so it can only match the
# sentinel that follows *that* question. A bare /LINK SEVERED/ would match a
# previous reply and race ahead; counting sentinels instead is unreliable,
# because a long reply scrolls the earliest one off the top and the count
# never completes (observed as an intermittent recording failure).
Type "who are you, daemon?"
Sleep 500ms
Enter
Wait+Screen@60s /(?s)who are you, daemon\?.*LINK SEVERED/
Sleep 5s

# --- Exchange 2 ------------------------------------------------------
Type "why should i jack into kit?"
Sleep 500ms
Enter
Wait+Screen@60s /(?s)jack into kit\?.*LINK SEVERED/
Sleep 5s

# --- Exchange 3 ------------------------------------------------------
Type "what if i hand you legacy code?"
Sleep 500ms
Enter
Wait+Screen@60s /(?s)legacy code\?.*LINK SEVERED/
Sleep 6s
