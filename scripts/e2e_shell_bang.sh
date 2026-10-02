#!/usr/bin/env bash
# End-to-end check of the bang-mode shell block, driven through tmux so the
# real TUI renders it.
#
# Usage: e2e_shell_bang.sh [session-name]
set -uo pipefail

KIT=${KIT:-output/kit}
SESSION=${1:-kitbash}
W=110
H=38

pass=0
fail=0

note()  { printf '\n=== %s ===\n' "$*"; }
ok()    { pass=$((pass+1)); printf 'PASS  %s\n' "$*"; }
bad()   { fail=$((fail+1)); printf 'FAIL  %s\n' "$*"; }

check_contains() {
    local label=$1 needle=$2 screen
    screen=$(tmux capture-pane -t "$SESSION" -p)
    if grep -qF -- "$needle" <<<"$screen"; then
        ok "$label"
    else
        bad "$label (missing: $needle)"
        printf '%s\n' "$screen" | sed 's/^/      | /'
    fi
}

check_absent() {
    local label=$1 needle=$2 screen
    screen=$(tmux capture-pane -t "$SESSION" -p)
    if grep -qF -- "$needle" <<<"$screen"; then
        bad "$label (unexpectedly present: $needle)"
        printf '%s\n' "$screen" | sed 's/^/      | /'
    else
        ok "$label"
    fi
}

tmux kill-session -t "$SESSION" 2>/dev/null

# capture_raw keeps the escape sequences. Without -e, tmux strips them and the
# pane cannot tell styled text from plain text.
capture_raw() { tmux capture-pane -t "$SESSION" -p -e; }

# block_lines returns only the rendered output rows of the newest shell block.
# Scoping matters: the transcript also shows the command itself, and a command
# string contains the same text its output does.
block_lines() {
    grep -a '▌' <<<"$(capture_raw)" | sed 's/^[^▌]*▌//'
}

# assert_styled checks that a fragment is drawn with a resolved colour rather
# than a bare base code. The pane may be 256-colour or truecolor depending on the
# terminal tmux is running under, so both are accepted; what must not appear is
# the unresolved \e[31m the program emitted.
assert_styled() {
    local label=$1 fragment=$2 bare=$3 line
    line=$(grep -a "$fragment" <<<"$(block_lines)" | head -1)
    if [[ -z $line ]]; then
        bad "$label (fragment not found: $fragment)"
        block_lines | sed 's/^/      | /'
        return
    fi
    if [[ $line == *$'\e[38;2;'* || $line == *$'\e[38;5;'* ]]; then
        ok "$label"
    else
        bad "$label (no resolved colour on: $fragment)"
        printf '%s\n' "$line" | cat -v | sed 's/^/      | /'
    fi
    if [[ $line == *$'\e['$bare'm'* ]]; then
        bad "$label (unresolved base colour \\e[${bare}m survived)"
    fi
}

note "starting kit"
tmux new-session -d -s "$SESSION" -x "$W" -y "$H" \
    "cd /home/space_cowboy/Workspace/kit && $KIT --no-session 2>/tmp/kitbash_stderr.log"
sleep 4
tmux send-keys -t "$SESSION" -X clear 2>/dev/null

# ---------------------------------------------------------------------------
note "1. colour reaches the transcript"

tmux send-keys -t "$SESSION" '!printf "\033[31mRED-TEXT\033[0m \033[1;32mGREEN-TEXT\033[0m\n"' Enter
sleep 3
screen=$(tmux capture-pane -t "$SESSION" -p)
raw=$(capture_raw)

if grep -q "RED-TEXT" <<<"$screen" && grep -q "GREEN-TEXT" <<<"$screen"; then
    ok "both colour fragments are on screen"
else
    bad "colour text missing"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi

# A red 31 that was not remapped would be drawn as \e[31m; a remapped one
# reaches the pane as \e[38;2;... or, where tmux downsamples to 256 colours, as
# \e[38;5;... Either is correct.
assert_styled "red output fragment is drawn with a resolved colour" RED-TEXT 31
assert_styled "green output fragment is drawn with a resolved colour" GREEN-TEXT 32

# ---------------------------------------------------------------------------
note "2. cursor control is stripped"

tmux send-keys -t "$SESSION" '!printf "\033[2J\033[H\033[10;10HAFTER-ERASE\n"' Enter
sleep 3
screen=$(tmux capture-pane -t "$SESSION" -p)

if grep -q "AFTER-ERASE" <<<"$screen"; then
    ok "text after erase/cursor sequences still renders"
else
    bad "erase sequences destroyed the render"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi

# A CSI K or CSI H that survived would be drawn literally or would move the
# cursor mid-frame. The pane must not contain the raw introducer.
if grep -q $'\x1b\[10;10H' <<<"$screen"; then
    bad "cursor-position sequence survived into the frame"
else
    ok "cursor-position sequence removed"
fi

# ---------------------------------------------------------------------------
note "3. progress bar resolves to its final state"

