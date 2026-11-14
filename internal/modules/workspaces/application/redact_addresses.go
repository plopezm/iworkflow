package application

import "strings"

// addressMarker is what replaces a service address in a text that is about to be logged.
const addressMarker = "[address]"

// redactAddresses replaces every service address in an error text, so the provider address and any credentials
// inside it never reach the log even when a port error carries them: URLs of any scheme (credentials included),
// user:password@host pairs and bare host:port pairs such as the ones of "dial tcp" errors. The rest of the message is
// kept, because it is what helps debugging. It works word by word and holds no state, so there is nothing to compile
// and nothing that can fail at start-up.
func redactAddresses(text string) string {
	if text == "" {
		return text
	}
	words := strings.Split(text, " ")
	for index, word := range words {
		words[index] = redactWord(word)
	}
	return strings.Join(words, " ")
}

// redactWord redacts one space-free word and keeps the punctuation around the address: Get "http://x/y": becomes
// Get "[address]":.
func redactWord(word string) string {
	core := strings.TrimLeft(word, `("'`)
	prefix := word[:len(word)-len(core)]
	trimmed := strings.TrimRight(core, `.,;:)"'`)
	suffix := core[len(trimmed):]
	if !isServiceAddress(trimmed) {
		return word
	}
	return prefix + addressMarker + suffix
}

// isServiceAddress reports whether a word is a URL, a credentials pair or a host:port pair.
func isServiceAddress(word string) bool {
	switch {
	case word == "":
		return false
	case strings.Contains(word, "://"):
		return true
	case strings.Contains(word, "@"):
		return true
	default:
		return isHostPort(word)
	}
}

// isHostPort reports whether a word is host:port with a numeric port and a host that looks like an address: a
// dotted name or IPv4, localhost, or a bracketed IPv6. Plain words with a colon (error:, retry:5) are not addresses.
func isHostPort(word string) bool {
	cut := strings.LastIndex(word, ":")
	if cut <= 0 || cut == len(word)-1 {
		return false
	}
	host, port := word[:cut], word[cut+1:]
	if len(port) > 5 || strings.Trim(port, "0123456789") != "" {
		return false
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		return len(host) > 2
	}
	if host == "localhost" {
		return true
	}
	return strings.Contains(host, ".") && strings.Trim(host, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-") == ""
}
