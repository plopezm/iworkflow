package inbound_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspace_groups/application"
	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound"
)

// stubRepo answers the reads the screens need and records deletions. It is the only part of the repository used here.
type stubRepo struct {
	groups  []models.Group
	members map[string]string
	failing error
	deleted *[]string
}

func (s stubRepo) ListGroups(context.Context) ([]models.Group, error) { return s.groups, s.failing }
func (s stubRepo) Memberships(context.Context) (map[string]string, error) {
	return s.members, s.failing
}
func (stubRepo) Pinned(context.Context) ([]string, error)         { return nil, nil }
func (stubRepo) SaveGroup(context.Context, models.Group) error    { return nil }
func (stubRepo) UpdateGroup(context.Context, models.Group) error  { return nil }
func (stubRepo) SetCollapsed(context.Context, string, bool) error { return nil }
func (stubRepo) Assign(context.Context, string, string) error     { return nil }
func (stubRepo) Unassign(context.Context, string) error           { return nil }
func (stubRepo) Pin(context.Context, string) error                { return nil }
func (stubRepo) Unpin(context.Context, string) error              { return nil }
func (s stubRepo) DeleteGroup(_ context.Context, id string) error {
	if s.deleted != nil {
		*s.deleted = append(*s.deleted, id)
	}
	return nil
}

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

func handlersOver(repo stubRepo, catalog stubCatalog) *inbound.Handlers {
	return inbound.NewHandlers(inbound.UseCases{
		Assignments: application.NewListAssignments(repo),
		Delete:      application.NewDeleteGroup(repo),
		Catalog:     catalog,
	})
}

func request(method string, target string, handlers *inbound.Handlers) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(url.Values{}.Encode()))
	req = req.WithContext(i18n.WithState(req.Context(), i18n.State{Locale: i18n.En, Chosen: true}))
	recorder := httptest.NewRecorder()
	chiRouter(handlers).ServeHTTP(recorder, req)
	return recorder
}

func fixture(t *testing.T) (stubRepo, stubCatalog) {
	repo := stubRepo{
		groups:  []models.Group{group(t, "g1", "Backend", "teal"), group(t, "g2", "Docs", "")},
		members: map[string]string{"a": "g1", "c": "g2"},
	}
	catalog := stubCatalog{list: []inbound.WorkspaceInfo{{ID: "a", Name: "alpha", Color: "red"}, {ID: "b", Name: "beta"}, {ID: "c", Name: "gamma"}}}
	return repo, catalog
}

func TestListShowsEveryGroupWithItsCountAndALinkToItsOwnTabAndNothingToEdit(t *testing.T) {
	repo, catalog := fixture(t)

	response := request(http.MethodGet, "/workspace-groups/list", handlersOver(repo, catalog))

	require.Equal(t, http.StatusOK, response.Code)
	body := response.Body.String()
	assert.Contains(t, body, `data-group-id="g1"`)
	assert.Contains(t, body, `data-group-id="g2"`)
	assert.Contains(t, body, `data-dock-path="/workspaces/groups/g1"`)
	assert.Contains(t, body, "1 workspaces")
	assert.NotContains(t, body, "<input", "the list edits nothing")
	assert.NotContains(t, body, "data-available-workspace", "workspaces are not assigned from the groups screens")
}

func TestListInvitesToCreateTheFirstGroupWhenThereAreNone(t *testing.T) {
	_, catalog := fixture(t)

	body := request(http.MethodGet, "/workspace-groups/list", handlersOver(stubRepo{}, catalog)).Body.String()

	assert.Contains(t, body, "No groups yet")
}

func TestMembersListsOnlyTheWorkspacesOfTheGroupWithARemoveButton(t *testing.T) {
	repo, catalog := fixture(t)

	body := request(http.MethodGet, "/workspace-groups/g1/members", handlersOver(repo, catalog)).Body.String()

	assert.Contains(t, body, `data-assigned-workspace="a"`)
	assert.NotContains(t, body, `data-assigned-workspace="b"`)
	assert.NotContains(t, body, `data-assigned-workspace="c"`)
	assert.Contains(t, body, `hx-post="/workspace-groups/workspaces/a/group"`)
	assert.Contains(t, body, "Remove from group")
	assert.NotContains(t, body, "Assign<", "the group screen never assigns")
}

