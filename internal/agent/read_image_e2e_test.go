package agent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"

	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/media"
	"github.com/mark3labs/kit/internal/message"
)

// This file holds the end-to-end guard for reading an image with the read
// tool. It drives a real agent loop — real core tools, real fantasy agent,
// real provider serialisation — over a fake OpenAI-compatible endpoint, and
// asserts on the bytes the provider would have put on the wire.
//
// Without this test, a change that drops the media branch anywhere along the
// chain (read tool, fantasy conversion, provider mapping) would still compile
// and still pass every unit test, while quietly sending the model nothing.

// writeTestPNG writes a PNG of the given size and returns its path.
func writeTestPNG(t *testing.T, dir, name string, w, h int, noisy bool) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(3))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 200, G: 30, B: 90, A: 255}
			if noisy {
				c = color.RGBA{
					R: uint8(rng.Intn(256)),
					G: uint8(rng.Intn(256)),
					B: uint8(rng.Intn(256)),
					A: 255,
				}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// toolCallSSE is the first turn: the model asks to read the file at path.
func toolCallSSE(path string) string {
	args, _ := json.Marshal(map[string]string{"path": path})
	call, _ := json.Marshal(string(args))
	return strings.Join([]string{
		`{"id":"x","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_img","type":"function","function":{"name":"read","arguments":""}}]},"finish_reason":null}]}`,
		fmt.Sprintf(`{"id":"x","created":1,"model":"m","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":%s}}]},"finish_reason":null}]}`, call),
		`{"id":"x","created":1,"model":"m","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}, "\n")
}

// answerSSE is the second turn: the model answers in plain text.
const answerSSE = `{"id":"x","created":1,"model":"m","choices":[{"index":0,"delta":{"role":"assistant","content":"I can see the image."},"finish_reason":null}]}
{"id":"x","created":1,"model":"m","choices":[{"index":0,"delta":{"content":""},"finish_reason":"stop"}]}`

// recordingServer answers each request with the next scripted SSE payload and
// keeps every request body for inspection.
type recordingServer struct {
	*httptest.Server
	mu       sync.Mutex
	bodies   [][]byte
	payloads []string
	turn     int
}

func newRecordingServer(t *testing.T, payloads ...string) *recordingServer {
	t.Helper()
	rs := &recordingServer{payloads: payloads}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		rs.mu.Lock()
		rs.bodies = append(rs.bodies, body)
		payload := answerSSE
		if rs.turn < len(rs.payloads) {
			payload = rs.payloads[rs.turn]
		}
		rs.turn++
		rs.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		for event := range strings.SplitSeq(payload, "\n") {
			if event = strings.TrimSpace(event); event == "" {
				continue
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", event)
			_ = rc.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		_ = rc.Flush()
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) body(i int) []byte {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if i >= len(rs.bodies) {
		return nil
	}
	return rs.bodies[i]
}

func (rs *recordingServer) requestCount() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return len(rs.bodies)
}

// newImageAgent builds an Agent backed by srv, with the real read tool wired
// in under the given image limits.
func newImageAgent(ctx context.Context, t *testing.T, srv *httptest.Server, limits media.Limits) *Agent {
	t.Helper()
	provider, err := openaicompat.New(
		openaicompat.WithBaseURL(srv.URL),
		openaicompat.WithAPIKey("test"),
		openaicompat.WithName("test-compat"),
	)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	model, err := provider.LanguageModel(ctx, "test-model")
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	readTool := core.NewReadTool(core.WithImageLimits(limits))
	return &Agent{
		fantasyAgent: fantasy.NewAgent(model, fantasy.WithTools(readTool)),
		// composeAllTools re-reads this on every step through
		// PrepareStep, so the agent must see the tool here too.
		coreTools:        []fantasy.AgentTool{readTool},
		model:            model,
		maxSteps:         4,
		streamingEnabled: true,
		providerType:     "test-compat",
	}
}

// imagePartsIn walks an OpenAI chat-completions request body and returns every
// base64 data URL it carries.
func imagePartsIn(t *testing.T, body []byte) []string {
	t.Helper()
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode request: %v\nbody: %s", err, body)
	}

	var urls []string
	for _, m := range req.Messages {
		var parts []struct {
			Type     string `json:"type"`
			ImageURL struct {
				URL string `json:"url"`
			} `json:"image_url"`
		}
		if err := json.Unmarshal(m.Content, &parts); err != nil {
			continue // a plain string content block
		}
		for _, p := range parts {
			if p.Type == "image_url" && strings.HasPrefix(p.ImageURL.URL, "data:") {
				urls = append(urls, p.ImageURL.URL)
			}
		}
	}
	return urls
}

// TestE2EReadImageReachesProvider is the core end-to-end assertion: the model
// calls read on a PNG, and the next request to the provider carries that PNG
// as a real base64 image part.
func TestE2EReadImageReachesProvider(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "diagram.png", 120, 90, false)

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	var resultText string
	result, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("what is in diagram.png?")},
		GenerateCallbacks{
			OnToolResult: func(_, _, _, text string, _ string, _ bool) {
				resultText = text
			},
		})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if srv.requestCount() < 2 {
		t.Fatalf("provider requests = %d, want at least 2; the tool loop did not run",
			srv.requestCount())
	}

	// 1. The second request must carry the image.
	urls := imagePartsIn(t, srv.body(1))
	if len(urls) != 1 {
		t.Fatalf("image parts in the follow-up request = %d, want 1; the image never reached the provider", len(urls))
	}

	// 2. The data URL must decode back to the PNG that was on disk.
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(urls[0], prefix) {
		t.Fatalf("data URL prefix = %.40q, want %q", urls[0], prefix)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(urls[0], prefix))
	if err != nil {
		t.Fatalf("decode base64 payload: %v", err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(decoded, onDisk) {
		t.Error("the payload sent is not the file on disk; a small image must pass through untouched")
	}

	// 3. The visible tool result must be the summary, never the base64.
	if !strings.HasPrefix(resultText, "Read image ") {
		t.Errorf("tool result text = %q, want the summary", resultText)
	}
	if strings.Contains(resultText, base64.StdEncoding.EncodeToString(onDisk[:32])) {
		t.Error("base64 payload leaked into the visible tool result")
	}

	// 4. The turn must finish normally.
	if got := result.FinalResponse.Content.Text(); got != "I can see the image." {
		t.Errorf("final response = %q, want %q", got, "I can see the image.")
	}
}

// TestE2EReadImagePersistsAndReplays checks the session path: the media part
// must survive conversion to a stored message and back, so that resuming a
// session re-sends the same image instead of an empty tool result.
func TestE2EReadImagePersistsAndReplays(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "shot.png", 64, 64, false)

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	result, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("look at shot.png")},
		GenerateCallbacks{})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Round-trip every conversation message through the stored form, the
	// way a session manager does.
	var replayed []fantasy.Message
	for _, m := range result.ConversationMessages {
		stored := message.FromLLMMessage(m)
		replayed = append(replayed, stored.ToLLMMessages()...)
	}

	var mediaParts int
	for _, m := range replayed {
		for _, part := range m.Content {
			trp, ok := part.(fantasy.ToolResultPart)
			if !ok {
				continue
			}
			out, ok := trp.Output.(fantasy.ToolResultOutputContentMedia)
			if !ok {
				continue
			}
			mediaParts++
			if out.MediaType != media.MediaTypePNG {
				t.Errorf("MediaType = %q, want %q", out.MediaType, media.MediaTypePNG)
			}
			if _, err := base64.StdEncoding.DecodeString(out.Data); err != nil {
				t.Errorf("stored payload is not valid base64: %v", err)
			}
		}
	}
	if mediaParts != 1 {
		t.Fatalf("media tool results after a session round trip = %d, want 1; the image was lost on persist", mediaParts)
	}
}

// TestE2EReadOversizedImageIsResizedBeforeSending checks that the byte budget
// is applied on the wire, not only in the tool: a large noisy PNG must arrive
// as a smaller JPEG.
func TestE2EReadOversizedImageIsResizedBeforeSending(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "huge.png", 900, 900, true)

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{MaxEdge: 300, MaxEncodedBytes: 40_000})

	if _, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("look at huge.png")},
		GenerateCallbacks{}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	urls := imagePartsIn(t, srv.body(1))
	if len(urls) != 1 {
		t.Fatalf("image parts = %d, want 1", len(urls))
	}

	comma := strings.Index(urls[0], ",")
	if comma < 0 {
		t.Fatalf("malformed data URL: %.60q", urls[0])
	}
	header, payload := urls[0][:comma], urls[0][comma+1:]
	if !strings.Contains(header, media.MediaTypeJPEG) {
		t.Errorf("media type header = %q, want %q; PNG cannot meet this budget",
			header, media.MediaTypeJPEG)
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(decoded) > 40_000 {
		t.Errorf("payload on the wire = %d bytes, want at most 40000", len(decoded))
	}
	if len(decoded) >= len(onDisk) {
		t.Errorf("payload = %d bytes, source = %d bytes; the image was not shrunk",
			len(decoded), len(onDisk))
	}

	img, _, err := image.Decode(bytes.NewReader(decoded))
	if err != nil {
		t.Fatalf("payload on the wire does not decode: %v", err)
	}
	if img.Bounds().Dx() > 300 || img.Bounds().Dy() > 300 {
		t.Errorf("size on the wire = %dx%d, want each edge at most 300",
			img.Bounds().Dx(), img.Bounds().Dy())
	}
}

// TestE2EReadTextFileSendsNoImage is the negative control: reading a source
// file must still produce a plain text tool result.
func TestE2EReadTextFileSendsNoImage(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	if _, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("read main.go")},
		GenerateCallbacks{}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if urls := imagePartsIn(t, srv.body(1)); len(urls) != 0 {
		t.Errorf("image parts = %d, want 0 for a text file", len(urls))
	}
	if !bytes.Contains(srv.body(1), []byte("package main")) {
		t.Error("the file text never reached the provider")
	}
}

// TestE2EReadCorruptImageSendsError checks that a broken image becomes a clean
// tool error rather than binary noise in the context.
func TestE2EReadCorruptImageSendsError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := writeTestPNG(t, dir, "broken.png", 80, 80, false)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, data[:len(data)/3], 0o644); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})

	var sawError bool
	if _, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("read broken.png")},
		GenerateCallbacks{
			OnToolResult: func(_, _, _, _ string, _ string, isError bool) {
				sawError = isError
			},
		}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if !sawError {
		t.Error("the tool result was not marked as an error")
	}
	if urls := imagePartsIn(t, srv.body(1)); len(urls) != 0 {
		t.Errorf("image parts = %d, want 0 for a corrupt image", len(urls))
	}
	if !bytes.Contains(srv.body(1), []byte("cannot read image")) {
		t.Error("the error message never reached the provider")
	}
}
