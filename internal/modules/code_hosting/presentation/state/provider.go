// Package state builds the view-models of the code hosting screens. The components render them as
// they are: every text is translated here, and the badge variants are decided here.
package state

import (
	"context"
	"strings"
)

// Provider identifiers as the transport carries them.
const (
	providerGitLab = "gitlab"
	providerGitHub = "github"
	githubComURL   = "https://github.com"
)

func isGitHub(provider string) bool {
	return provider == providerGitHub
}

// providerLabel is the name shown to the user. Unknown identifiers use the GitLab vocabulary, as the
// Rust version did.
func providerLabel(provider string) string {
	if isGitHub(provider) {
		return "GitHub"
	}
	return "GitLab"
}

// requestAbbrev is the short name of a request: PR on GitHub, MR on GitLab.
func requestAbbrev(provider string) string {
	if isGitHub(provider) {
		return "PR"
	}
	return "MR"
}

// requestPrefix precedes the number of a request: #12 on GitHub, !12 on GitLab.
func requestPrefix(provider string) string {
	if isGitHub(provider) {
		return "#"
	}
	return "!"
}

// pipelineNoun names the CI run: workflow on GitHub, pipeline on GitLab.
func pipelineNoun(provider string) string {
	if isGitHub(provider) {
		return "Workflow"
	}
	return "Pipeline"
}

// urlPlaceholder is the example base URL of the form.
func urlPlaceholder(provider string) string {
	if isGitHub(provider) {
		return githubComURL
	}
	return "https://gitlab.miempresa.local"
}

// tokenPlaceholder is the example token of the form, shown when no token is stored.
func tokenPlaceholder(provider string) string {
	if isGitHub(provider) {
		return "ghp_… / github_pat_…"
	}
	return "glpat-…"
}

// initialBaseURL is the URL the form starts with: GitHub proposes github.com while nothing is stored.
func initialBaseURL(provider string, stored string) string {
	if strings.TrimSpace(stored) == "" && isGitHub(provider) {
		return githubComURL
	}
	return stored
}

// providerHelp is the help text of the settings card of a provider.
func providerHelp(ctx context.Context, provider string) string {
	if isGitHub(provider) {
		return msgHelpGitHub.T(ctx)
	}
	return msgHelpGitLab.T(ctx)
}
