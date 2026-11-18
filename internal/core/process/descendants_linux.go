//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// processParents maps every process listed in /proc to its parent pid. An entry that exits while it is read is
// skipped: a process that is gone has no descendants left to report.
func processParents(ctx context.Context) (map[int]int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", procRoot, err)
	}
	parents := make(map[int]int, len(entries))
	for _, entry := range entries {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("scan the processes: %w", ctx.Err())
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile(procRoot + "/" + entry.Name() + "/stat")
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read the stat of process %d: %w", pid, err)
		}
		parent, err := parentPID(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse the stat of process %d: %w", pid, err)
		}
		parents[pid] = parent
	}
	return parents, nil
}

// parentPID extracts the parent pid (field 4 of the line) from a /proc/<pid>/stat line. comm may hold spaces
// and parentheses, so the line is split after the last ')': the state comes first, then the parent pid.
func parentPID(stat string) (int, error) {
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return 0, fmt.Errorf("no command name in %q", stat)
	}
	fields := strings.Fields(stat[closing+1:])
	if len(fields) < 2 {
		return 0, fmt.Errorf("too few fields in %q", stat)
	}
	parent, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, fmt.Errorf("parent pid %q: %w", fields[1], err)
	}
	return parent, nil
}
