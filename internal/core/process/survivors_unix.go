//go:build unix

package process

import (
	"context"
	"errors"
	"sync"
	"syscall"
)

// Survivors remembers the descendants of one process, wherever they go. A server that detaches (setsid)
// leaves the process group of its parent, so a group kill cannot reach it, and once the parent exits it is
// re-parented, so the tree can no longer be walked. Tracking while the parent lives is the only way to
// find it afterwards.
type Survivors struct {
	mu    sync.Mutex
	known map[int]string
}

// NewSurvivors returns an empty set.
func NewSurvivors() *Survivors {
	return &Survivors{known: make(map[int]string)}
}

// Track adds the current descendants of root and forgets the tracked processes that exited, so the set
// follows the live processes instead of growing with the churn of the agent. A failed process listing
// keeps what is already known.
func (s *Survivors) Track(ctx context.Context, root int) {
	pids, err := Descendants(ctx, root)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid := range s.known {
		// Signal 0 only checks existence, with no fork. A recycled pid stays until Kill compares its start stamp.
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			delete(s.known, pid)
		}
	}
	for _, pid := range pids {
		if _, seen := s.known[pid]; seen {
			continue
		}
		if start := processStartStamp(pid); start != "" {
			s.known[pid] = start
		}
	}
}

// Kill sends SIGKILL to every tracked process that is still the one that was recorded: the start time
// must match, so a pid recycled by an unrelated process is never signalled. It returns the number killed.
func (s *Survivors) Kill() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	killed := 0
	for pid, start := range s.known {
		if processStartStamp(pid) != start {
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err == nil {
			killed++
		}
	}
	s.known = make(map[int]string)
	return killed
}
