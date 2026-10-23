// Package browser holds the browser-side contract shared with the server.
//
// The original browser utilities (localStorage, cookies, scroll following, clipboard, downloads)
// ran only inside the WASM client. In the Go version that behavior lives in the static JavaScript
// under web/static, so the server keeps only the names both sides must agree on.
package browser

// ThemeStorageKey is the localStorage key where the theme mode is saved.
const ThemeStorageKey string = "pi-theme"
