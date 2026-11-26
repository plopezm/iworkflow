package inbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/application"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// fakeRegisterRepository keeps the saved workspaces and finds none by path, so every register is new.
type fakeRegisterRepository struct {
	outbound.WorkspaceRepository
	saved []models.Workspace
}

func (r *fakeRegisterRepository) FindByPath(context.Context, string) (*models.Workspace, error) {
	return nil, nil //nolint:nilnil // nil is "no workspace with this path"
}

func (r *fakeRegisterRepository) Save(_ context.Context, workspace models.Workspace) error {
	r.saved = append(r.saved, workspace)
	return nil
}

// fakeRegisterGit reports every folder as a repository with commits on main.
type fakeRegisterGit struct {
	outbound.GitRepository
}

func (fakeRegisterGit) Inspect(_ context.Context, path string) (models.GitRepositoryInfo, error) {
	return models.GitRepositoryInfo{Root: path, CurrentBranch: "main", HasCommits: true}, nil
}

// fakeRegisterRuntimes and fakeRegisterAgents offer no agents, runtimes or agents files.
type fakeRegisterRuntimes struct{}

func (fakeRegisterRuntimes) Installed(context.Context) []string { return nil }

type fakeRegisterAgents struct {
	outbound.AgentsFiles
}

func (fakeRegisterAgents) ListOptions(context.Context) ([]models.AgentsFileOption, error) {
	return nil, nil
}

// fakeGroupDirectory lists the given groups and records every assignment, failing with assignErr when set.
type fakeGroupDirectory struct {
	groups    []GroupInfo
	assignErr error
	assigned  [][2]string
}

func (g *fakeGroupDirectory) Directory(context.Context) ([]GroupInfo, map[string]string, error) {
	return g.groups, map[string]string{}, nil
}

func (g *fakeGroupDirectory) Assign(_ context.Context, workspaceID string, groupID string) error {
	g.assigned = append(g.assigned, [2]string{workspaceID, groupID})
	return g.assignErr
}

// registerHandlers builds the handlers over fakes, with the group directory when given.
func registerHandlers(t *testing.T, groups GroupDirectory) (*Handlers, *fakeRegisterRepository) {
	t.Helper()
	repo := &fakeRegisterRepository{}
	runtimes := fakeRegisterRuntimes{}
	agents := fakeRegisterAgents{}
	id, err := models.NewWorkspaceID()
	require.NoError(t, err)
	clock := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	newID := func() (models.WorkspaceID, error) { return id, nil }
	handlers := NewHandlers(UseCases{
		Register:          application.NewRegisterWorkspace(repo, fakeRegisterGit{}, agents, runtimes, clock, newID),
		ListAgentRuntimes: application.NewListAgentRuntimes(runtimes),
		ListAgentsFiles:   application.NewListAgentsFileOptions(agents),
		Groups:            groups,
	})
	return handlers, repo
}

// postRegister posts the register form in Spanish, the language the messages are asserted in.
func postRegister(t *testing.T, handlers *Handlers, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	handlers.Mount(router)
	request := httptest.NewRequest(http.MethodPost, "/workspaces/register", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: i18n.Es, Chosen: true}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	return rec
}

func TestRegisterWithGroupAssignsTheNewWorkspaceOnlyWhenAGroupIsChosen(t *testing.T) {
	cases := []struct {
		name        string
		groupID     string
		wantAssigns bool
	}{
		{name: "chosen group assigns the new workspace", groupID: "g1", wantAssigns: true},
		{name: "no group leaves the workspace ungrouped", groupID: "", wantAssigns: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			groups := &fakeGroupDirectory{groups: []GroupInfo{{ID: "g1", Name: "Backend"}}}
			handlers, repo := registerHandlers(t, groups)
			values := url.Values{"name": {"demo"}, "path": {t.TempDir()}}
			if tc.groupID != "" {
				values.Set("group_id", tc.groupID)
			}

			rec := postRegister(t, handlers, values)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Len(t, repo.saved, 1, "the workspace is registered")
			assert.NotContains(t, rec.Body.String(), "no se pudo añadir al grupo")
			if !tc.wantAssigns {
				assert.Empty(t, groups.assigned, "Assign is not called without a group")
				return
			}
			assert.Equal(t, [][2]string{{repo.saved[0].ID.String(), tc.groupID}}, groups.assigned)
		})
	}
}

func TestRegisterReportsThatTheWorkspaceExistsWhenTheGroupAssignFails(t *testing.T) {
	groups := &fakeGroupDirectory{
		groups:    []GroupInfo{{ID: "g1", Name: "Backend"}},
		assignErr: assert.AnError,
	}
	handlers, repo := registerHandlers(t, groups)

	rec := postRegister(t, handlers, url.Values{"name": {"demo"}, "path": {t.TempDir()}, "group_id": {"g1"}})

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, repo.saved, 1, "the workspace stays registered")
	require.Len(t, groups.assigned, 1)
	assert.Contains(t, rec.Body.String(), "El workspace se registró pero no se pudo añadir al grupo")
}

func getRegisterGroups(t *testing.T, handlers *Handlers, target string) string {
	t.Helper()
	router := chi.NewRouter()
	handlers.Mount(router)
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request = request.WithContext(i18n.WithState(request.Context(), i18n.State{Locale: i18n.Es, Chosen: true}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func TestTheRegisterFormPageDoesNotReadTheGroupsAndTheGroupsEndpointDoes(t *testing.T) {
	groups := &fakeGroupDirectory{groups: []GroupInfo{{ID: "g1", Name: "Backend"}, {ID: "g2", Name: "Family"}}}
	handlers, _ := registerHandlers(t, groups)

	body := getRegisterGroups(t, handlers, "/workspaces/register/groups?group_id=g2")

	assert.Contains(t, body, `data-register-group`)
	assert.Contains(t, body, "Backend")
	assert.Contains(t, body, "Family")
	assert.Regexp(t, `value="g2"[^>]*selected`, body, "the chosen group comes back selected")
}

func TestTheGroupsEndpointIsEmptyWithoutGroupsOrDirectory(t *testing.T) {
	for name, directory := range map[string]GroupDirectory{"no groups": &fakeGroupDirectory{}, "no directory": nil} {
		t.Run(name, func(t *testing.T) {
			handlers, _ := registerHandlers(t, directory)

			body := getRegisterGroups(t, handlers, "/workspaces/register/groups")

			assert.NotContains(t, body, `name="group_id"`)
		})
	}
}
