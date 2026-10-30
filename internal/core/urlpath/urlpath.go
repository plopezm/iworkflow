// Package urlpath encodes relative file paths (such as "a b/c.rs") as URL path segments for tab URLs.
package urlpath

import "strings"

const hexDigits string = "0123456789ABCDEF"

// isUnreserved reports whether b is an RFC 3986 unreserved character, which is never escaped.
func isUnreserved(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') ||
		b == '-' || b == '.' || b == '_' || b == '~'
}

// EncodePath percent-encodes every byte of path except the "/" separators and unreserved characters.
func EncodePath(path string) string {
	var out strings.Builder
	out.Grow(len(path))
	for index := range len(path) {
		b := path[index]
		if b == '/' || isUnreserved(b) {
			out.WriteByte(b)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hexDigits[b>>4])
		out.WriteByte(hexDigits[b&0x0F])
	}
	return out.String()
}

// DecodePath reverses EncodePath. Malformed escapes are kept as they are.
func DecodePath(encoded string) string {
	out := make([]byte, 0, len(encoded))
	for index := 0; index < len(encoded); {
		if encoded[index] == '%' && index+3 <= len(encoded) {
			if value, ok := parseEscape(encoded[index+1], encoded[index+2]); ok {
				out = append(out, value)
				index += 3
				continue
			}
		}
		out = append(out, encoded[index])
		index++
	}
	return strings.ToValidUTF8(string(out), "\uFFFD")
}

// parseEscape reads the two characters of a %XX escape. It accepts a leading '+' in the first
// position, as Rust's from_str_radix did in the original implementation.
func parseEscape(high byte, low byte) (byte, bool) {
	if high == '+' {
		digit, ok := hexValue(low)
		return digit, ok
	}
	upper, okHigh := hexValue(high)
	lower, okLow := hexValue(low)
	if !okHigh || !okLow {
		return 0, false
	}
	return upper<<4 | lower, true
}

func hexValue(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	default:
		return 0, false
	}
}
