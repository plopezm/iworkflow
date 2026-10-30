package application

import (
	"context"
	"fmt"
)

// WriteGuard serializes the load-mutate-save cycle of profiles.json across the use cases that
// share it. It is a one-slot semaphore instead of a mutex so a caller waiting for the lock gives
// up when its context ends.
type WriteGuard struct {
	slot chan struct{}
}

// NewWriteGuard builds an unlocked guard. Every use case that mutates profiles.json must receive
// the same instance.
func NewWriteGuard() *WriteGuard {
	return &WriteGuard{slot: make(chan struct{}, 1)}
}

func (g *WriteGuard) lock(ctx context.Context) error {
	select {
	case g.slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for profiles.json lock: %w", ctx.Err())
	}
}

func (g *WriteGuard) unlock() {
	<-g.slot
}
