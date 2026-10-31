// Package site resolves the static site assets (CSS, JS) for a packaged binary.
//
// A release archive ships the executable next to a `site/` directory. Without help the server
// would look for the assets relative to the current directory and serve the page unstyled.
package site

import (
	"net/netip"
	"os"
	"path/filepath"
)

const (
	siteDirName  string = "site"
	packagedPort uint16 = 9000
)

// Options are the site settings the server runs with.
type Options struct {
	// SiteRoot is the directory with the static assets.
	SiteRoot string
	// SiteAddr is the address the server listens on.
	SiteAddr netip.AddrPort
}

// BundledSiteRoot returns exeDir/site when that directory exists.
func BundledSiteRoot(exeDir string) (string, bool) {
	candidate := filepath.Join(exeDir, siteDirName)
	info, err := os.Stat(candidate)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return candidate, true
}

// ApplyPackagedDefaults replaces the site root with the one bundled next to the executable unless
// it was set explicitly. An empty exeDir means the executable location is unknown.
func ApplyPackagedDefaults(options Options, exeDir string, siteRootSet bool) Options {
	if !siteRootSet && exeDir != "" {
		if root, ok := BundledSiteRoot(exeDir); ok {
			options.SiteRoot = root
		}
	}
	return options
}

// ApplyPackagedPort moves the listen port to the packaged default unless the address was set
// explicitly. The host is kept (loopback), so the network-access setting still decides exposure.
func ApplyPackagedPort(options Options, siteAddrSet bool) Options {
	if !siteAddrSet {
		options.SiteAddr = netip.AddrPortFrom(options.SiteAddr.Addr(), packagedPort)
	}
	return options
}

// WithPackagedDefaults applies the packaged defaults using the real executable path.
func WithPackagedDefaults(options Options, siteRootSet bool, siteAddrSet bool) Options {
	exeDir := ""
	// Without an executable path the defaults keep the configured site root, as before.
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	options = ApplyPackagedDefaults(options, exeDir, siteRootSet)
	return ApplyPackagedPort(options, siteAddrSet)
}
