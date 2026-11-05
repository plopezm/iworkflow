package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

var (
	relatedAPI = uuid.NewString()
	relatedWeb = uuid.NewString()
)

func uuidWorkspaces() knownWorkspaces {
	return knownWorkspaces{
		ownWorkspace: {ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main"},
		relatedAPI:   {ID: relatedAPI, Name: "api", Path: "/repos/api", BaseBranch: "main"},
		relatedWeb:   {ID: relatedWeb, Name: "web", Path: "/repos/web", BaseBranch: "main"},
	}
}

// fakeRestarter records the restarts the related projects use case asks for and answers with outcome or err.
type fakeRestarter struct {
	ids      []models.TaskID
	sections []string
	outcome  application.RestartOutcome
	err      error
}

func (f *fakeRestarter) Request(_ context.Context, id models.TaskID, section string) (application.RestartOutcome, error) {
	f.ids = append(f.ids, id)
	f.sections = append(f.sections, section)
	return f.outcome, f.err
}

type setRelatedFixture struct {
	task      models.Task
	related   *fakeRelatedProjects
	restarter *fakeRestarter
	useCase   *application.SetTaskRelatedProjects
}

func newSetRelated(status models.TaskStatus) setRelatedFixture {
	task := newTask(status)
	known := uuidWorkspaces()
	repo := newFakeRepository(task)
	lifecycle := application.NewTaskLifecycle(repo, &fakePublisher{}, known, clock)
	related := newFakeRelatedProjects()
	restarter := &fakeRestarter{}
	probe := &fakeRelatedProbe{}
	useCase := application.NewSetTaskRelatedProjects(lifecycle, related, application.NewListTaskRelatedProjects(related, known), probe, restarter)
	return setRelatedFixture{task: task, related: related, restarter: restarter, useCase: useCase}
}

func TestSetTaskRelatedProjectsStoresTheListAndHandsTheNoticeOfTheAddedOnesToTheRestart(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusIdle)
	f.related.stored[f.task.ID.String()] = []string{relatedAPI}
	f.restarter.outcome = application.RestartOutcome{Restarted: true}

	result, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedAPI, relatedWeb})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{Restarted: true}, result)
	assert.Equal(t, []string{relatedAPI, relatedWeb}, f.related.stored[f.task.ID.String()])
	require.Len(t, f.restarter.ids, 1)
	assert.Equal(t, f.task.ID, f.restarter.ids[0])
	require.Len(t, f.restarter.sections, 1)
	assert.Contains(t, f.restarter.sections[0], "/repos/web")
	assert.NotContains(t, f.restarter.sections[0], "/repos/api", "only the projects added now are announced")
}

func TestSetTaskRelatedProjectsReportsARestartThatWaitsForTheAgent(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusRunning)
	f.restarter.outcome = application.RestartOutcome{Pending: true}

	result, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedAPI})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{RestartPending: true}, result)
	require.Len(t, f.restarter.ids, 1, "a running agent is handed the notice; the restarter waits for it to be idle")
}

func TestSetTaskRelatedProjectsRequestsNothingWhenNothingWasAdded(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusIdle)
	f.related.stored[f.task.ID.String()] = []string{relatedAPI, relatedWeb}

	result, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedWeb})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{}, result)
	assert.Empty(t, f.restarter.ids)
	assert.Equal(t, []string{relatedWeb}, f.related.stored[f.task.ID.String()])
}

func TestSetTaskRelatedProjectsStillSavesWhenTheRestartFails(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusRunning)
	f.restarter.err = errors.New("restart refused")

	result, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedAPI})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{}, result)
	assert.Equal(t, []string{relatedAPI}, f.related.stored[f.task.ID.String()])
}

func TestSetTaskRelatedProjectsKeepsABackloggedTaskQuiet(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusBacklog)

	result, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedAPI})

	require.NoError(t, err)
	assert.Equal(t, application.SetRelatedProjectsResult{}, result)
	assert.Empty(t, f.restarter.ids)
	assert.Equal(t, []string{relatedAPI}, f.related.stored[f.task.ID.String()])
}

func TestSetTaskRelatedProjectsRefusesAClosedTaskAndStoresNothing(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusMerged)

	_, err := f.useCase.Execute(context.Background(), f.task.ID, []string{relatedAPI})

	require.ErrorIs(t, err, werrors.InvalidState("relacionar proyectos", "merged"))
	assert.Zero(t, f.related.replaceCalls)
}

func TestSetTaskRelatedProjectsRefusesInvalidAndUnknownProjectsAndStoresNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ids  []string
		want error
	}{
		{name: "not a uuid", ids: []string{"no-es-uuid"}, want: werrors.InvalidRelatedProject("no-es-uuid")},
		{name: "unknown workspace", ids: []string{"00000000-0000-4000-8000-000000000000"}, want: werrors.WorkspaceNotFound("00000000-0000-4000-8000-000000000000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newSetRelated(models.StatusIdle)

			_, err := f.useCase.Execute(context.Background(), f.task.ID, tt.ids)

			require.ErrorIs(t, err, tt.want)
			assert.Zero(t, f.related.replaceCalls)
			assert.Empty(t, f.restarter.ids)
		})
	}
}

func TestSetTaskRelatedProjectsDropsTheOwnWorkspaceAndRepeats(t *testing.T) {
	t.Parallel()
	f := newSetRelated(models.StatusBacklog)

	_, err := f.useCase.Execute(context.Background(), f.task.ID, []string{ownWorkspace, relatedAPI, relatedAPI})

	require.NoError(t, err)
	assert.Equal(t, []string{relatedAPI}, f.related.stored[f.task.ID.String()])
}
