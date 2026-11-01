package models_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
)

func TestNewLauncherTrimsAndDropsBlankArguments(t *testing.T) {
	launcher, err := models.NewLauncher("id", "  Editor ", " /usr/bin/code ", []string{"--a", " ", "", "b c"}, nil)

	require.NoError(t, err)
	require.Equal(t, "Editor", launcher.Name())
	require.Equal(t, "/usr/bin/code", launcher.Path())
	require.Equal(t, []string{"--a", "b c"}, launcher.Args())
}

func TestNewLauncherRejectsBrokenInvariants(t *testing.T) {
	tests := map[string]struct {
		id, name, path string
		env            []models.EnvVar
	}{
		"blank id":          {id: " ", name: "n", path: "p"},
		"blank name":        {id: "i", name: " ", path: "p"},
		"blank path":        {id: "i", name: "n", path: " "},
		"nul in path":       {id: "i", name: "n", path: "a\x00b"},
		"empty var name":    {id: "i", name: "n", path: "p", env: []models.EnvVar{{Name: " ", Value: "v"}}},
		"var name with =":   {id: "i", name: "n", path: "p", env: []models.EnvVar{{Name: "A=B"}}},
		"var name with gap": {id: "i", name: "n", path: "p", env: []models.EnvVar{{Name: "A B"}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := models.NewLauncher(tt.id, tt.name, tt.path, nil, tt.env)

			require.ErrorIs(t, err, openerrors.ErrInvalidLauncher)
		})
	}
}

func TestLauncherReturnsCopiesAndBuildsTheSpec(t *testing.T) {
	launcher, err := models.NewLauncher("id", "n", "/bin/x", []string{"a"}, []models.EnvVar{{Name: "K", Value: "v"}})
	require.NoError(t, err)

	launcher.Args()[0] = "changed"
	launcher.Env()[0].Value = "changed"

	require.Equal(t, models.LaunchSpec{Path: "/bin/x", Args: []string{"a", "/w"}, Env: []models.EnvVar{{Name: "K", Value: "v"}}, Dir: "/w"}, launcher.SpecIn("/w"))
}
