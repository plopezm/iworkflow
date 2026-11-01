package dto_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/inbound/dto"
)

func TestCreateLauncherFromFormReadsOptionalLines(t *testing.T) {
	input, err := dto.CreateLauncherFromForm(url.Values{
		"name": {"Editor"}, "path": {"/usr/bin/code"},
		"args": {"--new-window\r\n\r\n--wait"}, "env": {"A=1\nB=x=y"},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"--new-window", "--wait"}, input.Args)
	require.Equal(t, []models.EnvVar{{Name: "A", Value: "1"}, {Name: "B", Value: "x=y"}}, input.Env)
}

func TestCreateLauncherFromFormAllowsMissingArgsAndEnv(t *testing.T) {
	input, err := dto.CreateLauncherFromForm(url.Values{"name": {"E"}, "path": {"code"}})

	require.NoError(t, err)
	require.Empty(t, input.Args)
	require.Empty(t, input.Env)
}

func TestCreateLauncherFromFormRejectsAVariableWithoutEquals(t *testing.T) {
	_, err := dto.CreateLauncherFromForm(url.Values{"name": {"E"}, "path": {"code"}, "env": {"BROKEN"}})

	require.ErrorContains(t, err, "NAME=VALUE")
}
