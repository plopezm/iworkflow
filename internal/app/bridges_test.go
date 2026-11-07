package app

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/listen"
	"iworkflow/internal/core/restart"
	codehostinginbound "iworkflow/internal/modules/code_hosting/domain/ports/inbound"
	hooksapp "iworkflow/internal/modules/task_hooks/application"
	hooksmodels "iworkflow/internal/modules/task_hooks/domain/models"
	taskserrors "iworkflow/internal/modules/tasks/domain/errors"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacesdi "iworkflow/internal/modules/workspaces/infrastructure/di"
)

// noWorkspaceTasks is the workspace cascade of the tests that compose only the workspaces module: no task exists.
type noWorkspaceTasks struct{}

func (noWorkspaceTasks) DeleteAll(context.Context, workspacesmodels.WorkspaceID) error { return nil }

// newWorkspaces builds the workspaces module over a migrated temporary database, with no workspace.
func newWorkspaces(t *testing.T) *workspacesdi.Module {
	t.Helper()
	return workspacesdi.NewModule(workspacesdi.Deps{
		DB:      dbtest.New(t),
		HomeDir: t.TempDir(),
		Tasks:   noWorkspaceTasks{},
	})
}

// composeWithTasks composes the whole server over a migrated temporary database, so that the tasks bridges read the
// real tasks module. No agent is started.
func composeWithTasks(t *testing.T) *App {
	t.Helper()
	return composeOn(t, dbtest.New(t))
}

// composeOn composes the whole server over the database, so a test can seed rows before the composition.
func composeOn(t *testing.T, db *sql.DB) *App {
	t.Helper()
	application, err := New(Deps{
		DB:        db,
		Config:    NewTestConfig(t),
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		StaticDir: t.TempDir(),
		Listen:    listen.Info{Addr: netip.AddrPortFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 3000)},
		Restart:   restart.NewHandle(),
	})
	require.NoError(t, err)
	return application
}

func TestWorkspaceRootsReturnNilForAnUnknownWorkspace(t *testing.T) {
	roots := &workspaceRoots{get: newWorkspaces(t).Get}

	root, err := roots.Find(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Nil(t, root)
}

func TestWorkspaceTargetsReturnNilForAnUnknownWorkspace(t *testing.T) {
	resolve := workspaceTargets(newWorkspaces(t).Get)

	target, err := resolve(context.Background(), uuid.New())

	require.NoError(t, err)
	require.Nil(t, target)
}

// The tasks lookups find nothing for an identifier no task has, as the Rust resolvers do: the ports answer nil and no
// error, so the screens show the task as missing instead of failing.
func TestTaskLookupsFindNothingForAnUnknownTask(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)

	root, err := (&taskRootLookup{get: application.tasks.Get}).Find(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, root)

	target, err := taskTargets(application.tasks.Get)(ctx, uuid.New())
	require.NoError(t, err)
	require.Nil(t, target)

	source, err := (&taskSourceLookup{get: application.tasks.Get, workspaces: newWorkspaces(t).Get}).Find(ctx, uuid.New().String())
	require.NoError(t, err)
	require.Nil(t, source)
}

// A task identifier that is not a UUID is unknown too, not an error: the merge request panel passes raw text.
func TestTaskSourceLookupTreatsAMalformedIdentifierAsUnknown(t *testing.T) {
	application := composeWithTasks(t)
	source := &taskSourceLookup{get: application.tasks.Get, workspaces: newWorkspaces(t).Get}

	found, err := source.Find(context.Background(), "not-a-uuid")

	require.NoError(t, err)
	assert.Nil(t, found)
}

// The workspace lookup of the tasks module answers nil for an unknown or malformed identifier, and lists nothing
// when there is no workspace.
func TestTaskWorkspaceLookupFindsNothingWithoutWorkspaces(t *testing.T) {
	ctx := context.Background()
	workspaces := newWorkspaces(t)
	lookup := &taskWorkspaceLookup{get: workspaces.Get, list: workspaces.List}

	found, err := lookup.Find(ctx, uuid.New().String())
	require.NoError(t, err)
	assert.Nil(t, found)

	malformed, err := lookup.Find(ctx, "not-a-uuid")
	require.NoError(t, err)
	assert.Nil(t, malformed)

	all, err := lookup.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, all)
}