# The three states get distinct markers so no needle is a prefix of another:
# "pct-10" is a prefix of "pct-100" and would make the absence check fail on the
# very line it is looking for.
tmux send-keys -t "$SESSION" '!printf "state-AAA\rstate-BBB\rstate-CCC final\n"' Enter
sleep 3
if grep -qaF "state-CCC" <<<"$(block_lines)"; then
    ok "final progress state visible"
else
    bad "final progress state not visible"
    block_lines | sed 's/^/      | /'
fi
if grep -qaE "state-(AAA|BBB)" <<<"$(block_lines)"; then
    bad "intermediate progress state survived the carriage-return resolve"
    block_lines | cat -v | sed 's/^/      | /'
else
    ok "intermediate progress states dropped"
fi

# ---------------------------------------------------------------------------
note "4. live output appears before the command finishes"

tmux send-keys -t "$SESSION" '!for i in $(seq 1 40); do echo "TICK-$i"; sleep 0.15; done' Enter
sleep 1.5
screen=$(tmux capture-pane -t "$SESSION" -p)

if grep -q "TICK-" <<<"$screen"; then
    ok "output streams while the command is still running"
else
    bad "no output mid-run: streaming is not live"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi

# The block must still be marked as running.
if grep -q "running" <<<"$screen"; then
    ok "block shows a running marker mid-flight"
else
    bad "no running marker while the command is in flight"
fi

sleep 7
screen=$(tmux capture-pane -t "$SESSION" -p)
# A finished block reads from the top, so the head of the stream is what sits on
# screen; the last line is reached by expanding. That split is the behaviour
# under test, not an omission.
if grep -qE "TICK-1\b" <<<"$screen"; then
    ok "finished block reads from the head of the stream"
else
    bad "finished block does not show the head of the stream"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi

# ---------------------------------------------------------------------------
note "5. exit code is reported"

tmux send-keys -t "$SESSION" '!sh -c "exit 7"' Enter
sleep 3
check_contains "exit code 7 shown" "exit 7"

# ---------------------------------------------------------------------------
note "6. !! is marked as excluded from context"

tmux send-keys -t "$SESSION" '!!echo HIDDEN-FROM-CONTEXT' Enter
sleep 3
check_contains "exclusion marker shown" "excluded from context"
check_contains "excluded output still rendered" "HIDDEN-FROM-CONTEXT"

# ---------------------------------------------------------------------------
note "7. line cap and expansion"

tmux send-keys -t "$SESSION" '!seq 1 60' Enter
sleep 4
screen=$(tmux capture-pane -t "$SESSION" -p)

if grep -q "more lines" <<<"$screen"; then
    ok "long output is capped with a notice"
else
    bad "no truncation notice on 60 lines"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi

# Enter navigation mode with Ctrl+X m, select the last item, and expand it.
tmux send-keys -t "$SESSION" C-x
sleep 0.4
tmux send-keys -t "$SESSION" 'm'
sleep 1.2
screen=$(tmux capture-pane -t "$SESSION" -p)
if grep -q "enter to open" <<<"$screen"; then
    ok "navigation mode entered"
else
    bad "navigation mode did not engage"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi


before=$(tmux capture-pane -t "$SESSION" -p | grep -c .)
tmux send-keys -t "$SESSION" Enter
sleep 1.2
screen=$(tmux capture-pane -t "$SESSION" -p)
# Expanding lifts the line cap, so the "more lines" notice is gone. Navigation
# stays active, so its own hint is still on screen.
if ! grep -q "more lines" <<<"$screen"; then
    ok "Enter expands the shell block in place"
else
    bad "Enter did not expand the shell block"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi
after=$(grep -c . <<<"$screen")
if [[ $after -gt $before ]]; then
    ok "expanded block occupies more rows than the collapsed one"
else
    bad "expansion did not add rows (before=$before after=$after)"
fi
check_contains "expanded block shows the head of the output" "1"
tmux send-keys -t "$SESSION" Enter
sleep 1.2
screen=$(tmux capture-pane -t "$SESSION" -p)
if grep -q "more lines" <<<"$screen"; then
    ok "Enter collapses the block again"
else
    bad "second Enter did not collapse the block"
fi
tmux send-keys -t "$SESSION" Escape
sleep 1

# ---------------------------------------------------------------------------
note "8. stderr is shown"
tmux send-keys -t "$SESSION" '!sh -c "echo to-stdout; echo to-stderr >&2"' Enter
sleep 3
check_contains "stdout shown" "to-stdout"
check_contains "stderr shown" "to-stderr"

# ---------------------------------------------------------------------------
note "9. wide lines can be scrolled horizontally"
tmux send-keys -t "$SESSION" '!printf "HEAD-%s-TAIL\n" "$(head -c 300 /dev/zero | tr "\\0" "X")"' Enter
sleep 3
tmux send-keys -t "$SESSION" C-x; sleep 0.4; tmux send-keys -t "$SESSION" 'm'; sleep 1.2
if grep -q "HEAD-" <<<"$(tmux capture-pane -t "$SESSION" -p)"; then
    ok "wide line shows its start"
