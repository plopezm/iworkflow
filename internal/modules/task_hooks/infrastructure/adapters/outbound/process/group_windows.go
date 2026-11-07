//go:build windows

package process

import "os/exec"

// ownProcessGroup is a no-op on Windows: only the direct child is killed on cancellation.
func ownProcessGroup(*exec.Cmd) {}
