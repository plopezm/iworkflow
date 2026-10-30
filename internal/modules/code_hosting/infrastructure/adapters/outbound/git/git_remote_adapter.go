// Package git reads and publishes the origin remote of a task repository through the git CLI.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"iworkflow/internal/core/gitx"
	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

const (
	originRemote       = "origin"
	emptyCommitMessage = "chore: empty commit to open the PR/MR"
	// fallbackName and fallbackEmail identify the empty commit only when git has no identity set.
	fallbackName  = "iworkflow"
	fallbackEmail = "iworkflow@localhost"
)

// RemoteAdapter implements outbound.GitRemote with the git CLI.
type RemoteAdapter struct{}

var _ outbound.GitRemote = (*RemoteAdapter)(nil)

// NewRemoteAdapter builds the adapter.
func NewRemoteAdapter() *RemoteAdapter {
	return &RemoteAdapter{}
}

// OriginURL returns the fetch URL of origin. A repository without origin fails with NoOrigin.
func (a *RemoteAdapter) OriginURL(ctx context.Context, repo string) (string, error) {
	output, err := gitx.RunRaw(ctx, repo, "remote", "get-url", originRemote)
	if err != nil {
		return "", codeerrors.Git(err.Error())
	}
	url := strings.TrimSpace(output.Stdout)
	if output.Success && url != "" {
		return url, nil
	}
	return "", codeerrors.NoOrigin(repo)
}

// PublishBranch pushes the branch to the HTTP(S) URL of the project. The token travels in the
// environment of the git process (never in its arguments), and only to the configured instance.
func (a *RemoteAdapter) PublishBranch(ctx context.Context, repo string, branch string, settings models.CodeHostSettings, project string) error {
	refspec := "refs/heads/" + branch + ":refs/heads/" + branch
	output, err := gitx.RunRawEnv(ctx, repo, tokenEnv(settings), "push", settings.PushURL(project), refspec)
	if err != nil {
		return codeerrors.Git(err.Error())
	}
	if output.Success {
		return nil
	}
	return codeerrors.BranchPublishFailed(branch, output.Stderr)
}

// EnsureCommitsAhead adds an empty commit to branch when it does not advance over base. The working
// tree and the index are not touched: the commit reuses the tree of the branch.
func (a *RemoteAdapter) EnsureCommitsAhead(ctx context.Context, repo string, branch string, base string) (bool, error) {
	count, err := gitx.RunRaw(ctx, repo, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return false, codeerrors.Git(err.Error())
	}
	// If the base does not resolve locally, the provider decides.
	if !count.Success || strings.TrimSpace(count.Stdout) != "0" {
		return false, nil
	}
	head, err := revParse(ctx, repo, "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	tree, err := revParse(ctx, repo, "refs/heads/"+branch+"^{tree}")
	if err != nil {
		return false, err
	}
	identity, err := fallbackIdentity(ctx, repo)
	if err != nil {
		return false, err
	}
	commit, err := gitx.RunRawEnv(ctx, repo, identity, "commit-tree", tree, "-p", head, "-m", emptyCommitMessage)
	if err != nil {
		return false, codeerrors.Git(err.Error())
	}
	newHead := strings.TrimSpace(commit.Stdout)
	if !commit.Success || newHead == "" {
		return false, codeerrors.Git(commit.Stderr)
	}
	// update-ref with the old value fails if someone moved the branch in the meantime.
	moved, err := gitx.RunRaw(ctx, repo, "update-ref", "refs/heads/"+branch, newHead, head)
	if err != nil {
		return false, codeerrors.Git(err.Error())
	}
	if !moved.Success {
		return false, codeerrors.Git(moved.Stderr)
	}
	return true, nil
}

// revParse resolves a revision to its object name.
func revParse(ctx context.Context, repo string, spec string) (string, error) {
	output, err := gitx.RunRaw(ctx, repo, "rev-parse", "--verify", spec)
	if err != nil {
		return "", codeerrors.Git(err.Error())
	}
	value := strings.TrimSpace(output.Stdout)
	if output.Success && value != "" {
		return value, nil
	}
	return "", codeerrors.Git(output.Stderr)
}

// fallbackIdentity returns the author and committer identity only when git has none configured,
// because commit-tree requires one.
func fallbackIdentity(ctx context.Context, repo string) ([]string, error) {
	identity, err := gitx.RunRaw(ctx, repo, "var", "GIT_COMMITTER_IDENT")
	if err != nil {
		return nil, codeerrors.Git(err.Error())
	}
	if identity.Success {
		return nil, nil
	}
	env := make([]string, 0, 4)
	for _, role := range []string{"AUTHOR", "COMMITTER"} {
		env = append(env,
			"GIT_"+role+"_NAME="+fallbackName,
			"GIT_"+role+"_EMAIL="+fallbackEmail,
		)
	}
	return env, nil
}

// tokenEnv configures git through GIT_CONFIG_* environment variables, so the token does not appear in
// the command line. The Basic header is scoped to the configured instance, and any credential helper
// of the user is cleared. A self-signed instance also disables certificate verification for it.
func tokenEnv(settings models.CodeHostSettings) []string {
	credentials := base64.StdEncoding.EncodeToString([]byte(settings.GitUsername() + ":" + settings.Token()))
	scope := "http." + settings.BaseURL() + "/"
	entries := [][2]string{
		{scope + ".extraHeader", "Authorization: Basic " + credentials},
		{"credential.helper", ""},
	}
	if settings.InsecureTLS {
		entries = append(entries, [2]string{scope + ".sslVerify", "false"})
	}
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=" + strconv.Itoa(len(entries)),
	}
	for index, entry := range entries {
		env = append(env,
			fmt.Sprintf("GIT_CONFIG_KEY_%d=%s", index, entry[0]),
			fmt.Sprintf("GIT_CONFIG_VALUE_%d=%s", index, entry[1]),
		)
	}
	return env
}
