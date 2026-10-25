package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// startFixture wires StartTask over the fakes of this package and keeps the fakes for the assertions.
type startFixture struct {
	repo        *fakeRepository
	lifecycle   *application.TaskLifecycle
	publisher   *fakePublisher
	worktrees   *fakeWorktree
	runtime     *resumeRuntime
	settings    *fakeLaunchSettings
	attachments *fakeAttachments
	storage     *fakeStorage
}

func newStartFixture(tasks ...models.Task) *startFixture {
	repo := newFakeRepository(tasks...)
	lifecycle, publisher := newLifecycle(repo)
	return &startFixture{
		repo:        repo,
		lifecycle:   lifecycle,
		publisher:   publisher,
		worktrees:   &fakeWorktree{exists: true},
		runtime:     newResumeRuntime(),
		settings:    &fakeLaunchSettings{},
		attachments: &fakeAttachments{},
		storage:     &fakeStorage{},
	}
}

// start builds the use case with the given identifier generator; newUUID is the default one.
func (f *startFixture) start(generate application.UUIDGenerator) *application.StartTask {
	return application.NewStartTask(f.lifecycle, f.worktrees, f.runtime, f.settings, &resumeSink{}, f.attachments, f.storage, generate, clock)
}

// fixedUUID returns the same identifier every time, so branch and folder names are predictable.
func fixedUUID(value uuid.UUID) application.UUIDGenerator {
	return func() (uuid.UUID, error) { return value, nil }
}

// limitOf returns a pointer to a limit value, for the optional ResourceLimits fields.
func limitOf(value uint32) *uint32 {
	return &value
}

// stored builds the metadata of one image the initial prompt carries.
func stored(name string) models.StoredAttachment {
	return models.StoredAttachment{ID: "id-" + name, Name: name, MimeType: "image/png", Path: "t/" + name, SizeBytes: 3}
}

// promptOf returns the single command the runtime received, which must be a prompt.
func promptOf(t *testing.T, sent []sentCommand) models.PromptCommand {
	t.Helper()
	require.Len(t, sent, 1)
	prompt, ok := sent[0].command.(models.PromptCommand)
	require.True(t, ok, "the initial message must be a prompt")
	return prompt
}

// Rust: start_task::creates_the_worktree_from_the_base_branch_chosen_for_the_task
func TestStartTaskCreatesTheWorktreeFromTheBaseBranchChosenForTheTask(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	task.BaseBranch = "origin/develop"
	fx := newStartFixture(task)

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, "origin/develop", started.BaseBranch)
	require.Len(t, fx.worktrees.created, 1)
	assert.Equal(t, "origin/develop", fx.worktrees.created[0].base)
}

// Rust: start_task::creates_worktree_starts_agent_and_sends_prompt
func TestStartTaskCreatesWorktreeStartsAgentAndSendsPrompt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Empty(t, fx.repo.statuses, "a successful start records no status transition")
	require.Len(t, fx.worktrees.created, 1)
	assert.Equal(t, "/repos/demo", fx.worktrees.created[0].repo)
	assert.Equal(t, "main", fx.worktrees.created[0].base)
	assert.Contains(t, fx.worktrees.created[0].branch, "pi/arregla-el-login-")
	require.Len(t, fx.runtime.started, 1)
	assert.True(t, strings.HasPrefix(fx.runtime.started[0].cwd, "/wt/demo-ws-1/"), fx.runtime.started[0].cwd)
	assert.False(t, fx.runtime.started[0].resume)
	prompt := promptOf(t, fx.runtime.sent)
	assert.Equal(t, "Arregla el login", prompt.Message)
	assert.Empty(t, prompt.Images)
}

// Rust: start_task::reads_stored_files_and_sends_images_with_the_initial_prompt
func TestStartTaskReadsStoredFilesAndSendsImagesWithTheInitialPrompt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.attachments.listed = []models.StoredAttachment{stored("a.png"), stored("b.png")}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	prompt := promptOf(t, fx.runtime.sent)
	assert.Equal(t, []models.ImageAttachment{
		{Name: "a.png", MimeType: "image/png", Data: "data-a.png"},
		{Name: "b.png", MimeType: "image/png", Data: "data-b.png"},
	}, prompt.Images)
}

