package models_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestConnectionNormalisesURLAndToken(t *testing.T) {
	connection, err := models.NewConnection("  https://jira.corp.local/jira/ ", " tok ", false)
	require.NoError(t, err)

	assert.Equal(t, "https://jira.corp.local/jira", connection.BaseURL())
	assert.Equal(t, "tok", connection.Token())
	assert.Equal(t, "https://jira.corp.local/jira/rest/api/2/myself", connection.APIURL("/myself"))
	assert.Equal(t, "https://jira.corp.local/jira/browse/ABC-1", connection.BrowseURL("ABC-1"))
}

func TestConnectionRejectsInvalidURLs(t *testing.T) {
	for _, url := range []string{"", "jira.local", "ftp://jira", "https://", "https:// jira"} {
		_, err := models.NewConnection(url, "t", false)
		require.ErrorIs(t, err, jiraerrors.InvalidURL(), url)
	}
}

func TestConnectionRejectsEmptyToken(t *testing.T) {
	_, err := models.NewConnection("https://jira", "  ", false)
	require.ErrorIs(t, err, jiraerrors.EmptyToken())
}

func TestConnectionKeepsTheCertificateFlag(t *testing.T) {
	connection, err := models.NewConnection("http://jira:8080", "t", true)
	require.NoError(t, err)
	assert.True(t, connection.AcceptInvalidCerts())
}

func TestConnectionNeverPrintsTheToken(t *testing.T) {
	connection, err := models.NewConnection("https://jira.local", "super-secret-token", false)
	require.NoError(t, err)

	assert.NotContains(t, fmt.Sprintf("%v %+v %#v %s", connection, connection, connection, connection), "super-secret-token")
}

func TestConnectionSyncRecordIsCopied(t *testing.T) {
	connection, err := models.NewConnection("https://jira.local", "t", false)
	require.NoError(t, err)
	message := "boom"
	recorded := connection.WithSyncRecord(nil, &message)
	message = "changed after the fact"

	require.NotNil(t, recorded.LastSyncError())
	assert.Equal(t, "boom", *recorded.LastSyncError())
	assert.Nil(t, recorded.LastSyncAt())
}
