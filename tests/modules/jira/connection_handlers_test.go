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
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound"
	"iworkflow/internal/modules/jira/infrastructure/di"
)

type connectionFixture struct {
	t        *testing.T
	router   chi.Router
	module   *di.Module
	fakeJira *fakeJira
	handlers *inbound.ConnectionHandlers
}

func newConnectionFixture(t *testing.T) *connectionFixture {
	t.Helper()
	fake := newFakeJira(t)
	db := dbtest.New(t)
	module := di.NewModule(di.Deps{
		DB:         db,
		Tasks:      &stubTasks{},
		Workspaces: stubWorkspaces{},
	})

	handlers := inbound.NewConnectionHandlers(
		module.GetConnection,
		module.SaveConnection,
		module.DeleteConnection,
	)

	router := chi.NewRouter()
	handlers.MountConnectionRoutes(router)

	return &connectionFixture{
		t:        t,
		router:   router,
		module:   module,
		fakeJira: fake,
		handlers: handlers,
	}
}

func (f *connectionFixture) serveWithLocale(req *http.Request, loc i18n.Locale) *httptest.ResponseRecorder {
	state := i18n.State{Locale: loc, Chosen: true}
	reqWithLang := req.WithContext(i18n.WithState(req.Context(), state))
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, reqWithLang)
	return rec
}

func (f *connectionFixture) serve(req *http.Request) *httptest.ResponseRecorder {
	return f.serveWithLocale(req, i18n.Es)
}

func (f *connectionFixture) get(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	return f.serve(req)
}

func (f *connectionFixture) postForm(path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	return f.serve(req)
}

func (f *connectionFixture) deleteReq(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("HX-Request", "true")
	return f.serve(req)
}

func TestConnectionHandlers_GetDisconnected(t *testing.T) {
	f := newConnectionFixture(t)

	res := f.get("/jira/connection")
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()

	assert.Contains(t, body, `id="jira-connection-card"`)
	assert.Contains(t, body, `Conexión con Jira`)
	assert.Contains(t, body, `URL del servidor`)
	assert.Contains(t, body, `name="base_url"`)
	assert.Contains(t, body, `name="token"`)
	assert.Contains(t, body, `Token de acceso personal`)
	assert.Contains(t, body, `Conectar`)
}

func TestConnectionHandlers_SaveValidAndGetConnected(t *testing.T) {
	f := newConnectionFixture(t)

	form := url.Values{
		"base_url":             {f.fakeJira.url()},
		"token":                {testToken},
		"accept_invalid_certs": {"true"},
	}

	res := f.postForm("/jira/connection", form)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()

	assert.Contains(t, body, `id="jira-connection-card"`)
	assert.Contains(t, body, f.fakeJira.url())
	assert.Contains(t, body, `Servidor: `)
	assert.Contains(t, body, `Última sincronización: `)
	assert.Contains(t, body, `Editar conexión`)
	assert.Contains(t, body, `Desconectar`)

	getRes := f.get("/jira/connection")
	assert.Equal(t, http.StatusOK, getRes.Code)
	assert.Contains(t, getRes.Body.String(), f.fakeJira.url())
}

func TestConnectionHandlers_SaveInvalid(t *testing.T) {
	f := newConnectionFixture(t)

	form := url.Values{
		"base_url": {"invalid-url"},
		"token":    {"some-token"},
	}

	res := f.postForm("/jira/connection", form)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()

	assert.Contains(t, body, `name="base_url"`)
	assert.Contains(t, body, `value="invalid-url"`)
	assert.Contains(t, body, `text-error`)
}

func TestConnectionHandlers_SaveKeepStoredTokenForSameServer(t *testing.T) {
	f := newConnectionFixture(t)

	// Initial save with token
	_, err := f.module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(t, err)

	// Save again for same server with empty token
	form := url.Values{
		"base_url":             {f.fakeJira.url()},
		"token":                {""},
		"accept_invalid_certs": {"false"},
	}
	res := f.postForm("/jira/connection", form)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, f.fakeJira.url())
	assert.Contains(t, body, `Editar conexión`)

	stored, err := f.module.GetConnection.Execute(context.Background())
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, testToken, stored.Token())
}

