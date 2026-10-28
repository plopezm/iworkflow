package jira_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

type boardFixture struct {
	t        *testing.T
	router   chi.Router
	module   *di.Module
	fakeJira *fakeJira
}

func newBoardFixture(t *testing.T) *boardFixture {
	t.Helper()
	fake := newFakeJira(t)
	db := dbtest.New(t)
	module := di.NewModule(di.Deps{
		DB:         db,
		Tasks:      &stubTasks{},
		Workspaces: stubWorkspaces{},
	})

	router := chi.NewRouter()
	module.Routes(router)

	return &boardFixture{
		t:        t,
		router:   router,
		module:   module,
		fakeJira: fake,
	}
}

func (f *boardFixture) serve(req *http.Request) *httptest.ResponseRecorder {
	state := i18n.State{Locale: i18n.Es, Chosen: true}
	reqWithLang := req.WithContext(i18n.WithState(req.Context(), state))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, reqWithLang)
	return rec
}

func (f *boardFixture) connect() {
	ctx := context.Background()
	_, err := f.module.SaveConnection.Execute(ctx, application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(f.t, err)
}

func (f *boardFixture) connectAndSync() {
	f.connect()
	_, err := f.module.SyncIssues.Execute(context.Background())
	require.NoError(f.t, err)
}

func TestBoardHandlers_Sync(t *testing.T) {
	f := newBoardFixture(t)
	f.connect()

	req := httptest.NewRequest(http.MethodPost, "/jira/sync", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `id="jira-board"`)
	assert.Contains(t, body, "Mis tareas")
	assert.Contains(t, body, "A-1")
	assert.Contains(t, body, "A-2")
	assert.Contains(t, body, "A-3")
}

func TestBoardHandlers_BoardFilter(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	// Filter by search for A-1
	req := httptest.NewRequest(http.MethodGet, "/jira/board?search=A-1", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)

	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "A-1")
	assert.NotContains(t, body, "A-2")
	assert.NotContains(t, body, "A-3")
	assert.Contains(t, body, "1 de 3")

	// Filter by status "To Do"
	req2 := httptest.NewRequest(http.MethodGet, "/jira/board?status=To+Do", nil)
	req2.Header.Set("HX-Request", "true")
	res2 := f.serve(req2)
	assert.Equal(t, http.StatusOK, res2.Code)
	assert.Contains(t, res2.Body.String(), "A-1")

	// Filter by non-existent status
	req3 := httptest.NewRequest(http.MethodGet, "/jira/board?status=Done", nil)
	req3.Header.Set("HX-Request", "true")
	res3 := f.serve(req3)
	assert.Equal(t, http.StatusOK, res3.Code)
	assert.Contains(t, res3.Body.String(), "Ninguna tarea de este carril coincide con los filtros.")
}

