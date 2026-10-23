package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/application"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestListProfilesReturnsCatalogFromPort(t *testing.T) {
	active := "Claude"
	catalog := &fakeProfileCatalog{catalog: models.ProfileCatalog{Active: &active, Names: []string{"Claude"}}}
	useCase := application.NewListProfiles(catalog)

	result, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, result.Active)
	require.Equal(t, "Claude", *result.Active)
	require.Equal(t, []string{"Claude"}, result.Names)
}
