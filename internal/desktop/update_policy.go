package desktop

import (
	"fmt"
	"strconv"
	"strings"
)

// Pure decisions of the update system, mirroring electron/src/update-policy.js. Nothing here performs I/O:
// the environment is read through an injected function and the release payload is passed in already decoded.

// UpdateRepo is the GitHub repository whose latest release is offered as an update.
const UpdateRepo = "plopezm/iworkflow"

// EnvUpdateCheck is the environment variable that disables update checks when set to a disabled value.
const EnvUpdateCheck = "IWF_UPDATE_CHECK"

// EnvAppImage is the environment variable set by the AppImage runtime.
const EnvAppImage = "APPIMAGE"

// Strategy says how this installation handles updates.
type Strategy string

const (
	// StrategyAuto downloads and installs the new version itself (Windows installer, Linux AppImage).
	StrategyAuto Strategy = "auto"
	// StrategyNotify only tells the user and opens the release page. Used on macOS (ad hoc signing)
	// and on Linux builds that are not an AppImage, which cannot replace themselves.
	StrategyNotify Strategy = "notify"
	// StrategyOff performs no check at all (development runs or IWF_UPDATE_CHECK=off).
	StrategyOff Strategy = "off"
)

// PolicyInput is what UpdateStrategy needs to decide. GOOS uses Go names ("windows", "linux", "darwin").
// Getenv may be nil, which behaves like an empty environment.
type PolicyInput struct {
	GOOS     string
	Packaged bool
	Getenv   func(key string) string
}

// UpdateStrategy returns how updates are handled for the given build and environment.
func UpdateStrategy(in PolicyInput) Strategy {
	if !in.Packaged {
		return StrategyOff
	}
	getenv := in.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	switch strings.ToLower(getenv(EnvUpdateCheck)) {
	case "off", "0", "false":
		return StrategyOff
	}
	// Every OS updates itself from the release archive (ArchiveInstaller). When the install folder is not
	// writable the installer reports ErrAutoUpdateUnsupported and the flow falls back to the notice.
	return StrategyAuto
}

// LatestReleaseAPIURL returns the GitHub API URL of the latest release of repo ("owner/name").
func LatestReleaseAPIURL(repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
}

// Version is a plain release version `[major, minor, patch]`.
type Version [3]int

// ParseVersion parses a plain `x.y.z` version with an optional leading `v`. Anything else, including
// pre-release or build suffixes, reports false.
func ParseVersion(text string) (Version, bool) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(text), "v")
	parts := strings.Split(trimmed, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var version Version
	for i, part := range parts {
		if !isDigits(part) {
			return Version{}, false
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, false
		}
		version[i] = n
	}
	return version, true
}

// IsNewerVersion reports whether candidate is a plain release version strictly greater than current.
func IsNewerVersion(current string, candidate string) bool {
	have, ok := ParseVersion(current)
	if !ok {
		return false
	}
	next, ok := ParseVersion(candidate)
	if !ok {
		return false
	}
	for i := range have {
		if next[i] != have[i] {
			return next[i] > have[i]
		}
	}
	return false
}

// Release is what the update notice needs from the latest GitHub release.
type Release struct {
	// Version is the tag without its leading `v`.
	Version string
	// URL is the HTML page of the release.
	URL string
}

// ReleasePayload is the subset of the GitHub "latest release" response that is read.
type ReleasePayload struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// ParseLatestRelease reduces a GitHub "latest release" payload to what the notice needs. It reports false
// for drafts, pre-releases and payloads without a tag or a page URL.
func ParseLatestRelease(payload *ReleasePayload) (Release, bool) {
	if payload == nil || payload.Draft || payload.Prerelease {
		return Release{}, false
	}
	if payload.TagName == "" || payload.HTMLURL == "" {
		return Release{}, false
	}
	return Release{Version: strings.TrimPrefix(payload.TagName, "v"), URL: payload.HTMLURL}, true
}

func isDigits(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
