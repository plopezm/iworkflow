package pty

import (
	"bytes"
	"io"
	"sync"
)

const (
	// scrollbackLimit bounds the output kept for reattaching. It matches the Rust version.
	scrollbackLimit int = 64 * 1024
	// subscriberCapacity is the number of chunks a subscriber may hold before it starts losing the
	// oldest ones. It matches the Rust broadcast capacity.
	subscriberCapacity int = 256
	// readBufferSize is the chunk size read from the PTY.
	readBufferSize int = 8192
)

// subscriber is the live output channel of one attachment.
type subscriber struct {
	ch chan []byte
}

// push delivers chunk without blocking the reader. A subscriber that does not keep up loses its
// oldest chunk, so it sees the newest output instead of stalling the shell (the broadcast "Lagged"
// behaviour of the Rust version).
func (s *subscriber) push(chunk []byte) {
	select {
	case s.ch <- chunk:
		return
	default:
	}
	select {
	case <-s.ch:
	default:
	}
	select {
	case s.ch <- chunk:
	default:
	}
}

// sharedOutput is the output of one session, shared by its reader and its attachments. The mutex
// makes the scrollback snapshot and the subscription atomic with respect to append: every chunk
// lands either in the snapshot or in the channel, never in both and never in neither.
type sharedOutput struct {
	mu          sync.Mutex
	scrollback  []byte
	subscribers map[*subscriber]struct{}
	closed      bool
}

func newSharedOutput() *sharedOutput {
	return &sharedOutput{subscribers: make(map[*subscriber]struct{})}
}

// append stores chunk in the bounded scrollback and fans it out to the subscribers.
func (o *sharedOutput) append(chunk []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.scrollback = append(o.scrollback, chunk...)
	if excess := len(o.scrollback) - scrollbackLimit; excess > 0 {
		// Shift in place so the backing array does not grow with the total output.
		kept := copy(o.scrollback, o.scrollback[excess:])
		o.scrollback = o.scrollback[:kept]
	}
	// The chunk is read-only for every subscriber, so one copy serves them all.
	shared := bytes.Clone(chunk)
	for sub := range o.subscribers {
		sub.push(shared)
	}
}

// finish marks the shell as ended. Subscribers drain what they already hold and then see their
// channel closed.
func (o *sharedOutput) finish() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return
	}
	o.closed = true
	for sub := range o.subscribers {
		close(sub.ch)
	}
	clear(o.subscribers)
}

// isRunning reports whether the shell still produces output.
func (o *sharedOutput) isRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.closed
}

// subscribe returns a copy of the scrollback and a live channel. The returned detach function removes
// the subscriber; it is idempotent. An ended session yields a closed channel.
func (o *sharedOutput) subscribe() ([]byte, <-chan []byte, func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	scrollback := bytes.Clone(o.scrollback)
	ch := make(chan []byte, subscriberCapacity)
	if o.closed {
		close(ch)
		return scrollback, ch, func() {}
	}
	sub := &subscriber{ch: ch}
	o.subscribers[sub] = struct{}{}
	detach := func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		delete(o.subscribers, sub)
	}
	return scrollback, ch, detach
}

// pump copies the PTY output into the shared output until the PTY reports end of file or an error,
// which happens once the shell and every process that holds the terminal have exited or been killed.
func pump(master io.Reader, output *sharedOutput) {
	defer output.finish()
	buffer := make([]byte, readBufferSize)
	for {
		read, err := master.Read(buffer)
		if read == 0 {
			return
		}
		output.append(buffer[:read])
		if err != nil {
			return
		}
	}
}