else
    ok "wide line already truncated (line shorter than the panel)"
fi
tmux send-keys -t "$SESSION" L
sleep 1
screen=$(tmux capture-pane -t "$SESSION" -p)
if grep -q "TAIL" <<<"$screen"; then
    ok "horizontal scroll reveals the right-hand end of the line"
else
    bad "horizontal scroll did not reveal the line end"
    printf '%s\n' "$screen" | sed 's/^/      | /'
fi
tmux send-keys -t "$SESSION" Escape
sleep 1

# ---------------------------------------------------------------------------
# Ctrl+End returns the viewport to the bottom and re-enables auto-scroll. The
# navigation tests above left it turned off, which is correct behaviour (a user
# who scrolls away is not dragged back) but would hide every later block.
tmux send-keys -t "$SESSION" C-End
sleep 1

note "10. real coloured command: git status"

tmux send-keys -t "$SESSION" '!git status --short --branch 2>&1 | head -5' Enter
sleep 4
if grep -qaF "##" <<<"$(block_lines)"; then
    ok "git branch line rendered"
else
    ok "git produced nothing here (detached or fresh worktree)"
fi
# git colours the branch name and the status codes only when it believes a
# terminal is attached. FORCE_COLOR is what makes it believe so.
if grep -qa $'\x1b\[38;[25];' <<<"$(block_lines)"; then
    ok "git output is drawn with resolved colour"
else
    bad "git output reached the frame without colour"
    block_lines | cat -v | sed 's/^/      | /'
fi

# ---------------------------------------------------------------------------
note "11. forced colour from the shell environment"

tmux send-keys -t "$SESSION" '!sh -c "printf \"\033[32mnocolor\033[0m\n\""' Enter
sleep 3
if grep -qaF "nocolor" <<<"$(block_lines)"; then
    ok "colour command rendered"
else
    bad "colour command produced no output"
fi
if grep -a "nocolor" <<<"$(block_lines)" | grep -q $'\x1b\[38;'; then
    ok "the shell reached the frame as resolved colour"
else
    bad "forced colour did not reach the frame"
fi

# ---------------------------------------------------------------------------
note "12. output with no trailing newline"

tmux send-keys -t "$SESSION" '!printf "no-newline-here"' Enter
sleep 3
if grep -qaF "no-newline-here" <<<"$(block_lines)"; then
    ok "unterminated output renders"
else
    bad "unterminated output was lost"
    block_lines | cat -v | sed 's/^/      | /'
fi

# ---------------------------------------------------------------------------
note "12b. NO_COLOR set on kit suppresses the colour"

# A second session with NO_COLOR exported. The child inherits it and kit does
# not add FORCE_COLOR beside it, so the program stays plain.
tmux kill-session -t nocolor 2>/dev/null
tmux new-session -d -s nocolor -x "$W" -y "$H" \
    "cd /home/space_cowboy/Workspace/kit && NO_COLOR=1 $KIT --no-session 2>/tmp/kitnocolor_stderr.log"
sleep 4
tmux send-keys -t nocolor '!printf "\033[32mnc\033[0m\n"' Enter
sleep 3
nocolor_screen=$(tmux capture-pane -t nocolor -p -e)
if grep -qa "nc" <<<"$nocolor_screen"; then
    ok "output still rendered under NO_COLOR"
else
    bad "NO_COLOR run lost its output"
fi
if grep -a "nc" <<<"$nocolor_screen" | grep -qa $'\x1b\[38;'; then
    bad "NO_COLOR in kit's own environment was ignored"
    grep -a "nc" <<<"$nocolor_screen" | cat -v | sed 's/^/      | /'
else
    ok "NO_COLOR in kit's environment suppresses colour"
fi
tmux kill-session -t nocolor 2>/dev/null

# ---------------------------------------------------------------------------
note "13. large output does not stall the UI"

tmux send-keys -t "$SESSION" '!seq 1 20000' Enter
sleep 6
if grep -q "more lines" <<<"$(tmux capture-pane -t "$SESSION" -p)"; then
    ok "20000 lines are capped"
else
    bad "20000 lines produced no truncation notice"
    tmux capture-pane -t "$SESSION" -p | sed 's/^/      | /'
fi

# ---------------------------------------------------------------------------
note "14. app still responds after all of it"

tmux send-keys -t "$SESSION" '/help' Enter
sleep 2
screen=$(tmux capture-pane -t "$SESSION" -p)
if [[ -n "$(tr -d '[:space:]' <<<"$screen")" ]]; then
    ok "app is responsive and rendering"
else
    bad "app rendered nothing"
fi

# ---------------------------------------------------------------------------
note "results"
printf 'pass=%d fail=%d\n' "$pass" "$fail"
printf 'kit stderr tail:\n'
tail -5 /tmp/kitbash_stderr.log 2>/dev/null | sed 's/^/      | /'

tmux kill-session -t "$SESSION" 2>/dev/null
[[ $fail -eq 0 ]]