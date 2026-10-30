//go:build !unix

package git

import "os/exec"

// killProcessTree falls back to killing git itself on platforms without process groups. Children that
// git started may outlive the kill there; this is not verified on Windows.
func killProcessTree(*exec.Cmd) {}
