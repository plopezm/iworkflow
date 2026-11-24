//go:build !(linux && cgo && gtk3)

package main

import "unsafe"

// trustLocalCertificate is not implemented yet for this platform (Windows WebView2 and macOS WKWebView need
// their own hooks; see odd/tasks/wails3-migration.md T2).
func trustLocalCertificate(unsafe.Pointer) error { return nil }
