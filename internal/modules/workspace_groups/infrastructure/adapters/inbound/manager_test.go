package inbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspace_groups/application"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound"
)

// stubRepo answers the reads the card needs: groups, memberships. It is the only part of the repository used here.
type stubRepo struct {
	groups  []models.Group
	members map[string]string
	pins    []string
	failing error
}

func (s stubRepo) ListGroups(context.Context) ([]models.Group, error) { return s.groups, s.failing }
func (s stubRepo) Memberships(context.Context) (map[string]string, error) {
	return s.members, s.failing
}
func (s stubRepo) Pinned(context.Context) ([]string, error)       { return s.pins, nil }
func (stubRepo) SaveGroup(context.Context, models.Group) error    { return nil }
func (stubRepo) UpdateGroup(context.Context, models.Group) error  { return nil }
func (stubRepo) DeleteGroup(context.Context, string) error        { return nil }
func (stubRepo) SetCollapsed(context.Context, string, bool) error { return nil }
func (stubRepo) Assign(context.Context, string, string) error     { return nil }
func (stubRepo) Unassign(context.Context, string) error           { return nil }
func (stubRepo) Pin(context.Context, string) error                { return nil }
func (stubRepo) Unpin(context.Context, string) error              { return nil }

type stubCatalog struct {
	list []inbound.WorkspaceInfo
	err  error
}

func (s stubCatalog) List(context.Context) ([]inbound.WorkspaceInfo, error) { return s.list, s.err }

func group(t *testing.T, id string, name string, color string) models.Group {
	t.Helper()
	built, err := models.NewGroup(id, name, color, false)
	require.NoError(t, err)
	return built
}

func serve(t *testing.T, repo stubRepo, catalog stubCatalog, target string) *httptest.ResponseRecorder {
	t.Helper()
	handlers := inbound.NewHandlers(inbound.UseCases{Assignments: application.NewListAssignments(repo), Catalog: catalog})
	router := chiRouter(handlers)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: i18n.En, Chosen: true}))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func fixture(t *testing.T) (stubRepo, stubCatalog) {
	repo := stubRepo{
		groups:  []models.Group{group(t, "g1", "Backend", "teal"), group(t, "g2", "Docs", "")},
		members: map[string]string{"a": "g1", "c": "g2"},
		pins:    []string{"a"},
	}
	catalog := stubCatalog{list: []inbound.WorkspaceInfo{{ID: "a", Name: "alpha", Color: "red"}, {ID: "b", Name: "beta"}, {ID: "c", Name: "gamma"}}}
	return repo, catalog
}

func TestManagerShowsEveryGroupWithItsCountAndColorAndNoAssignmentsByDefault(t *testing.T) {
	repo, catalog := fixture(t)

	response := serve(t, repo, catalog, "/workspace-groups/manage")

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `data-group-id="g1"`)
	assert.Contains(t, body, `data-group-id="g2"`)
	assert.Contains(t, body, `value="Backend"`)
	assert.Contains(t, body, `value="teal" checked`)
	assert.NotContains(t, body, "data-group-assignments", "assignments stay closed until asked")
}

func TestManagerOpenGroupListsItsMembersPinnedOnesIncludedAndTheOthersToAssign(t *testing.T) {
	repo, catalog := fixture(t)

	body := serve(t, repo, catalog, "/workspace-groups/manage?open=g1").Body.String()

	assert.Contains(t, body, `data-assigned-workspace="a"`, "a pinned workspace is still a member")
	assert.NotContains(t, body, `data-assigned-workspace="b"`)
	assert.Contains(t, body, `data-available-workspace="b"`)
	assert.Contains(t, body, `data-available-workspace="c"`, "a workspace of another group can be moved here")
	assert.Contains(t, body, "now in Docs")
	assert.NotContains(t, body, `data-available-workspace="a"`)
	assert.Contains(t, body, `name="open" value="g1"`, "a reload keeps the group open")
	assert.Equal(t, 1, countOf(body, "data-group-assignments"), "only the open group lists assignments")
}

func TestManagerIgnoresAnUnknownOpenGroup(t *testing.T) {
	repo, catalog := fixture(t)

	body := serve(t, repo, catalog, "/workspace-groups/manage?open=nope").Body.String()

	assert.NotContains(t, body, "data-group-assignments")
	assert.NotContains(t, body, `name="open"`)
}

func TestManagerFailsWhenTheWorkspacesOrTheGroupsCannotBeRead(t *testing.T) {
	repo, catalog := fixture(t)
	catalog.err = errors.New("boom")
	assert.Equal(t, http.StatusInternalServerError, serve(t, repo, catalog, "/workspace-groups/manage").Code)

	repo, catalog = fixture(t)
	repo.failing = errors.New("boom")
	assert.Equal(t, http.StatusInternalServerError, serve(t, repo, catalog, "/workspace-groups/manage").Code)
}
