//go:build !windows

package desktop

import "os/exec"

// hideConsole is only needed on Windows, where a console child opens its own terminal window.
func hideConsole(*exec.Cmd) {}
