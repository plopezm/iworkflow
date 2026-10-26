package state_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Port of the view helpers of src/modules/tasks/presentation/state/diff_view_state.rs. The editor helpers
// (save outcome, draft, dirty) belong to the file editor and are ported with it.

func langContext(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

// diffTestFile parses a one-file patch with the given header lines and returns the file.
func diffTestFile(t *testing.T, statusHeader string, path string, binary bool) state.FileDiff {
	t.Helper()
	body := "--- a/" + path + "\n+++ b/" + path + "\n@@ -1 +1 @@\n-a\n+b\n"
	if binary {
		body = "Binary files a/x and b/x differ\n"
	}
	files := state.ParsePatch("diff --git a/" + path + " b/" + path + "\n" + statusHeader + body)
	if len(files) != 1 {
		t.Fatalf("expected one file, got %d", len(files))
	}
	return files[0]
}

func TestViewModeTogglesAndLabels(t *testing.T) {
	t.Parallel()
	assert.Equal(t, state.ViewSplit, state.ParseViewMode(""))
	assert.Equal(t, state.ViewSplit, state.ViewSplit.Toggled().Toggled())
	assert.Equal(t, state.ViewUnified, state.ViewSplit.Toggled())
	assert.Equal(t, state.ViewSplit, state.ViewUnified.Toggled())
	assert.Equal(t, "Split", state.ViewSplit.Label(langContext(i18n.En)))
	assert.Equal(t, "Unified", state.ViewUnified.Label(langContext(i18n.En)))
	assert.Equal(t, "Dividido", state.ViewSplit.Label(langContext(i18n.Es)))
	assert.Equal(t, "Unificado", state.ViewUnified.Label(langContext(i18n.Es)))
}

func TestParseViewModeReadsTheQueryValueAndDefaultsToSplit(t *testing.T) {
	t.Parallel()
	assert.Equal(t, state.ViewUnified, state.ParseViewMode("unified"))
	assert.Equal(t, state.ViewSplit, state.ParseViewMode("split"))
	assert.Equal(t, state.ViewSplit, state.ParseViewMode("bogus"))
}

func TestAnchorIDIsDOMSafeAndInjective(t *testing.T) {
	t.Parallel()
	first := state.AnchorID("src/a-b.rs")
	second := state.AnchorID("src/a/b.rs")
	assert.NotEqual(t, first, second)
	assert.True(t, strings.HasPrefix(first, "diff-file-"))
	for _, char := range first {
		isSafe := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_'
		assert.True(t, isSafe, "unsafe character %q in %q", char, first)
	}
	assert.Equal(t, state.AnchorID("ñ.rs"), state.AnchorID("ñ.rs"))
}

func TestDeletedBinaryAndReadonlyFilesAreNotEditable(t *testing.T) {
	t.Parallel()
	modified := diffTestFile(t, "", "a.rs", false)
	added := diffTestFile(t, "new file mode 100644\n", "b.rs", false)
	deleted := diffTestFile(t, "deleted file mode 100644\n", "c.rs", false)
	binary := diffTestFile(t, "", "d.png", true)
	assert.True(t, state.CanEditFile(modified, true))
	assert.True(t, state.CanEditFile(added, true))
	assert.False(t, state.CanEditFile(deleted, true))
	assert.False(t, state.CanEditFile(binary, true))
	assert.False(t, state.CanEditFile(modified, false))
}
