package extensions

import (
	"context"
	"errors"
	"sync"
)

// ProviderOwner keeps interpreter callbacks alive until all successful runtimes
// are closed. Retirement stops new creates but never invalidates old runtimes.
type ProviderOwner struct {
	mu                sync.Mutex
	active            int
	retired           bool
	shutdownScheduled bool
	shutdown          func()
	runtimes          map[*providerAdapter]struct{}
}

func (o *ProviderOwner) acquire() bool {
	if o == nil {
		return true
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.retired {
		return false
	}
	o.active++
	return true
}
func (o *ProviderOwner) release() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.active--
	var shutdown func()
	if o.retired && o.active == 0 {
		shutdown = o.shutdown
		o.shutdown = nil
	}
	o.mu.Unlock()
	if shutdown != nil {
		shutdown()
	}
}

// Retire defers interpreter shutdown until the last runtime has closed.
// It invokes no extension callbacks while holding the owner lock.
func (o *ProviderOwner) Retire(shutdown func()) {
	if o == nil {
		shutdown()
		return
	}
	o.mu.Lock()
	if o.shutdownScheduled {
		o.mu.Unlock()
		return
	}
	o.retired = true
	o.shutdownScheduled = true
	if o.active == 0 {
		o.mu.Unlock()
		shutdown()
		return
	}
	o.shutdown = shutdown
	o.mu.Unlock()
}

func (o *ProviderOwner) track(runtime *providerAdapter) {
	if o == nil {
		return
	}
	o.mu.Lock()
	if o.runtimes == nil {
		o.runtimes = map[*providerAdapter]struct{}{}
	}
	o.runtimes[runtime] = struct{}{}
	o.mu.Unlock()
}
func (o *ProviderOwner) untrack(runtime *providerAdapter) {
	if o == nil {
		return
	}
	o.mu.Lock()
	delete(o.runtimes, runtime)
	o.mu.Unlock()
}

// CloseRuntimes cancels all owned runtimes before waiting for any of them.
// It must not be called synchronously from a provider callback.
func (o *ProviderOwner) CloseRuntimes(ctx context.Context) error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	for runtime := range o.runtimes {
		if runtime.callbackGID.Load() == goroutineID() {
			o.mu.Unlock()
			return errors.New("cannot shut down provider generation from its own callback")
		}
	}
	o.retired = true
	runtimes := make([]*providerAdapter, 0, len(o.runtimes))
	for runtime := range o.runtimes {
		runtimes = append(runtimes, runtime)
	}
	o.mu.Unlock()
	for _, runtime := range runtimes {
		runtime.cancel()
	}
	var errs []error
	for _, runtime := range runtimes {
		if err := runtime.closeContext(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
