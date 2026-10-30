package models_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func stringPtr(value string) *string {
	return &value
}

func TestProfileNameAcceptsValidNames(t *testing.T) {
	for _, raw := range []string{"Claude", "a", "gpt-5.1_test", stringRepeat("a", 64)} {
		name, err := models.ParseProfileName(raw)
		require.NoError(t, err, raw)
		require.Equal(t, raw, name.String())
	}
}

func TestProfileNameRejectsEmptyName(t *testing.T) {
	_, err := models.ParseProfileName("")

	require.Equal(t, agenterrors.InvalidProfileName(""), err)
}

func TestProfileNameRejectsSymbolAtStart(t *testing.T) {
	for _, raw := range []string{"-claude", ".claude", "_claude"} {
		_, err := models.ParseProfileName(raw)
		require.Error(t, err, raw)
	}
}

func TestProfileNameRejectsInvalidCharacters(t *testing.T) {
	for _, raw := range []string{"claude profile", "claude/5", "clauдe"} {
		_, err := models.ParseProfileName(raw)
		require.Error(t, err, raw)
	}
}

func TestProfileNameRejectsNameLongerThanMax(t *testing.T) {
	_, err := models.ParseProfileName(stringRepeat("a", 65))

	require.Error(t, err)
}

func TestProfileNameRejectsReservedNames(t *testing.T) {
	for _, reserved := range []string{"__proto__", "constructor", "prototype"} {
		_, err := models.ParseProfileName(reserved)
		require.Error(t, err, reserved)
	}
	_, err := models.ParseProfileName("constructor2")
	require.NoError(t, err)
}

func TestRoleNameAcceptsValidNames(t *testing.T) {
	_, errOne := models.ParseRoleName("sdd-apply")
	_, errTwo := models.ParseRoleName("reviewer2")

	require.NoError(t, errOne)
	require.NoError(t, errTwo)
}

func TestRoleNameRejectsInvalidNames(t *testing.T) {
	_, empty := models.ParseRoleName("")
	require.Equal(t, agenterrors.InvalidRoleName(""), empty)
	for _, raw := range []string{"-x", "a/b", "a b", stringRepeat("a", 65)} {
		_, err := models.ParseRoleName(raw)
		require.Error(t, err, raw)
	}
}

func TestRoleNameRejectsReservedNames(t *testing.T) {
	for _, reserved := range []string{"__proto__", "constructor", "prototype"} {
		_, err := models.ParseRoleName(reserved)
		require.Error(t, err, reserved)
	}
}

func TestNewAgentRoleValidateRejectsBlankModelAndNewlines(t *testing.T) {
	role, err := models.ParseRoleName("reviewer")
	require.NoError(t, err)
	build := func(model *string) models.NewAgentRole {
		return models.NewAgentRole{Name: role, Model: model}
	}

	require.Error(t, build(stringPtr("  ")).Validate())
	require.Error(t, build(stringPtr("a\nb")).Validate())
	require.Error(t, build(stringPtr("a\rb")).Validate())
	require.NoError(t, build(stringPtr("p/m")).Validate())
	require.NoError(t, build(nil).Validate())
}

func TestNewAgentRoleValidateRejectsUnknownThinking(t *testing.T) {
	role, err := models.ParseRoleName("reviewer")
	require.NoError(t, err)

	validation := models.NewAgentRole{Name: role, Thinking: stringPtr("ultra")}.Validate()

	require.ErrorIs(t, validation, agenterrors.InvalidRouting(""))
}

func TestThinkingLevelAcceptsEveryKnownLevel(t *testing.T) {
	for _, level := range models.ThinkingLevels() {
		require.NoError(t, models.ValidateThinkingLevel(level), level)
	}
}

func TestThinkingLevelRejectsUnknownLevel(t *testing.T) {
	err := models.ValidateThinkingLevel("ultra")

	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
	require.EqualError(t, err, "routing inválido: nivel de razonamiento inválido: ultra")
}

func TestRoutingEntryEmptyEntryIsValid(t *testing.T) {
	require.NoError(t, models.RoutingEntry{}.Validate())
}

func TestRoutingEntryAcceptsModelAndKnownThinking(t *testing.T) {
	entry := models.RoutingEntry{Model: stringPtr("anthropic/claude"), Thinking: stringPtr("high")}

	require.NoError(t, entry.Validate())
}

func TestRoutingEntryRejectsBlankModel(t *testing.T) {
	entry := models.RoutingEntry{Model: stringPtr("  ")}

	require.Equal(t, agenterrors.InvalidRouting("modelo vacío"), entry.Validate())
}

func TestRoutingEntryRejectsUnknownThinking(t *testing.T) {
	entry := models.RoutingEntry{Thinking: stringPtr("ultra")}

	require.ErrorIs(t, entry.Validate(), agenterrors.InvalidRouting(""))
}

func TestCloneRoutingSharesNoPointers(t *testing.T) {
	original := map[string]models.RoutingEntry{"orchestrator": {Model: stringPtr("a/b")}}

	cloned := models.CloneRouting(original)
	*original["orchestrator"].Model = "changed"

	require.Equal(t, "a/b", *cloned["orchestrator"].Model)
	require.NotNil(t, models.CloneRouting(nil))
}

func TestProfileOrchestratorKeyIsAllowed(t *testing.T) {
	routing := map[string]models.RoutingEntry{models.OrchestratorKey: {Model: stringPtr("a/b")}}

	require.NoError(t, models.ValidateRouting(routing))
}

func TestProfileRejectsBlankKey(t *testing.T) {
	routing := map[string]models.RoutingEntry{" ": {Model: stringPtr("a/b")}}

	require.Equal(t, agenterrors.InvalidRouting("clave de agente vacía"), models.ValidateRouting(routing))
}

func TestProfileRejectsInvalidEntry(t *testing.T) {
	routing := map[string]models.RoutingEntry{"sdd-apply": {Thinking: stringPtr("ultra")}}

	require.Error(t, models.ValidateRouting(routing))
}

func TestProfileStoreFindAndContainsMatchByName(t *testing.T) {
	fast, err := models.ParseProfileName("fast")
	require.NoError(t, err)
	slow, err := models.ParseProfileName("slow")
	require.NoError(t, err)
	store := models.NewProfileStore(nil, []models.Profile{models.NewProfile(fast, nil)})

	require.True(t, store.Contains(fast))
	require.False(t, store.Contains(slow))
	require.Equal(t, fast, store.Find(fast).Name)
}

func TestExportedProfileRoundTripsWithProfile(t *testing.T) {
	fast, err := models.ParseProfileName("fast")
	require.NoError(t, err)
	profile := models.NewProfile(fast, map[string]models.RoutingEntry{"orchestrator": {}})

	exported := models.NewExportedProfile(profile)

	require.Equal(t, profile, exported.ToProfile())
}

func stringRepeat(value string, count int) string {
	return strings.Repeat(value, count)
}
