package models

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// StreamingHTTPConfig sets HTTP limits for Codex and Copilot. There is no
// total request timeout; use the request context to set a total deadline.
// Zero values select defaults. Negative values disable the corresponding limit.
// BodyIdleTimeout measures time without bytes read, starting at response headers.
// It also applies when the caller does not read the body. Close each response body.
type StreamingHTTPConfig struct {
	// DialTimeout defaults to 30 seconds.
	DialTimeout time.Duration
	// TLSHandshakeTimeout defaults to 10 seconds.
	TLSHandshakeTimeout time.Duration
	// ResponseHeaderTimeout defaults to 120 seconds.
	ResponseHeaderTimeout time.Duration
	// BodyIdleTimeout defaults to 120 seconds. Any bytes, including SSE
	// comments and partial events, reset this limit.
	BodyIdleTimeout time.Duration
}

func streamingLimit(value, fallback time.Duration) time.Duration {
	if value == 0 {
		return fallback
	}
	if value < 0 {
		return 0
	}
	return value
}

func newStreamingTransport(skipVerify bool, config StreamingHTTPConfig) http.RoundTripper {
	base := http.DefaultTransport
	if transport, ok := base.(*http.Transport); ok {
		transport = transport.Clone()
		transport.DialContext = (&net.Dialer{
			Timeout:   streamingLimit(config.DialTimeout, 30*time.Second),
			KeepAlive: 30 * time.Second,
		}).DialContext
		transport.TLSHandshakeTimeout = streamingLimit(config.TLSHandshakeTimeout, 10*time.Second)
		transport.ResponseHeaderTimeout = streamingLimit(config.ResponseHeaderTimeout, 120*time.Second)
		if skipVerify {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
		base = transport
	}
	return &streamingTransport{base: base, idleTimeout: streamingLimit(config.BodyIdleTimeout, 120*time.Second)}
}

type streamingTransport struct {
	base        http.RoundTripper
	idleTimeout time.Duration
}

func (t *streamingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	resp, err := t.base.RoundTrip(req.Clone(ctx))
	if err != nil {
		cancel()
		// Preserve net.Error so http.Client can report transport timeouts.
		return nil, err
	}
	if resp.Body == nil || resp.Body == http.NoBody {
		cancel()
		return resp, nil
	}
	body := &idleResponseBody{body: resp.Body, cancel: cancel, timeout: t.idleTimeout}
	body.mu.Lock()
	body.lastRead = time.Now()
	if body.timeout > 0 {
		body.timer = time.AfterFunc(body.timeout, body.expire)
	}
	body.stopContext = context.AfterFunc(ctx, func() {
		// Cancellation is the terminal read error; Close returns any cleanup error.
		_ = body.finish(ctx.Err())
	})
	body.mu.Unlock()
	resp.Body = body
	return resp, nil
}

// streamingIdleError supports errors.Is(err, context.DeadlineExceeded) and net.Error.
type streamingIdleError struct{}

func (streamingIdleError) Error() string   { return "streaming HTTP response body idle timeout" }
func (streamingIdleError) Unwrap() error   { return context.DeadlineExceeded }
func (streamingIdleError) Timeout() bool   { return true }
func (streamingIdleError) Temporary() bool { return true }

type idleResponseBody struct {
	body        io.ReadCloser
	cancel      context.CancelFunc
	timeout     time.Duration
	mu          sync.Mutex
	lastRead    time.Time
	timer       *time.Timer
	stopContext func() bool
	done        bool
	err         error
	closeOnce   sync.Once
	closeErr    error
}

func (b *idleResponseBody) expire() {
	b.mu.Lock()
	if b.done {
		b.mu.Unlock()
		return
	}
	if remaining := b.timeout - time.Since(b.lastRead); remaining > 0 {
		b.timer.Reset(remaining)
		b.mu.Unlock()
		return
	}
	// Set the terminal state while locked so a late read cannot reset it.
	b.stopLocked(streamingIdleError{})
	b.mu.Unlock()
	// The idle timeout remains the terminal error. Close exposes cleanup errors.
	_ = b.closeBody()
}

func (b *idleResponseBody) stopLocked(err error) {
	b.done = true
	b.err = err
	if b.timer != nil {
		b.timer.Stop()
	}
	if b.stopContext != nil {
		b.stopContext()
	}
}

func (b *idleResponseBody) closeBody() error {
	b.cancel()
	b.closeOnce.Do(func() { b.closeErr = b.body.Close() })
	return b.closeErr
}

func (b *idleResponseBody) finish(err error) error {
	b.mu.Lock()
	if !b.done {
		b.stopLocked(err)
	}
	b.mu.Unlock()
	return b.closeBody()
}

func (b *idleResponseBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	b.mu.Lock()
	if !b.done && n > 0 {
		b.lastRead = time.Now()
		if b.timer != nil {
			b.timer.Reset(b.timeout)
		}
	}
	if b.err != nil {
		err = b.err
	}
	if err != nil && !b.done {
		b.stopLocked(err)
	}
	b.mu.Unlock()
	if err != nil {
		// Preserve the read error; Close exposes any cleanup error separately.
		_ = b.closeBody()
	}
	return n, err
}

func (b *idleResponseBody) Close() error { return b.finish(io.ErrClosedPipe) }
