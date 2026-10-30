package roles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func newTestRole(t *testing.T, description string, prompt string) models.NewAgentRole {
	t.Helper()
	name, err := models.ParseRoleName("r")
	require.NoError(t, err)
	return models.NewAgentRole{Name: name, Description: description, Prompt: prompt}
}

func TestParseFrontmatterReadsScalarKeysAndIgnoresListsAndBody(t *testing.T) {
	parsed := parseFrontmatter("---\nname: x\ndescription: 'It''s ok'\nmodel: p/m\nthinking:\ntools:\n  - read\n---\nname: nope\n")

	require.Equal(t, "x", parsed.name)
	require.Equal(t, "It's ok", parsed.description)
	require.Equal(t, "p/m", parsed.model)
	require.Empty(t, parsed.thinking)
}

func TestParseFrontmatterWithoutOpeningDelimiterYieldsNothing(t *testing.T) {
	require.Equal(t, frontmatter{}, parseFrontmatter("hello\n---\nname: x"))
}

func TestRoleFromFileFallsBackToStemWhenNameIsInvalid(t *testing.T) {
	role, ok := roleFromFile("stem", "---\nname: bad name\n---\n")

	require.True(t, ok)
	require.Equal(t, "stem", role.Name.String())
	_, ok = roleFromFile("bad stem", "no frontmatter")
	require.False(t, ok)
}

func TestRenderRoleQuotesRiskyDescriptionsAndRejectsNewlines(t *testing.T) {
	rendered, err := renderRole(newTestRole(t, "Does: things", "body"))
	require.NoError(t, err)

	require.Contains(t, rendered, "description: \"Does: things\"\n")
	require.Equal(t, "Does: things", parseFrontmatter(rendered).description)
	_, err = renderRole(newTestRole(t, "a\nb", "x"))
	require.ErrorIs(t, err, agenterrors.Store(""))
	_, err = renderRole(newTestRole(t, "a\rb", "x"))
	require.ErrorIs(t, err, agenterrors.Store(""))
}

func TestRenderRoleAppendsPromptWithSingleTrailingNewline(t *testing.T) {
	rendered, err := renderRole(newTestRole(t, "d", "body\n\n"))

	require.NoError(t, err)
	require.True(t, strings.HasSuffix(rendered, "---\n\nbody\n"))
}

func TestRenderRoleWritesModelAndThinkingOnlyWhenPresent(t *testing.T) {
	role := newTestRole(t, "d", "body")
	plain, err := renderRole(role)
	require.NoError(t, err)
	require.NotContains(t, plain, "model:")
	require.NotContains(t, plain, "thinking:")

	model := "p/m"
	thinking := "high"
	role.Model = &model
	role.Thinking = &thinking
	rendered, err := renderRole(role)
	require.NoError(t, err)
	require.Contains(t, rendered, "\nmodel: p/m\nthinking: high\n---\n")

	broken := "a\nb"
	role.Model = &broken
	_, err = renderRole(role)
	require.Error(t, err)
}

func TestSplitExtensionTreatsLeadingDotAsStem(t *testing.T) {
	stem, extension := splitExtension(".md")
	require.Equal(t, ".md", stem)
	require.Empty(t, extension)

	stem, extension = splitExtension("a.b.md")
	require.Equal(t, "a.b", stem)
	require.Equal(t, "md", extension)
}

func TestFinishWriteRemovesPartialFileOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "partial.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nname: par"), 0o600))

	err := finishWrite(path, errors.New("disk full"))

	require.ErrorIs(t, err, agenterrors.IO(""))
	require.NoFileExists(t, path)
	require.NoError(t, os.WriteFile(path, []byte("ok"), 0o600))
	require.NoError(t, finishWrite(path, nil))
	require.FileExists(t, path)
}

func TestReadRoleFileSkipsNonUTF8Content(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin.md")
	require.NoError(t, os.WriteFile(path, []byte{0xff, 0xfe, 0x00}, 0o600))

	_, ok := readRoleFile(path)

	require.False(t, ok)
}
