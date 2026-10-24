package code_hosting_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/git"
)

// runGit runs git in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
	return strings.TrimSpace(string(output))
}

// gitRepo creates a repository with one commit on main and returns its root.
func gitRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hola\n"), 0o600))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")
	return root
}

// recordingServer accepts one connection, answers 401 and sends the request headers it received
// through the returned channel. It is a plain TCP server because git speaks its own HTTP dialect.
func recordingServer(t *testing.T) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	received := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			received <- ""
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		var request bytes.Buffer
		chunk := make([]byte, 1024)
		for !bytes.Contains(request.Bytes(), []byte("\r\n\r\n")) {
			read, readErr := conn.Read(chunk)
			request.Write(chunk[:read])
			if readErr != nil {
				break
			}
		}
		// The client may already have given up; the request is what the test checks.
		_, _ = conn.Write([]byte("HTTP/1.1 401 Unauthorized\r\nWWW-Authenticate: Basic realm=\"git\"\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"))
		received <- request.String()
	}()
	return "http://" + listener.Addr().String(), received
}

func TestOriginURLReturnsTheConfiguredRemote(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "remote", "add", "origin", "git@git.corp:grupo/proyecto.git")

	url, err := git.NewRemoteAdapter().OriginURL(context.Background(), repo)

	require.NoError(t, err)
	require.Equal(t, "git@git.corp:grupo/proyecto.git", url)
}

func TestOriginURLFailsWithNoOriginWhenMissing(t *testing.T) {
	repo := gitRepo(t)

	_, err := git.NewRemoteAdapter().OriginURL(context.Background(), repo)

	require.ErrorIs(t, err, codeerrors.NoOrigin(""))
}

func TestOriginURLIgnoresOtherRemotes(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "remote", "add", "upstream", "https://x.corp/a/b.git")

	_, err := git.NewRemoteAdapter().OriginURL(context.Background(), repo)

	require.ErrorIs(t, err, codeerrors.NoOrigin(""))
}

func TestPublishBranchPushesToTheProviderURLAuthenticatingWithTheToken(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "remote", "add", "origin", "git@unreachable.invalid:acme/app.git")
	runGit(t, repo, "branch", "pi/feature")
	url, received := recordingServer(t)
	settings, err := models.NewCodeHostSettings(models.GitHub, url, "ghp_secret", false)
	require.NoError(t, err)

	err = git.NewRemoteAdapter().PublishBranch(context.Background(), repo, "pi/feature", settings, "acme/app")

	request := <-received
	expected := strings.ToLower("authorization: basic " + base64.StdEncoding.EncodeToString([]byte("x-access-token:ghp_secret")))
	require.True(t, strings.HasPrefix(request, "GET /acme/app.git/info/refs?service=git-receive-pack"), request)
	require.Contains(t, strings.ToLower(request), expected)
	// The server rejects the push: the failure is reported without leaking the token.
	require.ErrorIs(t, err, codeerrors.BranchPublishFailed("", ""))
	require.NotContains(t, err.Error(), "ghp_secret")
}

func TestPublishBranchAuthenticatesGitLabWithTheOAuth2User(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "branch", "pi/feature")
	url, received := recordingServer(t)
	settings, err := models.NewCodeHostSettings(models.GitLab, url, "glpat-secret", false)
	require.NoError(t, err)

	_ = git.NewRemoteAdapter().PublishBranch(context.Background(), repo, "pi/feature", settings, "grupo/proyecto")

	request := strings.ToLower(<-received)
	expected := strings.ToLower("authorization: basic " + base64.StdEncoding.EncodeToString([]byte("oauth2:glpat-secret")))
	require.Contains(t, request, expected)
}

func TestPublishBranchReportsFailureWhenTheServerIsUnreachable(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "branch", "pi/feature")
	settings, err := models.NewCodeHostSettings(models.GitLab, "http://127.0.0.1:1", "tok", false)
	require.NoError(t, err)

	err = git.NewRemoteAdapter().PublishBranch(context.Background(), repo, "pi/feature", settings, "grupo/proyecto")

	require.ErrorIs(t, err, codeerrors.BranchPublishFailed("", ""))
}

func TestEnsureCommitsAheadAddsAnEmptyCommitWhenTheBranchHasNone(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "branch", "pi/feature")
	treeBefore := runGit(t, repo, "rev-parse", "pi/feature^{tree}")

	created, err := git.NewRemoteAdapter().EnsureCommitsAhead(context.Background(), repo, "pi/feature", "HEAD")

	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, "1", runGit(t, repo, "rev-list", "--count", "HEAD..pi/feature"))
	require.Equal(t, treeBefore, runGit(t, repo, "rev-parse", "pi/feature^{tree}"))
}

func TestEnsureCommitsAheadLeavesABranchThatAlreadyAdvances(t *testing.T) {
	repo := gitRepo(t)
	runGit(t, repo, "checkout", "-q", "-b", "pi/feature")
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "work")
	headBefore := runGit(t, repo, "rev-parse", "pi/feature")

	created, err := git.NewRemoteAdapter().EnsureCommitsAhead(context.Background(), repo, "pi/feature", "HEAD~1")

	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, headBefore, runGit(t, repo, "rev-parse", "pi/feature"))
}
