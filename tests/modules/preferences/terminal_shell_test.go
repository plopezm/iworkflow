package preferences_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/persistence"
)

// fakePicker stands in for the native file dialog.
type fakePicker struct {
	path string
	err  error
}

func (f *fakePicker) Pick(context.Context) (string, error) {
	return f.path, f.err
}

func TestTerminalShellIsTheDefaultBeforeAnyChoice(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))

	shell, err := repository.TerminalShell(context.Background())

	require.NoError(t, err)
	require.Empty(t, shell.Path())
}

func TestTerminalShellRoundtripAndReset(t *testing.T) {
	repository := persistence.NewPreferencesRepository(dbtest.New(t))
	chosen, err := models.ParseTerminalShell(`C:\Program Files\PowerShell\7\pwsh.exe`)
	require.NoError(t, err)
	require.NoError(t, repository.SaveTerminalShell(context.Background(), chosen))

	shell, err := repository.TerminalShell(context.Background())
	require.NoError(t, err)
	require.Equal(t, `C:\Program Files\PowerShell\7\pwsh.exe`, shell.Path())

	require.NoError(t, repository.SaveTerminalShell(context.Background(), models.TerminalShell{}))
	shell, err = repository.TerminalShell(context.Background())
	require.NoError(t, err)
	require.Empty(t, shell.Path())
}

func TestTerminalShellCardShowsTheDefaultAndSavesAChoice(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodGet, "/preferences/terminal-shell", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Examinar")
	require.NotContains(t, rec.Body.String(), "Usar el predeterminado")

	rec = s.send(t, http.MethodPost, "/preferences/terminal-shell", url.Values{"shell": {"/bin/zsh"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `value="/bin/zsh"`)
	require.Contains(t, rec.Body.String(), "Usar el predeterminado")

	stored, err := s.module.GetTerminalShell.Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, "/bin/zsh", stored.Path())
}

func TestTerminalShellInvalidInputIsShownAndNotStored(t *testing.T) {
	s := newSettings(t, newTestToken(t))

	rec := s.send(t, http.MethodPost, "/preferences/terminal-shell", url.Values{"shell": {"a\nb"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "shell no válido")
	stored, err := s.module.GetTerminalShell.Execute(context.Background())
	require.NoError(t, err)
	require.Empty(t, stored.Path())
}

func TestTerminalShellResetRestoresTheDefault(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.send(t, http.MethodPost, "/preferences/terminal-shell", url.Values{"shell": {"/bin/zsh"}})

	rec := s.send(t, http.MethodPost, "/preferences/terminal-shell/reset", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "Usar el predeterminado")
	stored, err := s.module.GetTerminalShell.Execute(context.Background())
	require.NoError(t, err)
	require.Empty(t, stored.Path())
}

func TestTerminalShellPickStoresTheChosenProgram(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.picker.path = `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`

	rec := s.send(t, http.MethodPost, "/preferences/terminal-shell/pick", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	stored, err := s.module.GetTerminalShell.Execute(context.Background())
	require.NoError(t, err)
	require.Equal(t, s.picker.path, stored.Path())
}

func TestTerminalShellPickWithoutADialogExplainsIt(t *testing.T) {
	s := newSettings(t, newTestToken(t))
	s.picker.err = domainerrors.ErrPickerUnavailable

	rec := s.send(t, http.MethodPost, "/preferences/terminal-shell/pick", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "no se pudo abrir el selector")
}
