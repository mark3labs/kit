package app

import tea "charm.land/bubbletea/v2"

// Capture the target at enqueue time; replacing the program must not redirect
// notifications accepted for an older program.
type extensionMessage struct {
	program *tea.Program
	msg     tea.Msg
}

// enqueueExtensionMessage never waits for the event loop. It returns false only
// for headless operation. Closed apps discard notifications without headless
// output. Acceptance is not an acknowledgement of UI processing.
//
// The expandable FIFO preserves sequential calls, including quit, without
// dropping prints. Producers that outpace the UI can therefore consume memory.
func (a *App) enqueueExtensionMessage(msg tea.Msg) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.rootCtx.Err() != nil {
		return true
	}
	if a.program == nil {
		return false
	}
	if a.extensionWake == nil {
		a.extensionWake = make(chan struct{}, 1)
		a.wg.Add(1)
		go a.dispatchExtensionMessages()
	}
	a.extensionMessages = append(a.extensionMessages, extensionMessage{a.program, msg})
	select {
	case a.extensionWake <- struct{}{}:
	default:
	}
	return true
}

func (a *App) dispatchExtensionMessages() {
	defer a.wg.Done()
	defer func() {
		a.mu.Lock()
		a.extensionMessages = nil
		a.mu.Unlock()
	}()
	for {
		a.mu.Lock()
		if a.closed || a.rootCtx.Err() != nil {
			a.mu.Unlock()
			return
		}
		if len(a.extensionMessages) == 0 {
			a.extensionMessages = nil
			a.mu.Unlock()
			select {
			case <-a.rootCtx.Done():
				return
			case <-a.extensionWake:
			}
			continue
		}
		entry := a.extensionMessages[0]
		a.extensionMessages[0] = extensionMessage{}
		a.extensionMessages = a.extensionMessages[1:]
		a.mu.Unlock()
		// The registered program uses the app context, so cancellation also
		// releases a send when the program has not started receiving yet.
		entry.program.Send(entry.msg)
	}
}
