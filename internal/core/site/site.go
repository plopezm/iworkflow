// Package site resolves the static site assets (CSS, JS) for a packaged binary.
//
// A release archive ships the executable next to a `site/` directory. Without help the server
// would look for the assets relative to the current directory and serve the page unstyled.
package site

import (
	"net/netip"
	"os"
	"path/filepath"

	"iworkflow/internal/core/listen"
)

const (
	siteDirName   string = "site"
	outputName    string = "iworkflow"
	siteRootVar   string = "LEPTOS_SITE_ROOT"
	outputNameVar string = "LEPTOS_OUTPUT_NAME"
	packagedPort  uint16 = 9000
)

// Options are the site settings the server runs with.
type Options struct {
	// SiteRoot is the directory with the static assets.
	SiteRoot string
	// OutputName is the base name of the built assets.
	OutputName string
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

// ApplyPackagedDefaults fills in the options the environment did not set explicitly. An empty
// exeDir means the executable location is unknown.
func ApplyPackagedDefaults(options Options, exeDir string, siteRootSet bool, outputNameSet bool) Options {
	if !outputNameSet {
		options.OutputName = outputName
	}
	if !siteRootSet && exeDir != "" {
		if root, ok := BundledSiteRoot(exeDir); ok {
			options.SiteRoot = root
		}
	}
	return options
}

// ApplyPackagedPort moves the listen port to the packaged default unless the environment pinned
// the address. The host is kept (loopback), so the network-access setting still decides exposure.
func ApplyPackagedPort(options Options, siteAddrSet bool) Options {
	if !siteAddrSet {
		options.SiteAddr = netip.AddrPortFrom(options.SiteAddr.Addr(), packagedPort)
	}
	return options
}

// WithPackagedDefaults applies the packaged defaults using the real environment and executable path.
func WithPackagedDefaults(options Options) Options {
	exeDir := ""
	// Without an executable path the defaults keep the configured site root, as before.
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	_, siteRootSet := os.LookupEnv(siteRootVar)
	_, outputNameSet := os.LookupEnv(outputNameVar)
	_, siteAddrSet := os.LookupEnv(listen.SiteAddrVar)
	options = ApplyPackagedDefaults(options, exeDir, siteRootSet, outputNameSet)
	return ApplyPackagedPort(options, siteAddrSet)
}
