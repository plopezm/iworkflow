package agent_settings_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/infrastructure/di"
)

// newTestModule wires the module on a fresh database and a gentle-pi home with the given catalog.
func newTestModule(t *testing.T, profilesJSON string) (*di.Module, string) {
	t.Helper()
	configHome := t.TempDir()
	if profilesJSON != "" {
		require.NoError(t, os.WriteFile(filepath.Join(configHome, "profiles.json"), []byte(profilesJSON), 0o600))
	}
	module := di.NewModule(di.Dependencies{
		DB:                 dbtest.New(t),
		GentleShellBin:     "/no/existe-gentle-shell",
		PiBin:              "/no/existe-pi",
		CursorBin:          "/no/existe-agent",
		GentlePiConfigHome: configHome,
	})
	return module, configHome
}

func TestFreshModuleReturnsDefaultSettings(t *testing.T) {
	module, _ := newTestModule(t, "")

	settings, err := module.GetAgentSettings.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, models.AgentSettings{}, settings)
}

func TestUpdatedSettingsAreReturnedAndPersisted(t *testing.T) {
	module, _ := newTestModule(t, "")
	pi := models.RuntimePi
	thinking := models.ThinkingMedium
	memory := uint32(1024)

	saved, err := module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{
		Runtime:       &pi,
		Thinking:      &thinking,
		MemoryLimitMB: &memory,
	})
	require.NoError(t, err)
	loaded, err := module.GetAgentSettings.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, saved, loaded)
}

func TestUpdateRejectsAProfileMissingFromProfilesJSON(t *testing.T) {
	module, _ := newTestModule(t, `{"active": "Claude", "profiles": {"Claude": {}}}`)

	_, err := module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{
		Profile: profileOf(t, "Ghost"),
	})

	require.ErrorIs(t, err, agenterrors.ErrProfileNotFound)
	require.EqualError(t, err, "perfil no encontrado: Ghost")
	stored, err := module.GetAgentSettings.Execute(context.Background())
	require.NoError(t, err)
	require.Nil(t, stored.Profile)
}

func TestListProfilesReadsTheGentlePiConfigHome(t *testing.T) {
	module, _ := newTestModule(t, `{"active": "Gentleman", "profiles": {"Gentleman": {}, "Claude": {}}}`)

	catalog, err := module.ListProfiles.Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, catalog.Active)
	require.Equal(t, "Gentleman", *catalog.Active)
	require.Equal(t, []string{"Claude", "Gentleman"}, catalog.Names)
}

func TestClaudeModelsAreValidatedAgainstTheStaticAliases(t *testing.T) {
	module, _ := newTestModule(t, "")
	claude := models.RuntimeClaude
	opus := models.ModelRef{Provider: "anthropic", Model: "opus"}
	unknown := models.ModelRef{Provider: "anthropic", Model: "gpt-9"}

	saved, err := module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{
		Runtime: &claude,
		Model:   &opus,
	})
	require.NoError(t, err)
	require.Equal(t, &opus, saved.Model)

	_, err = module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{
		Runtime: &claude,
		Model:   &unknown,
	})
	require.EqualError(t, err, "modelo no disponible: anthropic/gpt-9")
}

func TestClaudeModelsAreListedWithoutRunningAnyBinary(t *testing.T) {
	module, _ := newTestModule(t, "")
	claude := models.RuntimeClaude
	_, err := module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{Runtime: &claude})
	require.NoError(t, err)

	list, err := module.ListModels.Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, list, 4)
}

func TestSavedSettingsFlowIntoThePrepareLaunchAndThePinOfTheWorktree(t *testing.T) {
	module, _ := newTestModule(t, `{"active": "Claude", "profiles": {"Claude": {}}}`)
	claude := models.RuntimeClaude
	effort := models.ThinkingHigh
	memory := uint32(2048)
	cpu := uint32(100)
	_, err := module.UpdateAgentSettings.Execute(context.Background(), models.AgentSettings{
		Runtime:         &claude,
		Profile:         profileOf(t, "Claude"),
		Thinking:        &effort,
		MemoryLimitMB:   &memory,
		CPULimitPercent: &cpu,
	})
	require.NoError(t, err)
	worktree := gitRepo(t)

	preferences, err := module.PrepareLaunch.Execute(context.Background(), worktree)

	require.NoError(t, err)
	require.Equal(t, models.LaunchPreferences{
		Runtime:         &claude,
		Thinking:        &effort,
		MemoryLimitMB:   &memory,
		CPULimitPercent: &cpu,
	}, preferences)
	pin, err := os.ReadFile(pinPath(filepath.Join(worktree, ".git")))
	require.NoError(t, err)
	require.Contains(t, string(pin), `"profile": "Claude"`)
}

func TestPrepareLaunchRemovesThePinWhenNoProfileIsSelected(t *testing.T) {
	module, _ := newTestModule(t, "")
	worktree := gitRepo(t)
	pinFile := pinPath(filepath.Join(worktree, ".git"))
	require.NoError(t, os.MkdirAll(filepath.Dir(pinFile), 0o755))
	require.NoError(t, os.WriteFile(pinFile, []byte("stale"), 0o644))

	_, err := module.PrepareLaunch.Execute(context.Background(), worktree)

	require.NoError(t, err)
	require.NoFileExists(t, pinFile)
}
