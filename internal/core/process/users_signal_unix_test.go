//go:build unix

package process

import (
	"os"
	"syscall"
)

func syscallZero() os.Signal { return syscall.Signal(0) }
