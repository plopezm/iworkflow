package inbound_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	httpinbound "iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
)

type sidebarWorkspaces struct{ refs []models.WorkspaceRef }

func (f sidebarWorkspaces) Find(context.Context, string) (*models.WorkspaceRef, error) {
	return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
}

func (f sidebarWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) { return f.refs, nil }

// sidebarTasks has no unfinished task: the sidebar only lists the workspaces here.
type sidebarTasks struct{ outbound.TaskRepository }

func (sidebarTasks) ListActive(context.Context, *string) ([]models.Task, error) { return nil, nil }

type sidebarArranger struct {
	layout models.SidebarLayout
	err    error
	ids    []string
}

func (f *sidebarArranger) Arrange(_ context.Context, ids []string) (models.SidebarLayout, error) {
	f.ids = ids
	return f.layout, f.err
}

func sidebarHTML(t *testing.T, uc httpinbound.BoardUseCases) string {
	t.Helper()
	uc.List = application.NewListTasks(sidebarTasks{})
	uc.Workspaces = sidebarWorkspaces{refs: []models.WorkspaceRef{{ID: "w1", Name: "api"}, {ID: "w2", Name: "web"}}}
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, httpinbound.NewBoardHandlers(uc, time.Now).Sidebar(ctx).Render(ctx, &out))
	return out.String()
}

func TestBoardSidebarWithoutAnArrangerListsEveryWorkspaceFlat(t *testing.T) {
	t.Parallel()

	html := sidebarHTML(t, httpinbound.BoardUseCases{})

	assert.Contains(t, html, `data-workspace-row="w1"`)
	assert.Contains(t, html, `data-workspace-row="w2"`)
	assert.NotContains(t, html, `data-group-id`)
}

func TestBoardSidebarRendersTheGroupsTheArrangerReturns(t *testing.T) {
	t.Parallel()
	arranger := &sidebarArranger{layout: models.SidebarLayout{
		Groups:    []models.LayoutGroup{{ID: "g1", Name: "Backend", Workspaces: []string{"w1"}}},
		Ungrouped: []string{"w2"},
	}}

	html := sidebarHTML(t, httpinbound.BoardUseCases{SidebarArranger: arranger})

	assert.Equal(t, []string{"w1", "w2"}, arranger.ids, "the arranger receives the workspace ids in the sidebar order")
	assert.Contains(t, html, `data-group-id="g1"`)
	assert.Contains(t, html, `data-workspace-row="w1"`)
	assert.Contains(t, html, `data-workspace-row="w2"`)
}

func TestBoardSidebarFallsBackToTheFlatListWhenArrangingFails(t *testing.T) {
	t.Parallel()
	arranger := &sidebarArranger{err: errors.New("database is locked")}

	html := sidebarHTML(t, httpinbound.BoardUseCases{SidebarArranger: arranger})

	assert.NotContains(t, html, `data-group-id`)
	assert.Equal(t, 1, strings.Count(html, `data-workspace-row="w1"`))
	assert.Equal(t, 1, strings.Count(html, `data-workspace-row="w2"`), "every workspace stays listed when the layout cannot be read")
}
