//go:build unix

package pty

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"

	"iworkflow/internal/core/process"
)

const (
	// procDir is where Linux exposes one directory per process.
	procDir string = "/proc"
	// maxKillPasses covers processes that fork while the previous pass is killing others.
	maxKillPasses int = 5
)

// procEntry holds the stat fields that locate the processes of one terminal.
type procEntry struct {
	pid     int
	ppid    int
	session int
}

// parseStat extracts ppid and session from a /proc/<pid>/stat line
// (`pid (comm) state ppid pgrp session ...`). comm may contain spaces and parentheses, so the line
// is split after the last ')'.
func parseStat(pid int, stat string) (procEntry, bool) {
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return procEntry{}, false
	}
	fields := strings.Fields(stat[closing+1:])
	// fields: state ppid pgrp session ...
	if len(fields) < 4 {
		return procEntry{}, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return procEntry{}, false
	}
	session, err := strconv.Atoi(fields[3])
	if err != nil {
		return procEntry{}, false
	}
	return procEntry{pid: pid, ppid: ppid, session: session}, true
}

// scanProc reads every process visible in /proc. Entries that vanish while being read are skipped.
func scanProc() []procEntry {
	dir, err := os.ReadDir(procDir)
	if err != nil {
		return nil
	}
	procs := make([]procEntry, 0, len(dir))
	for _, entry := range dir {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(procDir + "/" + entry.Name() + "/stat")
		if err != nil {
			continue
		}
		if parsed, ok := parseStat(pid, string(stat)); ok {
			procs = append(procs, parsed)
		}
	}
	return procs
}

// members returns the processes of the terminal whose session leader is root: the whole session
// (including orphans re-parented to init, background or foreground) plus every descendant by parent
// chain, which covers the processes that started their own session with setsid. root is excluded.
func members(root int, procs []procEntry) []int {
	found := make(map[int]struct{})
	for _, proc := range procs {
		if proc.session == root && proc.pid != root {
			found[proc.pid] = struct{}{}
		}
	}
	frontier := []int{root}
	for pid := range found {
		frontier = append(frontier, pid)
	}
	for len(frontier) > 0 {
		parent := frontier[len(frontier)-1]
		frontier = frontier[:len(frontier)-1]
		for _, proc := range procs {
			if proc.ppid != parent || proc.pid == root {
				continue
			}
			if _, seen := found[proc.pid]; !seen {
				found[proc.pid] = struct{}{}
				frontier = append(frontier, proc.pid)
			}
		}
	}
	pids := make([]int, 0, len(found))
	for pid := range found {
		pids = append(pids, pid)
	}
	slices.Sort(pids)
	return pids
}

// killDescendants SIGKILLs everything the shell root launched (its session and its descendants). It must
// run before the shell dies: afterwards its children are re-parented to init and can no longer be found
// by parent chain. The shell itself is not killed here.
func killDescendants(root int) {
	for range maxKillPasses {
		pids := members(root, scanProc())
		if len(pids) == 0 {
			return
		}
		for _, pid := range pids {
			// A process may exit between the scan and the signal, so the error is expected.
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

// killProcessTree kills the terminal's processes. The shell leads its own session and process group
// (creack/pty calls setsid), so the group reaches every job that kept it, and killDescendants reaches
// the ones that left it. Errors are ignored: the processes may already be gone.
func killProcessTree(pid int) {
	killDescendants(pid)
	_ = process.KillGroup(pid)
}
