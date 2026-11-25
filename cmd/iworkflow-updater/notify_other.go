//go:build !windows

package main

import (
	"fmt"
	"os"
)

// notifyFailure prints the failure; outside Windows the helper still has the terminal of its log only.
func notifyFailure(message string) {
	fmt.Fprintln(os.Stderr, message)
}