// Rust: start_task::marks_task_failed_without_starting_agent_when_images_cannot_be_loaded
func TestStartTaskMarksTaskFailedWithoutStartingAgentWhenImagesCannotBeLoaded(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.attachments.listErr = werrors.Repository("adjuntos ilegibles")

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	assert.Empty(t, fx.runtime.started, "the agent must not start without its prompt")
	require.NotNil(t, fx.repo.errorTexts[len(fx.repo.errorTexts)-1])
	assert.Contains(t, *fx.repo.errorTexts[len(fx.repo.errorTexts)-1], "adjuntos ilegibles")
}

// Rust: start_task::marks_task_failed_without_starting_agent_when_an_image_file_is_missing
func TestStartTaskMarksTaskFailedWithoutStartingAgentWhenAnImageFileIsMissing(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.attachments.listed = []models.StoredAttachment{stored("a.png")}
	fx.storage.readErr = errors.New("no existe")

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	assert.Empty(t, fx.runtime.started)
	require.NotNil(t, fx.repo.errorTexts[len(fx.repo.errorTexts)-1])
	errorText := *fx.repo.errorTexts[len(fx.repo.errorTexts)-1]
	assert.Contains(t, errorText, "a.png")
	assert.Contains(t, errorText, "no existe")
}

// Rust: start_task::marks_task_failed_when_agent_cannot_start
func TestStartTaskMarksTaskFailedWhenAgentCannotStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.runtime.startErrs[task.ID] = werrors.Runtime("no existe gentle-shell")

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	assert.Equal(t, []models.TaskStatus{models.StatusFailed}, fx.repo.statuses)
	require.NotNil(t, fx.repo.errorTexts[0])
	assert.Contains(t, *fx.repo.errorTexts[0], "gentle-shell")
	assert.Empty(t, fx.runtime.stopped, "nothing runs when the start failed, so there is nothing to stop")
}

// Rust: start_task::worktree_creation_failure_leaves_task_in_backlog
func TestStartTaskWorktreeCreationFailureLeavesTaskInBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.worktrees.createErr = werrors.Git("no space left")

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.Git("no space left"))
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, 1, fx.repo.reverts)
	assert.Equal(t, models.StatusBacklog, fx.repo.tasks[task.ID].Status)
}

// Rust: start_task::returns_create_error_and_marks_failed_when_revert_also_fails
func TestStartTaskReturnsCreateErrorAndMarksFailedWhenRevertAlsoFails(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.worktrees.createErr = werrors.Git("no space left")
	fx.repo.revertErr = werrors.Repository("db bloqueada")

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.Git("no space left"))
	assert.Equal(t, []models.TaskStatus{models.StatusFailed}, fx.repo.statuses)
	require.NotNil(t, fx.repo.errorTexts[0])
	assert.Contains(t, *fx.repo.errorTexts[0], "no space left")
}

// Rust: start_task::stops_runtime_when_prompt_send_fails_after_start
func TestStartTaskStopsRuntimeWhenPromptSendFailsAfterStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.runtime.sendErr = werrors.Runtime("pipe roto")

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	assert.Equal(t, []models.TaskStatus{models.StatusFailed}, fx.repo.statuses)
	assert.Equal(t, []models.TaskID{task.ID}, fx.runtime.stopped)
}

// Rust: start_task::rejects_when_claim_is_lost_to_a_concurrent_start
func TestStartTaskRejectsWhenClaimIsLostToAConcurrentStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	lost := false
	fx.repo.claimResult = &lost

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.InvalidState("iniciar", "backlog"))
	assert.Empty(t, fx.repo.statuses)
	assert.Empty(t, fx.worktrees.created)
}

// Rust: start_task::rejects_task_not_in_backlog
func TestStartTaskRejectsTaskNotInBacklog(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	fx := newStartFixture(task)

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.InvalidState("iniciar", "running"))
	assert.Empty(t, fx.runtime.started)
}

// Rust: start_task::passes_prepared_model_to_runtime_start
func TestStartTaskPassesPreparedModelToRuntimeStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.settings.options = models.LaunchOptions{Model: &models.LaunchModel{Provider: "anthropic", Model: "claude-5"}}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.runtime.started, 1)
	require.NotNil(t, fx.runtime.started[0].options.Model)
	assert.Equal(t, models.LaunchModel{Provider: "anthropic", Model: "claude-5"}, *fx.runtime.started[0].options.Model)
}

// Rust: start_task::passes_prepared_thinking_level_to_runtime_start
func TestStartTaskPassesPreparedThinkingLevelToRuntimeStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	high := "high"
	fx.settings.options = models.LaunchOptions{Thinking: &high}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.runtime.started, 1)
	require.NotNil(t, fx.runtime.started[0].options.Thinking)
	assert.Equal(t, "high", *fx.runtime.started[0].options.Thinking)
}

