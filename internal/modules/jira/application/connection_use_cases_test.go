package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/jira/application"
	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

func saveInput(token string) application.SaveConnectionInput {
	return application.SaveConnectionInput{BaseURL: "https://jira.local/", Token: token}
}

func TestSaveConnectionVerifiesThenSavesANewConnection(t *testing.T) {
	connections := &fakeConnections{}
	issues := &fakeIssues{}
	client := newFakeClient()

	saved, err := application.NewSaveConnection(connections, issues, client).Execute(context.Background(), saveInput("tok"))

	require.NoError(t, err)
	assert.Equal(t, 1, client.calls["verify"])
	require.Len(t, connections.saved, 1)
	assert.Equal(t, "https://jira.local", connections.saved[0].BaseURL())
	assert.Equal(t, "tok", connections.saved[0].Token())
	assert.Equal(t, "tok", saved.Token())
	assert.Equal(t, 0, issues.deleteAll, "a new connection discards nothing")
}

func TestAFailedVerificationSavesNothing(t *testing.T) {
	connections := &fakeConnections{}
	client := newFakeClient()
	client.verifyFn = func() (string, error) { return "", jiraerrors.Unauthorized() }

	_, err := application.NewSaveConnection(connections, &fakeIssues{}, client).Execute(context.Background(), saveInput("bad"))

	require.ErrorIs(t, err, jiraerrors.Unauthorized())
	assert.Empty(t, connections.saved)
}

func TestBlankTokenKeepsTheStoredOne(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "kept")
	issues := &fakeIssues{}
	client := newFakeClient()

	saved, err := application.NewSaveConnection(connections, issues, client).Execute(context.Background(), saveInput("  "))

	require.NoError(t, err)
	assert.Equal(t, "kept", saved.Token())
	assert.Equal(t, 0, issues.deleteAll)
}

func TestBlankTokenWithoutStoredConnectionIsRejected(t *testing.T) {
	_, err := application.NewSaveConnection(&fakeConnections{}, &fakeIssues{}, newFakeClient()).
		Execute(context.Background(), saveInput(""))

	require.ErrorIs(t, err, jiraerrors.EmptyToken())
}

func TestBlankTokenWithADifferentURLIsRejectedBeforeContactingTheServer(t *testing.T) {
	connections := connectedRepo(t, "https://old.local", "secret")
	client := newFakeClient()

	_, err := application.NewSaveConnection(connections, &fakeIssues{}, client).Execute(context.Background(), saveInput(""))

	require.ErrorIs(t, err, jiraerrors.TokenRequiredForNewURL())
	assert.Zero(t, client.calls["verify"])
	assert.Empty(t, connections.saved)
}

func TestAFailedSaveDoesNotWipeTheImportedIssues(t *testing.T) {
	connections := connectedRepo(t, "https://old.local", "tok")
	connections.saveErr = jiraerrors.Repository("disco lleno")
	issues := &fakeIssues{}

	_, err := application.NewSaveConnection(connections, issues, newFakeClient()).Execute(context.Background(), saveInput("tok"))

	require.ErrorIs(t, err, jiraerrors.Repository(""))
	assert.Equal(t, 0, issues.deleteAll)
}

func TestChangingTheServerDiscardsTheImportedIssues(t *testing.T) {
	connections := connectedRepo(t, "https://old.local", "tok")
	issues := &fakeIssues{}

	_, err := application.NewSaveConnection(connections, issues, newFakeClient()).Execute(context.Background(), saveInput("tok"))

	require.NoError(t, err)
	assert.Equal(t, 1, issues.deleteAll)
}

func TestGetConnectionReturnsNilWhenNotConfigured(t *testing.T) {
	connection, err := application.NewGetConnection(&fakeConnections{}).Execute(context.Background())
	require.NoError(t, err)
	assert.Nil(t, connection)
}

func TestDeleteConnectionRemovesIssuesAndConnection(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "tok")
	issues := &fakeIssues{}

	require.NoError(t, application.NewDeleteConnection(connections, issues).Execute(context.Background()))

	assert.Equal(t, 1, issues.deleteAll)
	assert.Equal(t, 1, connections.deleted)
}

func TestDeleteConnectionKeepsTheConnectionIfClearingIssuesFails(t *testing.T) {
	connections := connectedRepo(t, "https://jira.local", "tok")
	issues := &fakeIssues{deleteAllErr: jiraerrors.Repository("x")}

	err := application.NewDeleteConnection(connections, issues).Execute(context.Background())

	require.ErrorIs(t, err, jiraerrors.Repository(""))
	assert.Zero(t, connections.deleted)
}

func TestNoErrorLeaksTheTokenFromTheSaveFlow(t *testing.T) {
	connections := &fakeConnections{saveErr: errors.New("write failed")}
	_, err := application.NewSaveConnection(connections, &fakeIssues{}, newFakeClient()).Execute(context.Background(), saveInput("pat-secret"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "pat-secret")
}
