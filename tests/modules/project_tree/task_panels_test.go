package projecttree_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/project_tree/infrastructure/di"
)

// renderPanel renders a panel to its markup in the default language.
func renderPanel(t *testing.T, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &out))
	return out.String()
}

// TestTaskPanelsServeTheTaskScopeThroughTheModuleUseCases builds the handlers the way the composition root hands them
// to the tasks module: from the use cases the module exposes. The tree of a task links its files to the task tabs.
func TestTaskPanelsServeTheTaskScopeThroughTheModuleUseCases(t *testing.T) {
	taskDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.txt"), []byte("x"), 0o600))
	taskID := uuid.New()
	module := di.NewModule(di.Deps{
		Workspaces: mapLookup{},
		Tasks:      mapLookup{taskID: {Name: "task", Path: taskDir}},
	})
	panels := inbound.NewHandlers(inbound.UseCases{
		ListDirectory: module.ListDirectory,
		ReadFile:      module.ReadFile,
		WriteFile:     module.WriteFile,
		GitChanges:    module.GitChanges,
		FileBase:      module.FileBase,
		StagePaths:    module.StagePaths,
		UnstagePaths:  module.UnstagePaths,
		CommitChanges: module.CommitChanges,
		SearchFiles:   module.SearchFiles,
	})
	ctx := context.Background()
	wire := models.TaskScope(taskID).Wire()

	tree := renderPanel(t, panels.TreePanel(ctx, wire))
	assert.Contains(t, tree, "task.txt")
	assert.Contains(t, tree, "/tasks/"+taskID.String()+"/files/task.txt", "a file of a task opens in the task tab")
	assert.NotContains(t, tree, "<h1", "the panel is the tree alone; the page adds the header")

	page := renderPanel(t, panels.RenderTree(ctx, wire))
	assert.Contains(t, page, "<h1", "the Archivos dock tab is the full page")

	changes := renderPanel(t, panels.ChangesPanel(ctx, wire))
	assert.Contains(t, changes, `data-testid="git-changes"`)
}

// TestTaskPanelsAnswerAnInvalidScopeWithItsMessage keeps a malformed scope out of the panels: it is shown, not served.
func TestTaskPanelsAnswerAnInvalidScopeWithItsMessage(t *testing.T) {
	module := di.NewModule(di.Deps{Workspaces: mapLookup{}, Tasks: mapLookup{}})
	panels := inbound.NewHandlers(inbound.UseCases{ListDirectory: module.ListDirectory, GitChanges: module.GitChanges})
	ctx := context.Background()

	changes := renderPanel(t, panels.ChangesPanel(ctx, "not-a-scope"))
	tree := renderPanel(t, panels.TreePanel(ctx, "not-a-scope"))

	assert.NotEmpty(t, changes)
	assert.False(t, strings.Contains(changes, `data-testid="git-changes"`))
	assert.False(t, strings.Contains(tree, "<h1"))
}