// Rust: start_task::task_limits_override_global_limits_per_resource_on_start
func TestStartTaskTaskLimitsOverrideGlobalLimitsPerResourceOnStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	task.Limits = models.ResourceLimits{MemoryMB: limitOf(1024)}
	fx := newStartFixture(task)
	fx.settings.options = models.LaunchOptions{
		Limits: models.ResourceLimits{MemoryMB: limitOf(4096), CPUPercent: limitOf(200)},
	}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.runtime.started, 1)
	assert.Equal(t, models.ResourceLimits{MemoryMB: limitOf(1024), CPUPercent: limitOf(200)}, fx.runtime.started[0].options.Limits)
}

// Rust: start_task::strips_model_and_thinking_the_runtime_does_not_support
func TestStartTaskStripsModelAndThinkingTheRuntimeDoesNotSupport(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	high := "high"
	fx.runtime.capabilities = models.MinimalAgentCapabilities()
	fx.settings.options = models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
		Thinking: &high,
	}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.runtime.started, 1)
	assert.Nil(t, fx.runtime.started[0].options.Model)
	assert.Nil(t, fx.runtime.started[0].options.Thinking)
}

// Rust: start_task::keeps_only_the_option_the_runtime_supports
func TestStartTaskKeepsOnlyTheOptionTheRuntimeSupports(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	high := "high"
	fx.runtime.capabilities = models.AgentCapabilities{ModelSelection: true}
	fx.settings.options = models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
		Thinking: &high,
	}

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	require.Len(t, fx.runtime.started, 1)
	assert.NotNil(t, fx.runtime.started[0].options.Model)
	assert.Nil(t, fx.runtime.started[0].options.Thinking)
}

// Rust: start_task::marks_task_failed_when_launch_settings_prepare_fails
func TestStartTaskMarksTaskFailedWhenLaunchSettingsPrepareFails(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.settings.err = werrors.Runtime("pin de perfil inválido")

	started, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	assert.Equal(t, []models.TaskStatus{models.StatusFailed}, fx.repo.statuses)
	require.NotNil(t, fx.repo.errorTexts[0])
	assert.Contains(t, *fx.repo.errorTexts[0], "pin de perfil")
}

// Rule from source: start_task.rs treats a blank base branch as "not chosen" and falls back to the workspace
// base branch. Rust has no test for the blank case, so the raw blank value must never reach the worktree.
func TestStartTaskBlankBaseBranchFallsBackToTheWorkspaceBaseBranch(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	task.BaseBranch = "   "
	fx := newStartFixture(task)

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.worktrees.created, 1)
	assert.Equal(t, "main", fx.worktrees.created[0].base)
}

// Rule from source: a launch-settings failure happens before the agent is asked to start, so no process is started
// and nothing has to be stopped. Rust only checks the failed status.
func TestStartTaskPrepareFailureNeverStartsTheAgent(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.settings.err = werrors.Runtime("pin de perfil inválido")

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.runtime.sent)
	assert.Empty(t, fx.runtime.stopped)
}

// Rule from source: a start that lost the claim owns nothing, so it must not create a worktree nor revert a claim.
func TestStartTaskClaimConflictCreatesNoWorktreeAndRevertsNothing(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	lost := false
	fx.repo.claimResult = &lost

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.Error(t, err)
	assert.Empty(t, fx.worktrees.created)
	assert.Equal(t, 0, fx.repo.reverts)
}

// Rule from source: branch and worktree folder carry the same unique identifier, so they can be matched by name.
func TestStartTaskSharesTheUniqueSuffixBetweenBranchAndFolder(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	unique := uuid.MustParse("0b9f3c2e-1d4a-4e5b-8c6d-7e8f9a0b1c2d")

	_, err := fx.start(fixedUUID(unique)).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.worktrees.created, 1)
	assert.Equal(t, models.BranchName(task.Title, unique), fx.worktrees.created[0].branch)
	assert.Contains(t, fx.worktrees.created[0].path, models.WorktreeDirName(task.Title, unique))
}

// Rule from source: a missing workspace is detected before the task is claimed, so the task keeps its backlog state.
func TestStartTaskMissingWorkspaceFailsBeforeClaiming(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	task.WorkspaceID = "ws-missing"
	fx := newStartFixture(task)

	_, err := fx.start(newUUID).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.WorkspaceNotFound("ws-missing"))
	assert.Equal(t, 0, fx.repo.claims)
	assert.Equal(t, models.StatusBacklog, fx.repo.tasks[task.ID].Status)
}