func TestBoardHandlers_LaneLifecycle(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	// 1. Open new lane form
	reqNew := httptest.NewRequest(http.MethodGet, "/jira/lanes/new", nil)
	reqNew.Header.Set("HX-Request", "true")
	resNew := f.serve(reqNew)
	assert.Equal(t, http.StatusOK, resNew.Code)
	assert.Contains(t, resNew.Body.String(), "+ Nuevo carril")
	assert.Contains(t, resNew.Body.String(), `name="name"`)
	assert.Contains(t, resNew.Body.String(), `name="jql"`)

	// 2. Cancel new lane form
	reqCancel := httptest.NewRequest(http.MethodGet, "/jira/lanes/cancel", nil)
	reqCancel.Header.Set("HX-Request", "true")
	resCancel := f.serve(reqCancel)
	assert.Equal(t, http.StatusOK, resCancel.Code)
	assert.NotContains(t, resCancel.Body.String(), `name="jql"`)

	// 3. Create lane
	form := url.Values{
		"name": {"Bugs Críticos"},
		"jql":  {"project = ABC AND type = Bug"},
	}
	reqCreate := httptest.NewRequest(http.MethodPost, "/jira/lanes", strings.NewReader(form.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreate.Header.Set("HX-Request", "true")
	resCreate := f.serve(reqCreate)
	assert.Equal(t, http.StatusOK, resCreate.Code)
	bodyCreate := resCreate.Body.String()
	assert.Contains(t, bodyCreate, "Bugs Críticos")

	// Get lane id
	listing, err := f.module.ListIssues.Execute(context.Background(), models.IssueFilter{})
	require.NoError(t, err)
	var customLaneID string
	for _, l := range listing.Lanes {
		if l.Lane.Name == "Bugs Críticos" {
			customLaneID = l.Lane.ID
			break
		}
	}
	require.NotEmpty(t, customLaneID)

	// 4. Edit lane form
	reqEdit := httptest.NewRequest(http.MethodGet, "/jira/lanes/"+customLaneID+"/edit", nil)
	reqEdit.Header.Set("HX-Request", "true")
	resEdit := f.serve(reqEdit)
	assert.Equal(t, http.StatusOK, resEdit.Code)
	assert.Contains(t, resEdit.Body.String(), "Editar carril")
	assert.Contains(t, resEdit.Body.String(), "Bugs Críticos")

	// 5. Update lane
	updateForm := url.Values{
		"name": {"Bugs Urgentes"},
		"jql":  {"project = ABC AND type = Bug"},
	}
	reqUpdate := httptest.NewRequest(http.MethodPost, "/jira/lanes/"+customLaneID, strings.NewReader(updateForm.Encode()))
	reqUpdate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqUpdate.Header.Set("HX-Request", "true")
	resUpdate := f.serve(reqUpdate)
	assert.Equal(t, http.StatusOK, resUpdate.Code)
	assert.Contains(t, resUpdate.Body.String(), "Bugs Urgentes")

	// 6. Confirm delete
	reqConfirm := httptest.NewRequest(http.MethodGet, "/jira/lanes/"+customLaneID+"/confirm-delete", nil)
	reqConfirm.Header.Set("HX-Request", "true")
	resConfirm := f.serve(reqConfirm)
	assert.Equal(t, http.StatusOK, resConfirm.Code)
	assert.Contains(t, resConfirm.Body.String(), "Se elimina el carril")
	assert.Contains(t, resConfirm.Body.String(), "Eliminar carril")

	// 7. Delete lane
	reqDelete := httptest.NewRequest(http.MethodPost, "/jira/lanes/"+customLaneID+"/delete", nil)
	reqDelete.Header.Set("HX-Request", "true")
	resDelete := f.serve(reqDelete)
	assert.Equal(t, http.StatusOK, resDelete.Code)
	assert.NotContains(t, resDelete.Body.String(), "Bugs Urgentes")
}

func TestBoardHandlers_ToggleCollapse(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	// Default lane is id="mine"
	req := httptest.NewRequest(http.MethodPost, "/jira/lanes/mine/collapse", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	// Toggled from collapsed:false to collapsed:true
	assert.Contains(t, body, `aria-expanded="false"`)
	assert.Contains(t, body, "grid-rows-[0fr]")

	// Toggle back
	req2 := httptest.NewRequest(http.MethodPost, "/jira/lanes/mine/collapse", nil)
	req2.Header.Set("HX-Request", "true")
	res2 := f.serve(req2)
	assert.Equal(t, http.StatusOK, res2.Code)
	body2 := res2.Body.String()
	assert.Contains(t, body2, `aria-expanded="true"`)
	assert.Contains(t, body2, "grid-rows-[1fr]")
}

func TestBoardHandlers_IgnoreIssue(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	// Ignore A-1
	req := httptest.NewRequest(http.MethodPost, "/jira/issues/A-1/ignore?ignored=true", nil)
	req.Header.Set("HX-Request", "true")
	res := f.serve(req)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, "Ignorada")
	assert.Contains(t, body, "Restaurar")

	// Restore A-1
	reqRestore := httptest.NewRequest(http.MethodPost, "/jira/issues/A-1/ignore?ignored=false", nil)
	reqRestore.Header.Set("HX-Request", "true")
	resRestore := f.serve(reqRestore)
	assert.Equal(t, http.StatusOK, resRestore.Code)
	bodyRestore := resRestore.Body.String()
	assert.Contains(t, bodyRestore, "Nueva")
	assert.Contains(t, bodyRestore, "Ignorar")
}

func TestBoardHandlers_Page(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	tabs := f.module.Tabs()
	require.Len(t, tabs, 3)
	assert.Equal(t, "jira", tabs[0].Name)
	assert.Equal(t, "jira_convert", tabs[1].Name)
	assert.Equal(t, "jira_issue", tabs[2].Name)

	comp := tabs[0].Render(context.Background(), nil)
	rec := httptest.NewRecorder()
	err := comp.Render(context.Background(), rec)
	require.NoError(t, err)

	html := rec.Body.String()
	assert.Contains(t, html, "Jira")
	assert.Contains(t, html, `id="jira-connection-card"`)
	assert.Contains(t, html, `id="jira-board"`)
	assert.Contains(t, html, "Mis tareas")
	assert.Contains(t, html, "A-1")
}

func TestBoardHandlers_InvalidLaneOperations(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	// 1. Create with empty values
	emptyForm := url.Values{
		"name": {"   "},
		"jql":  {""},
	}
	reqEmpty := httptest.NewRequest(http.MethodPost, "/jira/lanes", strings.NewReader(emptyForm.Encode()))
	reqEmpty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEmpty.Header.Set("HX-Request", "true")
	resEmpty := f.serve(reqEmpty)
	assert.Equal(t, http.StatusOK, resEmpty.Code)
	assert.Contains(t, resEmpty.Body.String(), "El nombre y la consulta JQL son obligatorios.")

	// 2. Edit non-existent lane
	reqNonExist := httptest.NewRequest(http.MethodGet, "/jira/lanes/non-existent/edit", nil)
	reqNonExist.Header.Set("HX-Request", "true")
	resNonExist := f.serve(reqNonExist)
	assert.Equal(t, http.StatusOK, resNonExist.Code)
	assert.Contains(t, resNonExist.Body.String(), "Carril no encontrado")

	// 3. Delete default lane (forbidden)
	reqDelDefault := httptest.NewRequest(http.MethodPost, "/jira/lanes/mine/delete", nil)
	reqDelDefault.Header.Set("HX-Request", "true")
	resDelDefault := f.serve(reqDelDefault)
	assert.Equal(t, http.StatusOK, resDelDefault.Code)
	assert.Contains(t, resDelDefault.Body.String(), "no se puede")

	// 4. Update default lane (forbidden)
	updateForm := url.Values{
		"name": {"Nuevo Nombre"},
		"jql":  {"project = ABC"},
	}
	reqUpdateDefault := httptest.NewRequest(http.MethodPost, "/jira/lanes/mine", strings.NewReader(updateForm.Encode()))
	reqUpdateDefault.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqUpdateDefault.Header.Set("HX-Request", "true")
	resUpdateDefault := f.serve(reqUpdateDefault)
	assert.Equal(t, http.StatusOK, resUpdateDefault.Code)
	assert.Contains(t, resUpdateDefault.Body.String(), "no se puede")
}

func TestBoardHandlers_Locales(t *testing.T) {
	f := newBoardFixture(t)
	f.connectAndSync()

	locales := []struct {
		loc          i18n.Locale
		expectedCard string
		expectedSync string
	}{
		{loc: i18n.En, expectedCard: "Imported issues", expectedSync: "Sync"},
		{loc: i18n.Es, expectedCard: "Tareas importadas", expectedSync: "Sincronizar"},
		{loc: i18n.Zh, expectedCard: "已导入的任务", expectedSync: "同步"},
		{loc: i18n.De, expectedCard: "Importierte Aufgaben", expectedSync: "Synchronisieren"},
		{loc: i18n.Fr, expectedCard: "Tâches importées", expectedSync: "Synchroniser"},
	}

	for _, tc := range locales {
		t.Run(string(tc.loc), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/jira/board", nil)
			req.Header.Set("HX-Request", "true")
			res := f.serveWithLocale(req, tc.loc)
			assert.Equal(t, http.StatusOK, res.Code)
			body := res.Body.String()
			assert.Contains(t, body, tc.expectedCard)
			assert.Contains(t, body, tc.expectedSync)
		})
	}
}

func (f *boardFixture) serveWithLocale(req *http.Request, loc i18n.Locale) *httptest.ResponseRecorder {
	state := i18n.State{Locale: loc, Chosen: true}
	reqWithLang := req.WithContext(i18n.WithState(req.Context(), state))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, reqWithLang)
	return rec
}
