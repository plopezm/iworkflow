//go:build !(linux && cgo && gtk3)

package main

import "unsafe"

// trustLocalCertificate (and the external-link guard) is not implemented yet for this platform (Windows WebView2 and macOS WKWebView need
// their own hooks; see odd/tasks/wails3-migration.md T2).
func trustLocalCertificate(unsafe.Pointer, int) error { return nil }

func setTrustedPort(int) {}
