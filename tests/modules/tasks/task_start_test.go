package tasks_test

import (
	"context"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// claudeSettings launches every task on the claude runtime.
type claudeSettings struct{}

func (claudeSettings) Prepare(context.Context, string, models.TaskAgentOptions) (models.LaunchOptions, error) {
	runtime, err := models.ParseRuntimeID("claude")
	if err != nil {
		return models.LaunchOptions{}, err //nolint:wrapcheck // test double
	}
	return models.LaunchOptions{Runtime: &runtime}, nil
}

var _ outbound.LaunchSettings = claudeSettings{}

// gitWorkspace is a repository with one commit on main, the base branch the worktree of a task starts from.
func gitWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		command := exec.CommandContext(context.Background(), "git", append([]string{"-C", dir}, args...)...)
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
	}
	return dir
}

// startRouter wires the module with the fake claude agent of the e2e suite, so a start really launches a process.
func startRouter(t *testing.T, repo *Repo) chi.Router {
	t.Helper()
	fake, err := filepath.Abs("../../fixtures/smoke_claude.sh")
	require.NoError(t, err)
	binaries := testBinaries
	binaries.Claude = fake
	module, err := di.NewModule(di.Deps{
		DB:             repo.DB,
		Workspaces:     listingLookup{workspaceLookup{repo: repo}},
		AttachmentsDir: t.TempDir(),
		WorktreesDir:   t.TempDir(),
		Binaries:       binaries,
		LaunchSettings: claudeSettings{},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		module.StopAgents(ctx)
		module.WaitWorktreeCleanup(ctx)
		module.ShutdownEvents()
	})
	router := chi.NewRouter()
	module.Routes(router)
	return router
}

func TestStartFromTheCardLaunchesTheAgentAndTheTaskLeavesBacklog(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("the fake agent needs python3")
	}
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, gitWorkspace(t))
	backlog := newTask(t, workspaceID, "startable", models.StatusBacklog)
	insertTask(t, repo, backlog)
	router := startRouter(t, repo)

	recorder, body := doRequest(t, router, http.MethodPost, "/board/tasks/"+backlog.ID.String()+"/start", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "board:refresh", recorder.Header().Get("HX-Trigger"), "the list refreshes so the card moves")
	assert.Contains(t, body, `id="board-notice" hx-swap-oob="true"`)
	assert.NotContains(t, body, `text-error`, "a started task shows no failure")
	assert.NotEqual(t, string(models.StatusBacklog), statusOf(t, repo, backlog.ID), "the task left backlog")
	assert.NotEqual(t, string(models.StatusFailed), statusOf(t, repo, backlog.ID), "the agent started")
}