func TestMembersOfAnEmptyGroupPointsToTheWorkspaceEditScreen(t *testing.T) {
	repo, catalog := fixture(t)
	repo.members = map[string]string{}

	body := request(http.MethodGet, "/workspace-groups/g1/members", handlersOver(repo, catalog)).Body.String()

	assert.Contains(t, body, "This group has no workspaces")
}

func TestMembersOfADeletedGroupAnswerANoticeSoTheOpenTabRefreshesWithoutErrors(t *testing.T) {
	repo, catalog := fixture(t)

	response := request(http.MethodGet, "/workspace-groups/nope/members", handlersOver(repo, catalog))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "data-group-gone")
}

func TestScreensFailWhenTheWorkspacesOrTheGroupsCannotBeRead(t *testing.T) {
	repo, catalog := fixture(t)
	catalog.err = errors.New("boom")
	assert.Equal(t, http.StatusInternalServerError, request(http.MethodGet, "/workspace-groups/list", handlersOver(repo, catalog)).Code)

	repo, catalog = fixture(t)
	repo.failing = errors.New("boom")
	assert.Equal(t, http.StatusInternalServerError, request(http.MethodGet, "/workspace-groups/g1/members", handlersOver(repo, catalog)).Code)
}

func TestDeleteAndCloseDeletesTheGroupRefreshesTheSidebarAndSendsTheTabToTheList(t *testing.T) {
	repo, catalog := fixture(t)
	deleted := []string{}
	repo.deleted = &deleted

	response := request(http.MethodPost, "/workspace-groups/g1/delete-and-close", handlersOver(repo, catalog))

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, []string{"g1"}, deleted)
	assert.Equal(t, inbound.RefreshEvent, response.Header().Get("HX-Trigger"))
	assert.Contains(t, response.Body.String(), `data-dock-redirect="/workspaces/groups"`)
}

func TestDetailTabShowsItsNameAndColorAndTheBreadcrumbBackToTheList(t *testing.T) {
	repo, catalog := fixture(t)
	handlers := handlersOver(repo, catalog)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})

	var out strings.Builder
	require.NoError(t, handlers.DetailTab(ctx, dock.Params{"id": "g1"}).Render(ctx, &out))
	body := out.String()

	assert.Contains(t, body, `value="Backend"`)
	assert.Contains(t, body, `value="teal" checked`)
	assert.Contains(t, body, `data-dock-path="/workspaces"`)
	assert.Contains(t, body, `data-dock-path="/workspaces/groups"`)
	assert.Contains(t, body, `hx-get="/workspace-groups/g1/members"`)
	assert.Contains(t, body, `hx-swap="morph:innerHTML"`, "a self-refreshing fragment morphs")
	assert.Equal(t, "Backend", handlers.DetailTitle(ctx, dock.Params{"id": "g1"}))
	assert.Equal(t, "Groups", handlers.DetailTitle(ctx, dock.Params{"id": "nope"}))
}

func TestDetailTabOfAnUnknownGroupShowsItsMessage(t *testing.T) {
	repo, catalog := fixture(t)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})

	var out strings.Builder
	require.NoError(t, handlersOver(repo, catalog).DetailTab(ctx, dock.Params{"id": "nope"}).Render(ctx, &out))

	assert.Contains(t, out.String(), "This group no longer exists.")
}

func TestListTabHasTheCreateFormAndAMorphingList(t *testing.T) {
	repo, catalog := fixture(t)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})

	var out strings.Builder
	require.NoError(t, handlersOver(repo, catalog).ListTab(ctx, dock.Params{}).Render(ctx, &out))
	body := out.String()

	assert.Contains(t, body, `hx-post="/workspace-groups"`)
	assert.Contains(t, body, "data-group-create")
	assert.Contains(t, body, `hx-get="/workspace-groups/list"`)
	assert.Contains(t, body, `hx-swap="morph:innerHTML"`)
	assert.Contains(t, body, `hx-sync="this:replace"`)
	assert.NotContains(t, body, "data-assigned-workspace")
}
