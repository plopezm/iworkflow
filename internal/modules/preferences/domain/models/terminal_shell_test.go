package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestParseTerminalShell(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{name: "blank is the default", raw: "   ", want: ""},
		{name: "plain path", raw: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`, want: `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`},
		{name: "trimmed", raw: "  /bin/zsh \n", want: "/bin/zsh"},
		{name: "quotes from copy as path", raw: `"C:\Program Files\PowerShell\7\pwsh.exe"`, want: `C:\Program Files\PowerShell\7\pwsh.exe`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shell, err := models.ParseTerminalShell(tc.raw)
			require.NoError(t, err)
			assert.Equal(t, tc.want, shell.Path())
		})
	}
}

func TestParseTerminalShellRejectsMultipleLines(t *testing.T) {
	_, err := models.ParseTerminalShell("/bin/sh\nrm -rf /")
	var invalid *domainerrors.InvalidShellError
	require.ErrorAs(t, err, &invalid)
}
