package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
)

func TestNewGroupTrimsNameAndKeepsAnOptionalPaletteColor(t *testing.T) {
	group, err := models.NewGroup("g1", "  Backend  ", " blue ", false)
	require.NoError(t, err)
	assert.Equal(t, "Backend", group.Name())
	assert.Equal(t, "blue", group.Color())
	assert.Equal(t, "g1", group.ID())
	assert.False(t, group.Collapsed())

	plain, err := models.NewGroup("g2", "Misc", "", true)
	require.NoError(t, err)
	assert.Empty(t, plain.Color())
	assert.True(t, plain.Collapsed())
}

func TestNewGroupRejectsBlankIDNameAndUnknownColor(t *testing.T) {
	for name, args := range map[string][3]string{
		"blank id":      {" ", "a", ""},
		"blank name":    {"g", "   ", ""},
		"unknown color": {"g", "a", "chartreuse"},
		"long name":     {"g", string(make([]byte, 81)), ""},
	} {
		_, err := models.NewGroup(args[0], args[1], args[2], false)
		require.ErrorIs(t, err, gerrors.ErrInvalidGroup, name)
	}
}

func TestColorsListsThePaletteInDisplayOrder(t *testing.T) {
	assert.Equal(t, []string{"red", "orange", "amber", "green", "teal", "cyan", "blue", "violet", "pink", "gray"}, models.Colors())
}
