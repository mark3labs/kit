package daemon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// manageAttachedSession runs after the session input pump has stopped. It uses
// the connection's existing reader, never a second reader on the terminal.
func manageAttachedSession(ctx context.Context, c *clientConn, opts AttachOptions, action uint64) (bool, error) {
	entries, err := c.listSessions(ctx)
	if err != nil {
		return false, err
	}
	var name string
	for _, e := range entries {
		if e.ID == c.current() {
			name = e.Name
		}
	}
	host := opts.Host
	if host == "" {
		host = "local"
	}
	label := fmt.Sprintf("session %d %q on %s", c.current(), name, host)
	prompt := "Name for " + label + " (Esc cancels): "
	initial := name
	if action == killSentinel {
		prompt = "Kill " + label + "? Saved history will remain. [y/N]: "
		initial = ""
	}
	answer, cancelled, err := c.sessionPrompt(ctx, prompt, initial)
	if err != nil || cancelled {
		return false, err
	}
	typ := FrameSessionRename
	if action == killSentinel {
		if !strings.EqualFold(strings.TrimSpace(answer), "y") && !strings.EqualFold(strings.TrimSpace(answer), "yes") {
			return false, nil
		}
		// Release this wire before retiring the session, so its close notification
		// does not prevent receipt of the control result.
		if err := c.write(FrameSessionDetach, nil); err != nil {
			return false, err
		}
		typ = FrameSessionKill
	} else if strings.TrimSpace(answer) == "" {
		return false, nil
	}
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, c.current())
	if typ == FrameSessionRename {
		payload = append(payload, []byte(strings.TrimSpace(answer))...)
	}
	if err := c.write(typ, payload); err != nil {
		return false, err
	}
	reply, err := c.awaitCtrl(ctx, FrameSessionControlResult, 10*time.Second)
	if err != nil {
		return false, err
	}
	var result struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(reply.Payload, &result); err != nil {
		return false, fmt.Errorf("decode session result: %w", err)
	}
	if result.Error != "" {
		return false, fmt.Errorf("session control: %s", result.Error)
	}
	return typ == FrameSessionKill, nil
}

func (c *clientConn) sessionPrompt(ctx context.Context, prompt, initial string) (string, bool, error) {
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", false, fmt.Errorf("session prompt: %w", err)
	}
	defer func() { _ = term.Restore(fd, state) }()
	text := []rune(initial)
	render := func() { fmt.Fprintf(os.Stdout, "\r\x1b[2K%s%s", prompt, string(text)) }
	fmt.Fprint(os.Stdout, "\x1b[2J\x1b[H")
	render()
	for {
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case <-c.closedCh:
			return "", false, errStreamClosed
		case chunk, ok := <-c.stdinCh:
			if !ok {
				return "", true, nil
			}
			for _, r := range string(chunk) {
				switch r {
				case 27, 3:
					return "", true, nil
				case '\r', '\n':
					return string(text), false, nil
				case 127, 8:
					if len(text) > 0 {
						text = text[:len(text)-1]
					}
				default:
					if r >= 32 {
						text = append(text, r)
					}
				}
			}
			render()
		}
	}
}
