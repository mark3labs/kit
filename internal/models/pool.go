package models

import (
	"context"
	"sync"
	"time"

	"charm.land/fantasy"
)

// ProviderPool manages reusable LLM provider instances to reduce overhead
// when spawning multiple subagents or making repeated completion calls.
type ProviderPool struct {
	mu        sync.RWMutex
	providers map[string]*pooledProvider
	ttl       time.Duration
	closed    bool
	closeCh   chan struct{}
}

type pooledProvider struct {
	model        fantasy.LanguageModel
	closer       func() error
	providerOpts fantasy.ProviderOptions
	created      time.Time
	lastUsed     time.Time
	refs         int32
}

// DefaultPoolTTL is the default time-to-live for idle pooled providers.
const DefaultPoolTTL = 5 * time.Minute

// globalPool is the singleton provider pool instance.
var globalPool *ProviderPool
var poolOnce sync.Once

// GetGlobalPool returns the singleton provider pool instance.
func GetGlobalPool() *ProviderPool {
	poolOnce.Do(func() {
		globalPool = NewProviderPool(DefaultPoolTTL)
	})
	return globalPool
}

// NewProviderPool creates a provider pool with the given TTL for idle providers.
func NewProviderPool(ttl time.Duration) *ProviderPool {
	p := &ProviderPool{
		providers: make(map[string]*pooledProvider),
		ttl:       ttl,
		closeCh:   make(chan struct{}),
	}
	go p.cleanupLoop()
	return p
}

// Get returns a provider for the model string, creating one if needed.
// The returned release function must be called when the provider is no longer
// needed. The provider may be reused by subsequent Get calls.
func (p *ProviderPool) Get(ctx context.Context, modelString string) (fantasy.LanguageModel, fantasy.ProviderOptions, func(), error) {
	p.mu.Lock()

	// Check if we have an existing provider.
	if pp, ok := p.providers[modelString]; ok {
		pp.refs++
		pp.lastUsed = time.Now()
		p.mu.Unlock()
		return pp.model, pp.providerOpts, func() { p.release(modelString) }, nil
	}

	p.mu.Unlock()

	// Create a new provider outside the lock.
	config := &ProviderConfig{ModelString: modelString}
	result, err := CreateProvider(ctx, config)
	if err != nil {
		return nil, nil, nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check: another goroutine may have created one while we were unlocked.
	if pp, ok := p.providers[modelString]; ok {
		// Close the one we just created and use the existing one.
		if result.Closer != nil {
			_ = result.Closer.Close()
		}
		pp.refs++
		pp.lastUsed = time.Now()
		return pp.model, pp.providerOpts, func() { p.release(modelString) }, nil
	}

	var closerFn func() error
	if result.Closer != nil {
		closerFn = result.Closer.Close
	}

	pp := &pooledProvider{
		model:        result.Model,
		closer:       closerFn,
		providerOpts: result.ProviderOptions,
		created:      time.Now(),
		lastUsed:     time.Now(),
		refs:         1,
	}
	p.providers[modelString] = pp

	return pp.model, pp.providerOpts, func() { p.release(modelString) }, nil
}

func (p *ProviderPool) release(modelString string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if pp, ok := p.providers[modelString]; ok {
		pp.refs--
		pp.lastUsed = time.Now()
	}
}

func (p *ProviderPool) cleanupLoop() {
	ticker := time.NewTicker(p.ttl / 2)
	defer ticker.Stop()

	for {
		select {
		case <-p.closeCh:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *ProviderPool) cleanup() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, pp := range p.providers {
		// Only clean up providers with no active references and past TTL.
		if pp.refs <= 0 && now.Sub(pp.lastUsed) > p.ttl {
			if pp.closer != nil {
				_ = pp.closer()
			}
			delete(p.providers, key)
		}
	}
}

// Close shuts down the pool and releases all providers.
func (p *ProviderPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	close(p.closeCh)

	for key, pp := range p.providers {
		if pp.closer != nil {
			_ = pp.closer()
		}
		delete(p.providers, key)
	}
	p.mu.Unlock()
}
