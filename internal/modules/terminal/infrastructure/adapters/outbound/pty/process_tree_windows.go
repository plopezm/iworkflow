//go:build windows

package pty

import "iworkflow/internal/core/process"

// killProcessTree kills the shell and everything it launched with taskkill /T. Windows has no session
// sweep; the parent tree is the only way to find the children. Errors are ignored: the tree may already
// be gone.
func killProcessTree(pid int) {
	_ = process.KillTree(uint32(pid))
}
