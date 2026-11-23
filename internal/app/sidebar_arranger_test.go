package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	wgmodels "iworkflow/internal/modules/workspace_groups/domain/models"
)

type fakeLayouts struct {
	layout wgmodels.Layout
	err    error
	ids    []string
}

func (f *fakeLayouts) Execute(_ context.Context, ids []string) (wgmodels.Layout, error) {
	f.ids = ids
	return f.layout, f.err
}

func TestSidebarArrangerMapsTheGroupsLayoutToTheTasksLayout(t *testing.T) {
	t.Parallel()
	group, err := wgmodels.NewGroup("g1", "Backend", "blue", true)
	require.NoError(t, err)
	layouts := &fakeLayouts{layout: wgmodels.Layout{
		Pinned:    []string{"p"},
		Groups:    []wgmodels.GroupMembers{{Group: group, Workspaces: []string{"w1"}}},
		Ungrouped: []string{"u"},
	}}

	got, err := (&sidebarArranger{layouts: layouts}).Arrange(context.Background(), []string{"p", "w1", "u"})

	require.NoError(t, err)
	assert.Equal(t, []string{"p", "w1", "u"}, layouts.ids, "the workspace ids reach the groups module in the sidebar order")
	assert.Equal(t, tasksmodels.SidebarLayout{
		Pinned:    []string{"p"},
		Groups:    []tasksmodels.LayoutGroup{{ID: "g1", Name: "Backend", Color: "blue", Collapsed: true, Workspaces: []string{"w1"}}},
		Ungrouped: []string{"u"},
	}, got)
}

func TestSidebarArrangerWrapsTheLayoutError(t *testing.T) {
	t.Parallel()
	boom := errors.New("database is locked")

	_, err := (&sidebarArranger{layouts: &fakeLayouts{err: boom}}).Arrange(context.Background(), nil)

	require.ErrorIs(t, err, boom)
}
