package extensions

import "sync"

// ProviderOwner keeps interpreter callbacks alive until all successful runtimes
// are closed. Retirement stops new creates but never invalidates old runtimes.
type ProviderOwner struct {
	mu       sync.Mutex
	active   int
	retired  bool
	shutdown func()
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
	if o.retired {
		o.mu.Unlock()
		return
	}
	o.retired = true
	if o.active == 0 {
		o.mu.Unlock()
		shutdown()
		return
	}
	o.shutdown = shutdown
	o.mu.Unlock()
}
