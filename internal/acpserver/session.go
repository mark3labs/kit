package acpserver

import (
	"context"
	"fmt"
	"sync"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// acpSession maps an ACP session to a Kit instance with its own tree session.
type acpSession struct {
	kit       *kit.Kit
	cancelFn  context.CancelFunc // cancels the current prompt
	cancelMu  sync.Mutex
	cwd       string
	sessionID string // Kit-generated session ID (from JSONL header)
}

// sessionRegistry is a thread-safe registry of ACP session ID → Kit sessions.
type sessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*acpSession // ACP session ID → session
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{
		sessions: make(map[string]*acpSession),
	}
}

// create creates a new Kit instance with a persisted tree session for the
// given working directory. The Kit-generated session ID is used as the ACP
// session ID so the mapping is 1:1.
func (r *sessionRegistry) create(ctx context.Context, cwd string) (*acpSession, error) {
	kitInstance, err := kit.New(ctx, &kit.Options{
		SessionDir: cwd,
		Quiet:      true,
		Streaming:  true,
	})
	if err != nil {
		return nil, fmt.Errorf("create kit instance: %w", err)
	}

	sessionID := kitInstance.GetSessionID()
	if sessionID == "" {
		_ = kitInstance.Close()
		return nil, fmt.Errorf("kit instance has no session ID")
	}

	sess := &acpSession{
		kit:       kitInstance,
		cwd:       cwd,
		sessionID: sessionID,
	}

	r.mu.Lock()
	r.sessions[sessionID] = sess
	r.mu.Unlock()

	return sess, nil
}

// get retrieves a session by ACP session ID.
func (r *sessionRegistry) get(sessionID string) (*acpSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[sessionID]
	return s, ok
}

// closeAll closes all sessions.
func (r *sessionRegistry) closeAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, sess := range r.sessions {
		if sess.kit != nil {
			_ = sess.kit.Close()
		}
		delete(r.sessions, id)
	}
}

// cancelPrompt cancels the current prompt for a session, if any.
func (s *acpSession) cancelPrompt() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
}

// setCancel stores a cancel function for the current prompt.
func (s *acpSession) setCancel(cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelFn = cancel
}

// clearCancel clears the stored cancel function (called when prompt completes).
func (s *acpSession) clearCancel() {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancelFn = nil
}
