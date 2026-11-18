//go:build unix && !linux

package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// processParents maps every process to its parent pid through `ps -A -o pid=,ppid=`, which every macOS and BSD
// ships. The command is bounded by ctx. A line that does not hold exactly two numbers fails the whole read,
// since a partial tree would silently miss descendants.
func processParents(ctx context.Context) (map[int]int, error) {
	out, err := exec.CommandContext(ctx, "ps", "-A", "-o", "pid=,ppid=").Output()
	if err != nil {
		return nil, fmt.Errorf("run ps: %w", err)
	}
	parents := map[int]int{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("unexpected ps line %q", line)
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("pid in ps line %q: %w", line, err)
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("ppid in ps line %q: %w", line, err)
		}
		parents[pid] = ppid
	}
	return parents, nil
}
