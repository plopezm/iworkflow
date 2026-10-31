package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

const tableHeader = "provider model context max-out thinking images"

func bothRuntimes(gentleShell, pi string) *AgentCLIModelCatalogAdapter {
	return NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{
		models.RuntimeGentleShell: gentleShell,
		models.RuntimePi:          pi,
	})
}

func TestClaudeListsItsStaticAliasesWithoutRunningABinary(t *testing.T) {
	adapter := NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{})

	result, err := adapter.List(context.Background(), models.RuntimeClaude)

	require.NoError(t, err)
	require.Equal(t, []models.ModelRef{
		{Provider: "anthropic", Model: "fable"},
		{Provider: "anthropic", Model: "opus"},
		{Provider: "anthropic", Model: "sonnet"},
		{Provider: "anthropic", Model: "haiku"},
	}, result)
}

func TestRuntimeWithoutARegisteredBinaryListsNoModels(t *testing.T) {
	adapter := NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{models.RuntimePi: "/no/existe"})

	result, err := adapter.List(context.Background(), models.RuntimeGentleShell)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result)
}

func TestConstructorCopiesTheBinaryMap(t *testing.T) {
	bins := map[models.AgentRuntime]string{models.RuntimePi: "/no/existe"}
	adapter := NewAgentCLIModelCatalogAdapter(bins)
	bins[models.RuntimePi] = "changed-after-construction"

	require.Equal(t, "/no/existe", adapter.bins[models.RuntimePi])
}

func TestListModelsCommandRunsTheBinaryInALoginShellWithACleanEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX login shell")
	}
	vars := []string{"SHELL=/bin/zsh", "HOME=/home/u", "APP_ENV=dev"}

	command := listModelsCommand(context.Background(), "my pi", vars)

	require.Equal(t, "/bin/zsh", command.Path)
	require.Equal(t, []string{"/bin/zsh", "-l", "-c", "exec 'my pi' '--list-models'"}, command.Args)
	require.Equal(t, []string{"SHELL=/bin/zsh", "HOME=/home/u", "PATH=" + defaultPOSIXPath}, command.Env)
}

// fakeAgent writes an executable script that behaves like an agent CLI with the given body.
func fakeAgent(t *testing.T, body string) string {
	t.Helper()
	return writeExecutable(t, "#!/bin/sh\n"+body+"\n")
}

const listTableBody = "echo '" + tableHeader + "'\necho 'anthropic claude-5 200000 8192 yes yes'"

func TestListsModelsThroughTheLoginShellWithoutParentVariables(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	t.Setenv("IWF_TEST_PARENT_ONLY", "leaked")
	bin := fakeAgent(t, listTableBody+"\nif [ -n \"$IWF_TEST_PARENT_ONLY\" ]; then echo 'leak leaked 1 1 no no'; fi")
	adapter := bothRuntimes(bin, bin)

	result, err := adapter.List(context.Background(), models.RuntimePi)

	require.NoError(t, err)
	require.Equal(t, []models.ModelRef{{Provider: "anthropic", Model: "claude-5"}}, result)
}

func TestFailingBinaryIsACatalogErrorWithItsStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := fakeAgent(t, "echo boom >&2; exit 2")
	adapter := bothRuntimes(bin, bin)

	_, err := adapter.List(context.Background(), models.RuntimeGentleShell)

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
	require.Contains(t, err.Error(), "boom")
}

func TestRepeatedListingReusesTheCachedModelsInsteadOfSpawningAgain(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	counter := filepath.Join(t.TempDir(), "runs")
	bin := fakeAgent(t, "echo run >> '"+counter+"'\n"+listTableBody)
	adapter := bothRuntimes(bin, bin)

	first, err := adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)
	second, err := adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)

	require.Equal(t, first, second)
	runs, err := os.ReadFile(counter)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(runs), "run"))
}

func TestCachedListingIsIsolatedFromCallerMutations(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := fakeAgent(t, listTableBody)
	adapter := bothRuntimes(bin, bin)

	first, err := adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)
	first[0].Model = "mutated"
	second, err := adapter.List(context.Background(), models.RuntimePi)

	require.NoError(t, err)
	require.Equal(t, "claude-5", second[0].Model)
}

