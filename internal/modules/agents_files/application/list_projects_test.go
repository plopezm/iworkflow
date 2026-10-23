package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agents_files/application"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
)

func TestListProjectsReturnsProjectsFromTheLookup(t *testing.T) {
	projects := &fakeProjects{projects: []models.ProjectRef{{ID: "1", Name: "demo", Path: "/repos/demo"}}}
	uc := application.NewListProjects(projects)

	result, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []models.ProjectRef{{ID: "1", Name: "demo", Path: "/repos/demo"}}, result)
}

func TestListProjectsPropagatesLookupErrors(t *testing.T) {
	uc := application.NewListProjects(&fakeProjects{listErr: agentserrors.Repository("boom")})

	_, err := uc.Execute(context.Background())

	require.True(t, errors.Is(err, agentserrors.Repository("")))
	require.Equal(t, agentserrors.Repository("boom"), err)
}
