package components

// terminalKey is one special key under the terminal: its accessible name, the key name understood by
// web/static/js/terminal_view.js and terminal.js, and the SVG content of its icon (24x24 grid).
type terminalKey struct {
	Label string
	Key   string
	Icon  string
}

// terminalKeys are the keys offered under the terminal, in display order. Every Key must be handled by
// web/static/terminal.js; the test in this package checks it.
var terminalKeys = []terminalKey{
	{Label: "Esc", Key: "esc", Icon: `<circle cx="12" cy="12" r="9"/><path d="m9 9 6 6M15 9l-6 6"/>`},
	{Label: "Tab", Key: "tab", Icon: `<path d="M3 12h14"/><path d="m13 8 4 4-4 4"/><path d="M21 5v14"/>`},
	{Label: "Up", Key: "up", Icon: `<path d="m5 12 7-7 7 7"/><path d="M12 19V5"/>`},
	{Label: "Left", Key: "left", Icon: `<path d="m12 19-7-7 7-7"/><path d="M19 12H5"/>`},
	{Label: "Right", Key: "right", Icon: `<path d="M5 12h14"/><path d="m12 5 7 7-7 7"/>`},
	{Label: "Down", Key: "down", Icon: `<path d="M12 5v14"/><path d="m19 12-7 7-7-7"/>`},
	{Label: "Ctrl+C", Key: "ctrl_c", Icon: `<text x="12" y="16" text-anchor="middle" font-size="11" font-weight="700" fill="currentColor" stroke="none">^C</text>`},
	{Label: "Copy", Key: "copy", Icon: `<rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>`},
	{Label: "Paste", Key: "paste", Icon: `<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>`},
}
