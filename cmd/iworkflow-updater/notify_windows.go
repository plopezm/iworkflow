//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	mbOK        = 0x00000000
	mbIconError = 0x00000010
	mbTopmost   = 0x00040000
)

// notifyFailure shows a native message box: the helper has no console, so without it a failed update would
// leave the user with no sign of what happened.
func notifyFailure(message string) {
	title, err := syscall.UTF16PtrFromString("iworkflow update")
	if err != nil {
		return
	}
	text, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	proc := syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), mbOK|mbIconError|mbTopmost) //nolint:errcheck // nothing more to do when the box cannot be shown
}