func TestCacheEntryExpiresAfterTheTTL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	counter := filepath.Join(t.TempDir(), "runs")
	bin := fakeAgent(t, "echo run >> '"+counter+"'\n"+listTableBody)
	adapter := bothRuntimes(bin, bin)
	now := time.Now()
	adapter.clock = func() time.Time { return now }
	_, err := adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)

	now = now.Add(modelsCacheTTL - time.Second)
	_, err = adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)
	now = now.Add(2 * time.Second)
	_, err = adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)

	runs, err := os.ReadFile(counter)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(runs), "run"))
}

func TestFailuresAreNotCached(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	counter := filepath.Join(t.TempDir(), "runs")
	bin := fakeAgent(t, "echo run >> '"+counter+"'; echo boom >&2; exit 2")
	adapter := bothRuntimes(bin, bin)

	_, firstErr := adapter.List(context.Background(), models.RuntimePi)
	_, secondErr := adapter.List(context.Background(), models.RuntimePi)

	require.Error(t, firstErr)
	require.Error(t, secondErr)
	runs, err := os.ReadFile(counter)
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(runs), "run"))
}

func TestEachRuntimeHasItsOwnCacheEntry(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	shell := fakeAgent(t, listTableBody)
	pi := fakeAgent(t, "echo '"+tableHeader+"'\necho 'openai gpt-5 128000 4096 no yes'")
	adapter := bothRuntimes(shell, pi)

	gentleShellModels, err := adapter.List(context.Background(), models.RuntimeGentleShell)
	require.NoError(t, err)
	piModels, err := adapter.List(context.Background(), models.RuntimePi)
	require.NoError(t, err)

	require.Equal(t, "anthropic", gentleShellModels[0].Provider)
	require.Equal(t, "openai", piModels[0].Provider)
}

func TestSlowListingIsStoppedAtTheTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := fakeAgent(t, "sleep 30")
	adapter := bothRuntimes(bin, bin)
	adapter.timeout = 200 * time.Millisecond

	_, err := adapter.List(context.Background(), models.RuntimePi)

	require.EqualError(t, err, "error de catálogo: "+timeoutMessage)
}

func TestParseModelTableSkipsTheHeader(t *testing.T) {
	text := tableHeader + "\n" +
		"anthropic claude-5 200000 8192 yes yes\n" +
		"openai gpt-5 128000 4096 no yes\n"

	require.Equal(t, []models.ModelRef{
		{Provider: "anthropic", Model: "claude-5"},
		{Provider: "openai", Model: "gpt-5"},
	}, parseModelTable(text))
}

func TestParseModelTableOfHeaderOnlyOrEmptyInputIsEmpty(t *testing.T) {
	require.Empty(t, parseModelTable(tableHeader+"\n"))
	require.NotNil(t, parseModelTable(""))
	require.Empty(t, parseModelTable(""))
}

func TestParseModelTableIgnoresBlankLinesAndShortRows(t *testing.T) {
	text := tableHeader + "\n\nanthropic\nanthropic claude-5 200000 8192 yes yes\n"

	require.Equal(t, []models.ModelRef{{Provider: "anthropic", Model: "claude-5"}}, parseModelTable(text))
}

func TestParseModelTableAcceptsCRLFLineEndings(t *testing.T) {
	text := tableHeader + "\r\nanthropic claude-5 200000 8192 yes yes\r\n"

	require.Equal(t, []models.ModelRef{{Provider: "anthropic", Model: "claude-5"}}, parseModelTable(text))
}

func TestSplitLinesFollowsRustLinesSemantics(t *testing.T) {
	require.Nil(t, splitLines(""))
	require.Equal(t, []string{""}, splitLines("\n"))
	require.Equal(t, []string{"a", ""}, splitLines("a\n\n"))
	require.Equal(t, []string{"a"}, splitLines("a"))
	require.Equal(t, []string{"a", "b"}, splitLines("a\r\nb\r\n"))
}