// Deleting the tasks of a workspace that has none succeeds, so the workspace delete is not blocked by the cascade.
func TestWorkspaceTasksBridgeDeletesNothingForAWorkspaceWithoutTasks(t *testing.T) {
	application := composeWithTasks(t)
	bridge := &workspaceTasksBridge{deleteAll: application.tasks.DeleteWorkspace}

	err := bridge.DeleteAll(context.Background(), workspacesmodels.WorkspaceIDFromUUID(uuid.New()))

	require.NoError(t, err)
}

// registerRepository registers a real git repository as a workspace of the composed server.
func registerRepository(t *testing.T, application *App) workspacesmodels.Workspace {
	t.Helper()
	ctx := context.Background()
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "-c", "user.name=iworkflow", "-c", "user.email=iworkflow@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	workspace, err := application.workspaces.Register.Execute(ctx, workspacesmodels.NewWorkspace{Name: "Proyecto", Path: repo})
	require.NoError(t, err)
	return workspace
}

// runGit runs git in dir and fails the test when it does not succeed.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// createTask creates a task through the composed tasks module. Its row is in backlog, as Create leaves it.
func createTask(t *testing.T, application *App, workspace workspacesmodels.Workspace) tasksmodels.Task {
	t.Helper()
	created, err := application.tasks.Create.Execute(context.Background(), tasksmodels.NewTask{
		WorkspaceID: workspace.ID.String(),
		Title:       "Arregla el login",
		Prompt:      "Revisa el formulario\nmás detalles",
	})
	require.NoError(t, err)
	return created.Task
}

// startTask moves the row of a task created through the composed module to the state a started task has: the branch
// and the worktree are claimed in the module's own repository. No agent runs.
func startTask(t *testing.T, application *App, task tasksmodels.Task) tasksmodels.Task {
	t.Helper()
	ctx := context.Background()
	claimed, err := application.tasks.Lifecycle.Repository().ClaimForStart(ctx, task.ID, "iwf/arregla-login", filepath.Join(t.TempDir(), "arregla-login"), "main")
	require.NoError(t, err)
	require.True(t, claimed)
	started, err := application.tasks.Get.Execute(ctx, task.ID)
	require.NoError(t, err)
	return started
}

// The lookups map a started task row to the fields the modules read: the worktree as the root and the target, and the
// repository, branch and prompt as the source with the workspace it belongs to.
func TestTaskLookupsMapAStartedTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	task := startTask(t, application, createTask(t, application, workspace))
	id, err := uuid.Parse(task.ID.String())
	require.NoError(t, err)

	root, err := (&taskRootLookup{get: application.tasks.Get}).Find(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, root)
	assert.Equal(t, task.Title, root.Name)
	assert.Equal(t, task.WorktreePath, root.Path)

	target, err := taskTargets(application.tasks.Get)(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.Equal(t, workspaceUUID(t, workspace), target.WorkspaceID)
	assert.Equal(t, task.Title, target.Name)
	assert.Equal(t, task.WorktreePath, target.Path)

	source, err := (&taskSourceLookup{get: application.tasks.Get, workspaces: application.workspaces.Get}).Find(ctx, task.ID.String())
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, workspace.Path, source.RepoPath, "the source is the repository of the workspace")
	assert.Equal(t, "iwf/arregla-login", source.Branch)
	assert.Equal(t, "main", source.BaseBranch)
	assert.Equal(t, task.Title, source.Title)
	assert.Equal(t, task.Prompt, source.Description)
}

// A task still in backlog has no branch and no worktree yet, so none of the lookups finds it.
func TestTaskLookupsFindNothingForABacklogTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	task := createTask(t, application, workspace)
	id, err := uuid.Parse(task.ID.String())
	require.NoError(t, err)

	root, err := (&taskRootLookup{get: application.tasks.Get}).Find(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, root)

	target, err := taskTargets(application.tasks.Get)(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, target)

	source, err := (&taskSourceLookup{get: application.tasks.Get, workspaces: application.workspaces.Get}).Find(ctx, task.ID.String())
	require.NoError(t, err)
	assert.Nil(t, source)
}

func workspaceUUID(t *testing.T, workspace workspacesmodels.Workspace) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(workspace.ID.String())
	require.NoError(t, err)
	return id
}

// fakeRemoteMerge is the code_hosting remote merge, with the outcome or the error the test chooses.
type fakeRemoteMerge struct {
	outcome codehostinginbound.RemoteMergeOutcome
	err     error
}

func (f fakeRemoteMerge) MergeOpenRequest(context.Context, string) (codehostinginbound.RemoteMergeOutcome, error) {
	return f.outcome, f.err
}

