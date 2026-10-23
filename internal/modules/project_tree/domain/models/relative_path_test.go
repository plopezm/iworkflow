package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
)

func TestValidateRelativeAcceptsRootAndPlainNames(t *testing.T) {
	for _, relative := range []string{"", "a", "src/main.rs", ".env", "...", ".hidden/x"} {
		require.NoError(t, models.ValidateRelative(relative), "relative %q", relative)
	}
}

func TestValidateRelativeRejectsEscapesAndMalformedPaths(t *testing.T) {
	for _, relative := range []string{
		"..", "../x", "a/../b", "a/..", "./a", "a/.", "/etc", "/tmp",
		"a//b", "a/", "/",
	} {
		require.Equal(t, treeerrors.InvalidPath(relative), models.ValidateRelative(relative), "relative %q", relative)
	}
}

func TestIsWithinComparesWholeComponents(t *testing.T) {
	require.True(t, models.IsWithin("/work/app", "/work/app"))
	require.True(t, models.IsWithin("/work/app", "/work/app/src/main.rs"))
	require.True(t, models.IsWithin("/work/app", "/work/app/..foo"))
	require.False(t, models.IsWithin("/work/app", "/work/app2"), "a sibling with the same prefix is outside")
	require.False(t, models.IsWithin("/work/app", "/work"))
	require.False(t, models.IsWithin("/work/app", "/other/x"))
}
