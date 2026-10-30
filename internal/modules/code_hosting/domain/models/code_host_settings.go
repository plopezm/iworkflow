//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"fmt"
	"strings"
	"unicode"

	"iworkflow/internal/modules/code_hosting/domain/errors"
)

const (
	githubCom       = "github.com"
	githubSSHHost   = "ssh.github.com"
	httpsScheme     = "https://"
	httpScheme      = "http://"
	credentialsHint = "no incluyas credenciales en la URL"
)

// CodeHostSettings is the connection to one instance of a hosting provider. The token never appears
// in its textual form: String and GoString hide it, so it cannot reach a log by accident.
type CodeHostSettings struct {
	provider HostingProvider
	baseURL  string
	token    string
	// InsecureTLS accepts untrusted TLS certificates (internal CA or self-signed).
	InsecureTLS bool
}

// NewCodeHostSettings validates and normalizes the connection: the base URL is trimmed and loses its
// trailing slashes, and the token is trimmed and must not be blank.
func NewCodeHostSettings(provider HostingProvider, baseURL string, token string, insecureTLS bool) (CodeHostSettings, error) {
	normalized := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	rest, found := cutScheme(normalized)
	if !found {
		return CodeHostSettings{}, errors.InvalidURL(normalized)
	}
	if rest == "" || strings.HasPrefix(rest, "/") || strings.IndexFunc(rest, unicode.IsSpace) >= 0 {
		return CodeHostSettings{}, errors.InvalidURL(normalized)
	}
	if strings.Contains(rest, "@") {
		return CodeHostSettings{}, errors.InvalidURL(credentialsHint)
	}
	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		return CodeHostSettings{}, errors.MissingToken()
	}
	return CodeHostSettings{
		provider:    provider,
		baseURL:     normalized,
		token:       trimmedToken,
		InsecureTLS: insecureTLS,
	}, nil
}

// cutScheme strips the http or https scheme and reports whether one was present.
func cutScheme(value string) (string, bool) {
	if rest, found := strings.CutPrefix(value, httpsScheme); found {
		return rest, true
	}
	return strings.CutPrefix(value, httpScheme)
}

// SelectForHost picks the settings of the provider that owns the host of `origin`. A github.com host
// without settings is reported as an unconfigured GitHub; any other unknown host as UnknownHost.
func SelectForHost(configured []CodeHostSettings, remoteHost string) (CodeHostSettings, error) {
	for _, settings := range configured {
		if settings.MatchesHost(remoteHost) {
			return settings, nil
		}
	}
	if remoteHost == githubCom || remoteHost == githubSSHHost {
		return CodeHostSettings{}, errors.NotConfigured(GitHub)
	}
	return CodeHostSettings{}, errors.UnknownHost(remoteHost)
}

// Provider is the platform these settings belong to.
func (s CodeHostSettings) Provider() HostingProvider { return s.provider }

// BaseURL is the normalized instance URL, without trailing slash.
func (s CodeHostSettings) BaseURL() string { return s.baseURL }

// Token is the access token. Only the code that talks to the provider may read it.
func (s CodeHostSettings) Token() string { return s.token }

// String shows the settings with the token masked.
func (s CodeHostSettings) String() string {
	return fmt.Sprintf("CodeHostSettings{provider: %s, base_url: %s, token: ***, insecure_tls: %t}",
		s.provider, s.baseURL, s.InsecureTLS)
}

// GoString is the %#v form, masked the same way.
func (s CodeHostSettings) GoString() string { return s.String() }

// APIURL is the root of the REST API: GitLab `/api/v4`; GitHub `api.github.com` or, on Enterprise,
// `/api/v3` under the configured URL.
func (s CodeHostSettings) APIURL() string {
	if s.provider == GitHub {
		if s.Host() == githubCom {
			return "https://api.github.com"
		}
		return s.baseURL + "/api/v3"
	}
	return s.baseURL + "/api/v4"
}

// PushURL is the HTTP(S) git URL of the project on this instance, without credentials:
// `<base>/<project>.git`. Pushing to it uses the token even when `origin` is SSH.
func (s CodeHostSettings) PushURL(project string) string {
	return s.baseURL + "/" + strings.Trim(project, "/") + ".git"
}

// GitUsername is the user that the provider accepts with the token as password over git HTTP.
func (s CodeHostSettings) GitUsername() string {
	if s.provider == GitHub {
		return "x-access-token"
	}
	return "oauth2"
}

// authorityAndPath splits the base URL (scheme removed) into its authority and its path prefix.
func (s CodeHostSettings) authorityAndPath() (string, string) {
	rest := s.baseURL
	if _, after, found := strings.Cut(s.baseURL, "://"); found {
		rest = after
	}
	authority, path, _ := strings.Cut(rest, "/")
	return authority, path
}

// Host is the lowercase host of the base URL, without port.
func (s CodeHostSettings) Host() string {
	authority, _ := s.authorityAndPath()
	return HostOf(authority)
}

// MatchesHost tells whether a host of `origin` belongs to this instance. It ignores port and scheme;
// `ssh.github.com` is the alternative SSH endpoint of github.com.
func (s CodeHostSettings) MatchesHost(remoteHost string) bool {
	configured := s.Host()
	return remoteHost == configured ||
		(s.provider == GitHub && configured == githubCom && remoteHost == githubSSHHost)
}

// ProjectPath is the `namespace/project` inside this instance for the `origin` remote. It fails when
// the host differs, and it strips the relative URL root (`/gitlab`) when the instance has one.
func (s CodeHostSettings) ProjectPath(remote RemoteURL) (string, error) {
	if !s.MatchesHost(remote.Host) {
		return "", errors.HostMismatch(remote.Host, s.Host())
	}
	_, prefixPath := s.authorityAndPath()
	prefix := strings.Trim(prefixPath, "/")
	path := remote.Path
	if prefix != "" {
		if remainder, found := strings.CutPrefix(remote.Path, prefix); found {
			path = strings.TrimLeft(remainder, "/")
		}
	}
	// GitHub only knows `owner/repository`.
	if s.provider == GitHub && strings.Count(path, "/") != 1 {
		return "", errors.InvalidProjectPath(s.provider, path)
	}
	return path, nil
}
