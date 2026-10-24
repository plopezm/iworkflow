package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

func TestLaneDefinitionTrimsNameAndQuery(t *testing.T) {
	definition, err := models.NewLaneDefinition("  Backend ", " project = X ORDER BY updated ")
	require.NoError(t, err)
	assert.Equal(t, "Backend", definition.Name())
	assert.Equal(t, "project = X ORDER BY updated", definition.JQL())
}

func TestLaneDefinitionRejectsBlankNameAndQuery(t *testing.T) {
	_, err := models.NewLaneDefinition(" ", "a = b")
	require.ErrorIs(t, err, jiraerrors.EmptyLaneName())

	_, err = models.NewLaneDefinition("x", "  ")
	require.ErrorIs(t, err, jiraerrors.EmptyLaneJQL())
}

func TestLaneDefinitionRejectsOverlongNames(t *testing.T) {
	_, err := models.NewLaneDefinition(strings.Repeat("x", models.MaxLaneNameChars+1), "a = b")
	require.ErrorIs(t, err, jiraerrors.LaneNameTooLong(models.MaxLaneNameChars))
	require.EqualError(t, err, "el nombre del carril no puede superar 60 caracteres")

	_, err = models.NewLaneDefinition(strings.Repeat("x", models.MaxLaneNameChars), "a = b")
	require.NoError(t, err)
}