func TestConnectionHandlers_SaveNewServerRequiresNewToken(t *testing.T) {
	f := newConnectionFixture(t)

	// Initial save
	_, err := f.module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(t, err)

	// Submitting a different URL with empty token should fail
	form := url.Values{
		"base_url": {"https://other-jira.example.com"},
		"token":    {""},
	}
	res := f.postForm("/jira/connection", form)
	assert.Equal(t, http.StatusOK, res.Code)
	body := res.Body.String()
	assert.Contains(t, body, `text-error`)
	assert.Contains(t, body, `value="https://other-jira.example.com"`)
}

func TestConnectionHandlers_EditAndCancel(t *testing.T) {
	f := newConnectionFixture(t)

	_, err := f.module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(t, err)

	editRes := f.get("/jira/connection/edit")
	assert.Equal(t, http.StatusOK, editRes.Code)
	editBody := editRes.Body.String()
	assert.Contains(t, editBody, `name="base_url"`)
	assert.Contains(t, editBody, f.fakeJira.url())
	assert.Contains(t, editBody, `Vacío = conservar el token guardado`)
	assert.Contains(t, editBody, `Cancelar`)

	cancelRes := f.get("/jira/connection/cancel")
	assert.Equal(t, http.StatusOK, cancelRes.Code)
	cancelBody := cancelRes.Body.String()
	assert.Contains(t, cancelBody, `Servidor: `)
	assert.Contains(t, cancelBody, f.fakeJira.url())
	assert.Contains(t, cancelBody, `Editar conexión`)
}

func TestConnectionHandlers_CancelWhenDisconnected(t *testing.T) {
	f := newConnectionFixture(t)

	cancelRes := f.get("/jira/connection/cancel")
	assert.Equal(t, http.StatusOK, cancelRes.Code)
	cancelBody := cancelRes.Body.String()
	assert.Contains(t, cancelBody, `URL del servidor`)
	assert.Contains(t, cancelBody, `Conectar`)
}

func TestConnectionHandlers_ConfirmDeleteAndDisconnect(t *testing.T) {
	f := newConnectionFixture(t)

	_, err := f.module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(t, err)

	confirmRes := f.get("/jira/connection?confirm_delete=1")
	assert.Equal(t, http.StatusOK, confirmRes.Code)
	confirmBody := confirmRes.Body.String()
	assert.Contains(t, confirmBody, `Se olvidan el token y las tareas importadas.`)
	assert.Contains(t, confirmBody, `hx-post="/jira/connection/disconnect"`)

	discRes := f.postForm("/jira/connection/disconnect", url.Values{})
	assert.Equal(t, http.StatusOK, discRes.Code)
	discBody := discRes.Body.String()
	assert.Contains(t, discBody, `Token de acceso personal`)
	assert.NotContains(t, discBody, f.fakeJira.url())

	stored, err := f.module.GetConnection.Execute(context.Background())
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestConnectionHandlers_DeleteMethodDisconnect(t *testing.T) {
	f := newConnectionFixture(t)

	_, err := f.module.SaveConnection.Execute(context.Background(), application.SaveConnectionInput{
		BaseURL: f.fakeJira.url(),
		Token:   testToken,
	})
	require.NoError(t, err)

	delRes := f.deleteReq("/jira/connection")
	assert.Equal(t, http.StatusOK, delRes.Code)
	delBody := delRes.Body.String()
	assert.Contains(t, delBody, `Token de acceso personal`)

	stored, err := f.module.GetConnection.Execute(context.Background())
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestConnectionHandlers_Locales(t *testing.T) {
	f := newConnectionFixture(t)

	locales := []struct {
		loc           i18n.Locale
		expectedTitle string
		expectedBtn   string
	}{
		{loc: i18n.En, expectedTitle: "Jira connection", expectedBtn: "Connect"},
		{loc: i18n.Es, expectedTitle: "Conexión con Jira", expectedBtn: "Conectar"},
		{loc: i18n.Zh, expectedTitle: "Jira 连接", expectedBtn: "连接"},
		{loc: i18n.De, expectedTitle: "Jira-Verbindung", expectedBtn: "Verbinden"},
		{loc: i18n.Fr, expectedTitle: "Connexion à Jira", expectedBtn: "Connecter"},
	}

	for _, tc := range locales {
		t.Run(string(tc.loc), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/jira/connection", nil)
			req.Header.Set("HX-Request", "true")
			res := f.serveWithLocale(req, tc.loc)
			assert.Equal(t, http.StatusOK, res.Code)
			body := res.Body.String()
			assert.Contains(t, body, tc.expectedTitle)
			assert.Contains(t, body, tc.expectedBtn)
		})
	}
}
