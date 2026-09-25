package kit

import (
	"context"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
)

// promptRecordingModel is an echoModel that records the prompt of every
// streaming call.
type promptRecordingModel struct {
	echoModel

	mu      sync.Mutex
	prompts [][]fantasy.Message
}

func (m *promptRecordingModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.prompts = append(m.prompts, append([]fantasy.Message(nil), call.Prompt...))
	m.mu.Unlock()
	return m.echoModel.Stream(ctx, call)
}

func (m *promptRecordingModel) recordedPrompts() [][]fantasy.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prompts
}

// userTextCount returns how many user messages in msgs contain text.
func userTextCount(msgs []fantasy.Message, text string) int {
	n := 0
	for _, msg := range msgs {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok && strings.Contains(tp.Text, text) {
				n++
			}
		}
	}
	return n
}

func steerTexts(msgs []SteerMessage) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Text
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestInjectSteer_keptWhenIdle verifies that a steer message sent while no
// generation runs (turn setup, compaction) is kept, not silently dropped.
func TestInjectSteer_keptWhenIdle(t *testing.T) {
	t.Parallel()
	m := &Kit{}

	m.InjectSteer("one")
	m.InjectSteer("two")

	if got := steerTexts(m.DrainSteer()); !equalStrings(got, []string{"one", "two"}) {
		t.Fatalf("DrainSteer() = %v, want [one two]", got)
	}
	if got := m.DrainSteer(); got != nil {
		t.Fatalf("second DrainSteer() = %v, want nil", got)
	}
}

// TestInjectSteer_channelFullKeepsOrder verifies that messages that do not
// fit in the live channel are kept and returned after the channel contents.
func TestInjectSteer_channelFullKeepsOrder(t *testing.T) {
	t.Parallel()
	m := &Kit{steerCh: make(chan agent.SteerMessage, 1)}

	m.InjectSteer("in-channel")
	m.InjectSteer("overflow")

	if got := steerTexts(m.DrainSteer()); !equalStrings(got, []string{"in-channel", "overflow"}) {
		t.Fatalf("DrainSteer() = %v, want [in-channel overflow]", got)
	}
}

// TestInjectSteer_idleDeliveredAtNextTurn verifies that generate() moves a
// steer message injected while no turn was active into the first step of
// the next turn, and that the message is consumed there.
func TestInjectSteer_idleDeliveredAtNextTurn(t *testing.T) {
	const steerText = "STEER: injected while idle"

	model := &promptRecordingModel{}
	model.provider, model.model = "rec", "m"
	factory := func(context.Context, *ProviderConfig, string) (*ProviderResult, error) {
		return &ProviderResult{Model: model}, nil
	}
	k := newProviderTestKit(t, &Options{
		Model:     "rec/m",
		Providers: map[string]ProviderFactory{"rec": factory},
	})

	if k.IsGenerating() {
		t.Fatal("IsGenerating() = true before the first turn")
	}
	k.InjectSteer(steerText)

	res, err := k.PromptResult(context.Background(), "go")
	if err != nil {
		t.Fatalf("PromptResult: %v", err)
	}

	prompts := model.recordedPrompts()
	if len(prompts) == 0 {
		t.Fatal("model was not called")
	}
	if n := userTextCount(prompts[0], steerText); n != 1 {
		t.Fatalf("first model prompt contains the idle steer message %d time(s), want 1", n)
	}
	if n := userTextCount(res.Messages, steerText); n != 1 {
		t.Fatalf("TurnResult.Messages contains the idle steer message %d time(s), want 1", n)
	}
	if got := k.DrainSteer(); got != nil {
		t.Fatalf("DrainSteer() after the turn = %v, want nil (message must be consumed)", steerTexts(got))
	}
}
