package code_hosting_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/inbound"
	"iworkflow/internal/modules/code_hosting/infrastructure/di"
)

// noTasks is a task lookup where no task exists: it is enough for the settings flow.
type noTasks struct{}

func (noTasks) Find(_ context.Context, _ string) (*models.TaskSource, error) {
	return nil, nil
}

func TestModuleSavesAndReadsSettingsThroughTheDatabase(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t), Tasks: noTasks{}})

	saved, err := module.SaveSettings.Execute(ctx, application.SaveCodeHostSettingsInput{
		Provider: models.GitLab,
		BaseURL:  "https://git.corp/",
		Token:    "tok-1",
	})
	require.NoError(t, err)
	require.Equal(t, "https://git.corp", saved.BaseURL())

	read, err := module.GetSettings.Execute(ctx, models.GitLab)
	require.NoError(t, err)
	require.NotNil(t, read)
	require.Equal(t, "tok-1", read.Token())
}

func TestModuleRemoteMergeIsBoundToTheMergeUseCase(t *testing.T) {
	ctx := context.Background()
	// The composition root creates the port first, hands it to the tasks module and binds it later.
	remote := application.NewRemoteMerge()
	module := di.NewModule(di.Deps{DB: dbtest.New(t), Tasks: noTasks{}, RemoteMerge: remote})

	require.Same(t, remote, module.RemoteMerge)
	outcome, err := remote.MergeOpenRequest(ctx, "task-1")

	require.NoError(t, err)
	require.Equal(t, inbound.RemoteMergeNoOpenRequest, outcome)
}
