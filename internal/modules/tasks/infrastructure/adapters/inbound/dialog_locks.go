package inbound

import "sync"

// dialogKey identifies a dialog of a task: the answers to different dialogs never wait for each other.
type dialogKey struct {
	task   string
	dialog string
}

// dialogLock is the lock of one dialog. refs counts the answers that hold it or wait for it.
type dialogLock struct {
	mu   sync.Mutex
	refs int
}

// dialogLocks serializes the answers to each dialog. An entry exists only while an answer holds or waits for it, so the
// map does not grow with the dialogs answered over the life of the server. The zero value is ready to use.
type dialogLocks struct {
	mu    sync.Mutex
	locks map[dialogKey]*dialogLock
}

// lock waits for the dialog to be free and returns the function that frees it.
func (d *dialogLocks) lock(key dialogKey) func() {
	d.mu.Lock()
	if d.locks == nil {
		d.locks = map[dialogKey]*dialogLock{}
	}
	entry, found := d.locks[key]
	if !found {
		entry = &dialogLock{}
		d.locks[key] = entry
	}
	entry.refs++
	d.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		d.mu.Lock()
		defer d.mu.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(d.locks, key)
		}
	}
}