func (f fakeRemoteMerge) PreviewMergeOpenRequest(context.Context, string) (codehostinginbound.RemoteMergeOutcome, error) {
	return f.outcome, f.err
}

// Every outcome of the code_hosting merge maps to the tasks outcome of the same meaning; an outcome the tasks module
// does not know is an error, and the error of the merge passes through.
func TestRemoteMergeBridgeMapsEveryOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		outcome codehostinginbound.RemoteMergeOutcome
		want    tasksoutbound.RemoteMergeOutcome
	}{
		{name: "merged on the provider", outcome: codehostinginbound.RemoteMergeMerged, want: tasksoutbound.RemoteMerged},
		{name: "no open request", outcome: codehostinginbound.RemoteMergeNoOpenRequest, want: tasksoutbound.RemoteNoOpenRequest},
		{name: "already merged on the provider", outcome: codehostinginbound.RemoteMergeAlreadyMerged, want: tasksoutbound.RemoteAlreadyMerged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bridge := &remoteMergeBridge{inner: fakeRemoteMerge{outcome: tc.outcome}}

			got, err := bridge.MergeOpenRequest(context.Background(), "task")

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("unknown outcome", func(t *testing.T) {
		bridge := &remoteMergeBridge{inner: fakeRemoteMerge{outcome: 99}}

		_, err := bridge.MergeOpenRequest(context.Background(), "task")

		require.ErrorContains(t, err, "unknown remote merge outcome")
	})

	t.Run("merge error passes through", func(t *testing.T) {
		failure := errors.New("provider unreachable")
		bridge := &remoteMergeBridge{inner: fakeRemoteMerge{err: failure}}

		_, err := bridge.MergeOpenRequest(context.Background(), "task")

		require.ErrorIs(t, err, failure)
	})
}

func TestTaskFileSearchFindsNothingUntilItIsBound(t *testing.T) {
	t.Parallel()
	bridge := &taskFileSearch{}

	id, err := tasksmodels.NewTaskID()
	require.NoError(t, err)

	files, err := bridge.Files(context.Background(), id, "a")

	require.NoError(t, err)
	assert.Empty(t, files)

	wsFiles, err := bridge.WorkspaceFiles(context.Background(), uuid.New().String(), "a")
	require.NoError(t, err)
	assert.Empty(t, wsFiles)
}

func TestTaskFileSearchWorkspaceFilesWithInvalidOrEmptyUUIDReturnsNil(t *testing.T) {
	t.Parallel()
	bridge := &taskFileSearch{}

	files, err := bridge.WorkspaceFiles(context.Background(), "", "a")
	require.NoError(t, err)
	assert.Empty(t, files)

	files, err = bridge.WorkspaceFiles(context.Background(), "invalid-uuid", "a")
	require.NoError(t, err)
	assert.Empty(t, files)
}

// A blocking pre hook stops the start through the real hooks module: the script runs in the worktree, the task ends
// failed with the hook named, and the run record is kept for the status strip of the task.
func TestStartTaskRunsABlockingPreHookThroughTheRealHooks(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	_, err := application.taskHooks.SaveHook.Execute(ctx, hooksapp.SaveHookInput{
		WorkspaceID: workspace.ID.String(), Phase: hooksmodels.PhasePre, Event: hooksmodels.EventTaskStart,
		Kind: hooksmodels.KindScript, Enabled: true, OnFailure: hooksmodels.FailureBlock,
		Timeout: time.Minute, Script: hooksmodels.ScriptSpec{Command: "false"},
	})
	require.NoError(t, err)
	task := createTask(t, application, workspace)

	started, err := application.tasks.Start.Execute(ctx, task.ID)

	require.NoError(t, err)
	assert.Equal(t, tasksmodels.StatusFailed, started.Status)
	require.NotNil(t, started.Error)
	assert.Contains(t, *started.Error, "un hook previo al inicio falló")
	assert.Contains(t, *started.Error, "a hook blocked the action")
	states, err := application.taskHooks.ListTaskHookState.Execute(ctx, workspace.ID.String(), task.ID.String())
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.NotNil(t, states[0].LastRun)
	assert.Equal(t, hooksmodels.RunFailed, states[0].LastRun.Status)
}

// idleTaskInWorktree moves a task created through the composed module to idle, the state of a task whose agent is
// running or was stopped. Its worktree is a real git repository, so the finish can commit the changes.
func idleTaskInWorktree(t *testing.T, application *App, task tasksmodels.Task) tasksmodels.Task {
	t.Helper()
	ctx := context.Background()
	worktree := t.TempDir()
	runGit(t, worktree, "init", "-q", "-b", "iwf/arregla-login")
	runGit(t, worktree, "-c", "user.name=iworkflow", "-c", "user.email=iworkflow@example.com", "commit", "-q", "--allow-empty", "-m", "init")
	repository := application.tasks.Lifecycle.Repository()
	claimed, err := repository.ClaimForStart(ctx, task.ID, "iwf/arregla-login", worktree, "main")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, repository.UpdateStatus(ctx, task.ID, tasksmodels.StatusIdle, nil))
	idle, err := application.tasks.Get.Execute(ctx, task.ID)
	require.NoError(t, err)
	return idle
}

