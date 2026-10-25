package state_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestScopedNamesAnElementAfterItsInstance(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "board-list-k3", state.Scoped("board-list", "k3"))
	assert.Equal(t, "board-list", state.Scoped("board-list", ""), "a request without a scope keeps the plain name")
}

func TestSanitizeScopeKeepsOnlyTheShapeOfAScope(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "k3x9", state.SanitizeScope("k3x9"))
	for _, raw := range []string{"", "a b", "x\"y", "#x", "UPPER", "a-b", "a&b=c", strings.Repeat("a", 33)} {
		assert.Empty(t, state.SanitizeScope(raw), "scope %q must be dropped", raw)
	}
}

func TestScopedPathCarriesTheScopeOnlyWhenThereIsOne(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/board/tasks/new?scope=k3", state.ScopedPath("/board/tasks/new", "k3"))
	assert.Equal(t, "/board/tasks/new", state.ScopedPath("/board/tasks/new", ""))
}
