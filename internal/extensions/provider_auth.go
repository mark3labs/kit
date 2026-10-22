package extensions

import (
	"context"
	"sync"
	"time"
)

// ProviderAuthCache holds only safe status, never credentials. One cache belongs
// to one definition generation. Pending checks are deduplicated and bounded.
type ProviderAuthCache struct {
	mu      sync.Mutex
	status  ProviderAuthResult
	expires time.Time
	pending bool
}

// Status starts an asynchronous safe status check and returns unknown while it
// runs. The callback is never invoked on the UI event-loop goroutine.
func (c *ProviderAuthCache) Status(def ProviderDef, configured bool) ProviderAuthResult {
	if def.Auth.Mode == "" || def.Auth.Mode == ProviderAuthNone {
		return ProviderAuthResult{Status: ProviderAuthConfigured}
	}
	if def.Auth.Mode == ProviderAuthAPIKey {
		if configured {
			return ProviderAuthResult{Status: ProviderAuthConfigured}
		}
		return ProviderAuthResult{Status: ProviderAuthUnconfigured, Instruction: def.Auth.SetupInstruction}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.status
	}
	if !c.pending && def.Auth.Status != nil {
		c.pending = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result := ProviderAuthResult{Status: ProviderAuthUnknown}
			// Trusted code must cooperate with Done. We do not leak an extra waiter
			// goroutine per timed-out check or start another while it remains active.
			func() {
				defer func() {
					if recover() != nil {
						result = ProviderAuthResult{Status: ProviderAuthUnknown}
					}
				}()
				result = def.Auth.Status(ProviderAuthRequest{Done: ctx.Done()})
			}()
			if ctx.Err() != nil || result.Error != nil {
				result = ProviderAuthResult{Status: ProviderAuthUnknown}
			}
			switch result.Status {
			case ProviderAuthConfigured, ProviderAuthUnconfigured, ProviderAuthUnknown:
			default:
				result.Status = ProviderAuthUnknown
			}
			c.mu.Lock()
			c.status = result
			c.expires = time.Now().Add(30 * time.Second)
			c.pending = false
			c.mu.Unlock()
		}()
	}
	return ProviderAuthResult{Status: ProviderAuthUnknown, Instruction: def.Auth.SetupInstruction}
}

// Refresh invalidates status without starting overlapping callbacks.
func (c *ProviderAuthCache) Refresh() { c.mu.Lock(); c.expires = time.Time{}; c.mu.Unlock() }

// AuthStatus queries generation-scoped safe availability without generation.
func (def ProviderDef) AuthStatus(configured bool) ProviderAuthResult {
	if def.authCache == nil {
		return ProviderAuthResult{Status: ProviderAuthUnknown}
	}
	return def.authCache.Status(def, configured)
}
