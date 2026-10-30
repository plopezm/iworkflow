package models

import (
	"slices"
	"strings"

	"iworkflow/internal/modules/code_hosting/domain/errors"
)

// remoteSchemes are the URL schemes accepted for a remote; local paths and others are rejected.
var remoteSchemes = []string{"http", "https", "ssh", "git"}

// RemoteURL is the host and path of a git remote, without credentials or port.
type RemoteURL struct {
	Host string
	// Path is the repository path without leading or trailing slashes and without the `.git` suffix.
	Path string
}

// ParseRemoteURL accepts `https://[user@]host[:port]/path.git`, `ssh://git@host[:port]/path.git` and
// the scp form `git@host:path.git`. Local paths and unknown schemes are rejected.
func ParseRemoteURL(raw string) (RemoteURL, error) {
	value := strings.TrimSpace(raw)
	unparsable := func() error { return errors.UnparsableRemote(redact(value)) }
	var authority, path string
	if scheme, rest, found := strings.Cut(value, "://"); found {
		if !slices.Contains(remoteSchemes, scheme) {
			return RemoteURL{}, unparsable()
		}
		var ok bool
		authority, path, ok = strings.Cut(rest, "/")
		if !ok {
			return RemoteURL{}, unparsable()
		}
	} else if !strings.HasPrefix(value, "/") && !strings.HasPrefix(value, ".") {
		var ok bool
		authority, path, ok = strings.Cut(value, ":")
		if !ok {
			return RemoteURL{}, unparsable()
		}
	} else {
		return RemoteURL{}, unparsable()
	}
	host := HostOf(authority)
	path = trimGitSuffix(strings.Trim(path, "/"))
	path = strings.Trim(path, "/")
	if host == "" || path == "" {
		return RemoteURL{}, unparsable()
	}
	return RemoteURL{Host: host, Path: path}, nil
}

// trimGitSuffix removes every trailing ".git", like Rust's trim_end_matches.
func trimGitSuffix(path string) string {
	for strings.HasSuffix(path, ".git") {
		path = strings.TrimSuffix(path, ".git")
	}
	return path
}

// HostOf extracts the lowercase host from `[user[:pw]@]host[:port]`, handling bracketed IPv6.
func HostOf(authority string) string {
	withoutUser := authority
	if index := strings.LastIndex(authority, "@"); index >= 0 {
		withoutUser = authority[index+1:]
	}
	var host string
	if strings.HasPrefix(withoutUser, "[") {
		if before, _, found := strings.Cut(withoutUser, "]"); found {
			host = strings.TrimLeft(before, "[")
		} else {
			host = withoutUser
		}
	} else {
		host, _, _ = strings.Cut(withoutUser, ":")
	}
	return asciiLower(host)
}

// redact hides the credentials embedded in a URL before it is shown in an error.
func redact(value string) string {
	scheme, rest, found := strings.Cut(value, "://")
	if !found {
		return value
	}
	index := strings.LastIndex(rest, "@")
	if index < 0 {
		return value
	}
	return scheme + "://***@" + rest[index+1:]
}
