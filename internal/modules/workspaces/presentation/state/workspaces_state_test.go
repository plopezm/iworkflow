package state

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entry(name string) DirectoryEntry {
	return DirectoryEntry{Name: name, Path: "/x/" + name}
}

func TestFilterEntriesIsCaseInsensitiveAndTrimmed(t *testing.T) {
	entries := []DirectoryEntry{entry("Alpha"), entry("beta")}

	assert.Equal(t, []DirectoryEntry{entry("Alpha")}, FilterEntries(entries, "  ALP "))
}

func TestFilterEntriesWithEmptyNeedleKeepsEverything(t *testing.T) {
	entries := []DirectoryEntry{entry("a"), entry("b")}

	assert.Len(t, FilterEntries(entries, ""), 2)
}

func TestFilterEntriesWithoutMatchesIsEmptyNotNil(t *testing.T) {
	filtered := FilterEntries([]DirectoryEntry{entry("a")}, "zzz")

	require.NotNil(t, filtered)
	assert.Empty(t, filtered)
}

func TestResubmitJSONCarriesTheAttemptAndForcesInitialize(t *testing.T) {
	prompt := InitializationPrompt{
		Path:         "/repos/demo",
		Reason:       ReasonNoCommits,
		Name:         "Demo",
		BaseBranch:   "dev",
		AgentsFileID: "abc",
	}

	var values map[string]string
	require.NoError(t, json.Unmarshal([]byte(prompt.ResubmitJSON()), &values))

	assert.Equal(t, map[string]string{
		"name":           "Demo",
		"path":           "/repos/demo",
		"base_branch":    "dev",
		"agents_file_id": "abc",
		"initialize":     "true",
	}, values)
}

func TestResubmitJSONCarriesTheChosenDefaultRuntime(t *testing.T) {
	prompt := InitializationPrompt{Path: "/repos/demo", Reason: ReasonMissingFolder, Name: "Demo", DefaultRuntime: "codex"}

	var values map[string]string
	require.NoError(t, json.Unmarshal([]byte(prompt.ResubmitJSON()), &values))

	assert.Equal(t, "codex", values["default_runtime"])
}

func TestResubmitJSONOmitsDefaultRuntimeWhenEmpty(t *testing.T) {
	prompt := InitializationPrompt{Path: "/repos/demo", Reason: ReasonMissingFolder, Name: "Demo"}

	var values map[string]string
	require.NoError(t, json.Unmarshal([]byte(prompt.ResubmitJSON()), &values))

	assert.NotContains(t, values, "default_runtime")
}

func TestResubmitJSONCarriesTheChosenColorOnlyWhenThereIsOne(t *testing.T) {
	with := InitializationPrompt{Name: "a", Path: "/p", Color: "teal"}.ResubmitJSON()
	assert.Contains(t, with, `"color":"teal"`)

	without := InitializationPrompt{Name: "a", Path: "/p"}.ResubmitJSON()
	assert.NotContains(t, without, "color")
}
