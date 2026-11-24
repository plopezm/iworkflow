// Package contentsearch implements the content search of the project: ripgrep when it is installed and
// a pure Go walker otherwise. Both return the same shape and honour the same limits.
package contentsearch

import (
	"strings"
	"unicode/utf8"
)

// maxSnippetBytes caps the line text kept per match so a minified file cannot bloat the page.
const maxSnippetBytes int = 300

// snippetOf returns line without its line terminator, cut at a rune boundary (pure).
func snippetOf(line string) string {
	line = strings.TrimRight(line, "\r\n")
	if len(line) <= maxSnippetBytes {
		return line
	}
	cut := maxSnippetBytes
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return line[:cut]
}

// charCol converts a byte offset into a 1-based column in characters (pure).
func charCol(line string, byteOffset int) int {
	byteOffset = min(max(byteOffset, 0), len(line))
	return utf8.RuneCountInString(line[:byteOffset]) + 1
}