// A blocking pre finish hook stops the finish through the real hooks module: Finish returns ErrHookBlocked and the task
// keeps its status, so nothing was stopped or committed.
func TestFinishTaskRunsABlockingPreHookThroughTheRealHooks(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	_, err := application.taskHooks.SaveHook.Execute(ctx, hooksapp.SaveHookInput{
		WorkspaceID: workspace.ID.String(), Phase: hooksmodels.PhasePre, Event: hooksmodels.EventTaskFinish,
		Kind: hooksmodels.KindScript, Enabled: true, OnFailure: hooksmodels.FailureBlock,
		Timeout: time.Minute, Script: hooksmodels.ScriptSpec{Command: "false"},
	})
	require.NoError(t, err)
	task := idleTaskInWorktree(t, application, createTask(t, application, workspace))

	err = application.tasks.Finish.Execute(ctx, task.ID)

	require.ErrorIs(t, err, taskserrors.ErrHookBlocked)
	kept, err := application.tasks.Get.Execute(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, tasksmodels.StatusIdle, kept.Status)
}

// A post finish hook runs after the task reached awaiting review: the script records the status it sees, and the finish
// returns once the script ran.
func TestFinishTaskRunsAPostHookAfterTheTransitionThroughTheRealHooks(t *testing.T) {
	ctx := context.Background()
	application := composeWithTasks(t)
	workspace := registerRepository(t, application)
	marker := filepath.Join(t.TempDir(), "post-finish-status")
	_, err := application.taskHooks.SaveHook.Execute(ctx, hooksapp.SaveHookInput{
		WorkspaceID: workspace.ID.String(), Phase: hooksmodels.PhasePost, Event: hooksmodels.EventTaskFinish,
		Kind: hooksmodels.KindScript, Enabled: true, OnFailure: hooksmodels.FailureWarn,
		Timeout: time.Minute,
		Script:  hooksmodels.ScriptSpec{Command: "sh", Args: []string{"-c", `printf '%s' "$IWF_TASK_STATUS" > "$0"`, marker}},
	})
	require.NoError(t, err)
	task := idleTaskInWorktree(t, application, createTask(t, application, workspace))

	require.NoError(t, application.tasks.Finish.Execute(ctx, task.ID))

	finished, err := application.tasks.Get.Execute(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, tasksmodels.StatusAwaitingReview, finished.Status)
	written, err := os.ReadFile(marker)
	require.NoError(t, err, "the post hook ran before Finish returned")
	assert.Equal(t, string(tasksmodels.StatusAwaitingReview), string(written), "the post hook saw the status after the transition")
}

// Review advisory (g): a run left running by a process that died is closed as an error when the server composes,
// before it serves, so the status strip never shows it as running forever.
func TestComposeClosesRunsLeftRunningByAPreviousProcess(t *testing.T) {
	db := dbtest.New(t)
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES ('t1', 'w1', 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO task_hooks (id, workspace_id, phase, event, kind, position, enabled, on_failure, timeout_seconds)
		VALUES ('h1', 'w1', 'pre', 'task_start', 'script', 1, 1, 'warn', 30)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO task_hook_runs (id, task_id, hook_id, phase, event, status, exit_code, output, started_at, finished_at)
		VALUES ('orphan', 't1', 'h1', 'pre', 'task_start', 'running', 0, '', '2026-01-01T00:00:00.000000000Z', '')`)
	require.NoError(t, err)

	composeOn(t, db)

	var status, output string
	require.NoError(t, db.QueryRow(`SELECT status, output FROM task_hook_runs WHERE id = 'orphan'`).Scan(&status, &output))
	assert.Equal(t, "error", status)
	assert.Equal(t, hooksmodels.StaleRunOutput, output)
}
