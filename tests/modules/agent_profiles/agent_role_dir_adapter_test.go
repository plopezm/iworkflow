package agent_profiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/infrastructure/adapters/outbound/roles"
)

func newRole(t *testing.T, name string, description string, prompt string) models.NewAgentRole {
	t.Helper()
	roleName, err := models.ParseRoleName(name)
	require.NoError(t, err)
	return models.NewAgentRole{Name: roleName, Description: description, Prompt: prompt}
}

func roleNames(roleList []models.AgentRole) []string {
	names := make([]string, 0, len(roleList))
	for _, role := range roleList {
		names = append(names, role.Name.String())
	}
	return names
}

func TestMissingDirListsEmpty(t *testing.T) {
	dir := t.TempDir()
	adapter := roles.NewAgentRoleDirAdapter(filepath.Join(dir, "nope"))

	listed, err := adapter.List(t.Context())

	require.NoError(t, err)
	require.Empty(t, listed)
}

func TestCreateThenListSorted(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "agents")
	adapter := roles.NewAgentRoleDirAdapter(agents)
	require.NoError(t, adapter.Create(t.Context(), newRole(t, "zeta", "Last one", "Do z.\n")))
	require.NoError(t, adapter.Create(t.Context(), newRole(t, "alpha", "First", "Do a.")))

	listed, err := adapter.List(t.Context())

	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "zeta"}, roleNames(listed))
	require.Equal(t, "Last one", listed[1].Description)
	require.Nil(t, listed[0].Model)
	raw, err := os.ReadFile(filepath.Join(agents, "alpha.md"))
	require.NoError(t, err)
	require.Contains(t, string(raw), "---\nname: alpha\ndescription: First\n---\n")
	require.Contains(t, string(raw), "Do a.")
}

func TestCreateNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	adapter := roles.NewAgentRoleDirAdapter(dir)
	require.NoError(t, adapter.Create(t.Context(), newRole(t, "dup", "one", "first")))

	err := adapter.Create(t.Context(), newRole(t, "dup", "two", "second"))

	require.Equal(t, agenterrors.RoleAlreadyExists("dup"), err)
	raw, err := os.ReadFile(filepath.Join(dir, "dup.md"))
	require.NoError(t, err)
	require.Contains(t, string(raw), "first")
	require.NotContains(t, string(raw), "second")
}

func TestCreateRejectsNewlineInDescriptionAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	adapter := roles.NewAgentRoleDirAdapter(dir)

	err := adapter.Create(t.Context(), newRole(t, "bad", "line1\nmodel: evil", "p"))

	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(dir, "bad.md"))
}

func TestListParsesExistingFilesAndSkipsNonMd(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "worker.md"),
		[]byte("---\nname: gentle-worker\ndescription: Does work\nmodel: p/m\nthinking: low\ntools:\n  - read\n---\n\nBody\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stem-only.md"), []byte("---\ndescription: \"Quoted\"\n---\nx"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600))
	adapter := roles.NewAgentRoleDirAdapter(dir)

	listed, err := adapter.List(t.Context())

	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, "gentle-worker", listed[0].Name.String())
	require.Equal(t, "p/m", *listed[0].Model)
	require.Equal(t, "low", *listed[0].Thinking)
	require.Equal(t, "stem-only", listed[1].Name.String())
	require.Equal(t, "Quoted", listed[1].Description)
}

func TestListSkipsUnreadableEntries(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "good.md"), []byte("---\nname: good\n---\nbody\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "binary.md"), []byte{0xff, 0xfe, 0x00, 0xc3}, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad name.md"), []byte("no frontmatter"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "dangling.md")))
	adapter := roles.NewAgentRoleDirAdapter(dir)

	listed, err := adapter.List(t.Context())

	require.NoError(t, err)
	require.Equal(t, []string{"good"}, roleNames(listed))
}

func TestCreateWithModelAndThinkingPersistsAndListsThem(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "agents")
	adapter := roles.NewAgentRoleDirAdapter(agents)
	role := newRole(t, "tuned", "Tuned role", "Body.")
	model := "anthropic/claude"
	thinking := "high"
	role.Model = &model
	role.Thinking = &thinking
	require.NoError(t, adapter.Create(t.Context(), role))

	raw, err := os.ReadFile(filepath.Join(agents, "tuned.md"))
	require.NoError(t, err)
	require.Contains(t, string(raw), "\nmodel: anthropic/claude\n")
	require.Contains(t, string(raw), "\nthinking: high\n")
	listed, err := adapter.List(t.Context())
	require.NoError(t, err)
	require.Equal(t, "anthropic/claude", *listed[0].Model)
	require.Equal(t, "high", *listed[0].Thinking)
}

func TestCreateWithoutModelAndThinkingWritesNoSuchLines(t *testing.T) {
	dir := t.TempDir()
	agents := filepath.Join(dir, "agents")
	adapter := roles.NewAgentRoleDirAdapter(agents)
	require.NoError(t, adapter.Create(t.Context(), newRole(t, "plain", "Plain", "Body.")))

	raw, err := os.ReadFile(filepath.Join(agents, "plain.md"))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "model:")
	require.NotContains(t, string(raw), "thinking:")
}
