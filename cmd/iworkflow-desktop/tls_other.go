//go:build !linux

package main

import "unsafe"

// The web view hooks (external-link guard, trust of the local self-signed certificate) are not implemented yet
// for this platform: Windows WebView2 and macOS WKWebView need their own (see odd/tasks/wails3-migration.md T2).

func installWebViewGuards(unsafe.Pointer, int) error { return nil }

func allowServerCertificate(string, []byte) error { return nil }

func setTrustedPort(int) {}
