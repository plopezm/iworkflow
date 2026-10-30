package agents_files_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/infrastructure/di"
)

const (
	// spanishCardError is the Rust text of the 201-character description error, with its prefix.
	spanishCardError = "error running server function: la descripción es obligatoria (máximo 200 caracteres)"
	noFilesText      = "Aún no hay ficheros AGENTS.md."
	overwriteText    = "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?"
	committedText    = "Añadido y commiteado"
	uncommittedText  = "Añadido (no se pudo crear el commit; queda sin commitear)"
)

// disabledAttr matches the boolean disabled attribute, not hx-disabled-elt.
var disabledAttr = regexp.MustCompile(`\sdisabled[\s>]`)

// server is the module mounted on a chi router, with a migrated database and the given projects.
type server struct {
	t      *testing.T
	router http.Handler
	module *di.Module
}

func newServer(t *testing.T, projects ...models.ProjectRef) *server {
	t.Helper()
	module := di.NewModule(di.Deps{
		DB:         dbtest.New(t),
		StorageDir: filepath.Join(t.TempDir(), "agents-files"),
		Projects:   staticProjects{projects: projects},
		Logger:     quietLogger(),
	})
	router := chi.NewRouter()
	module.Routes(router)
	return &server{t: t, router: router, module: module}
}

// send performs one request in Spanish, as the e2e suite does (iw-lang=es).
func (s *server) send(method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	s.t.Helper()
	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req = req.WithContext(i18n.WithState(req.Context(), i18n.State{Locale: i18n.Es, Chosen: true}))
	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, req)
	return recorder
}

// upload posts the upload form with a description and a file.
func (s *server) upload(description string, content string) *httptest.ResponseRecorder {
	s.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(s.t, writer.WriteField("description", description))
	file, err := writer.CreateFormFile("file", "AGENTS.md")
	require.NoError(s.t, err)
	_, err = file.Write([]byte(content))
	require.NoError(s.t, err)
	require.NoError(s.t, writer.Close())
	return s.send(http.MethodPost, "/agents-files/upload", &body, writer.FormDataContentType())
}

// storedFile saves a file through the use case and returns its identifier.
func (s *server) storedFile(description string) models.AgentsFile {
	s.t.Helper()
	file, err := s.module.Upload.Execute(context.Background(), models.NewAgentsFile{
		Description: description,
		Content:     []byte("# " + description + "\n"),
	})
	require.NoError(s.t, err)
	return file
}

func TestTabRegistersTheScreenAsADockTab(t *testing.T) {
	module := di.NewModule(di.Deps{DB: dbtest.New(t), StorageDir: t.TempDir(), Projects: staticProjects{}, Logger: quietLogger()})
	registry, err := dock.NewRegistry(module.Tabs()...)
	require.NoError(t, err)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})

	var page strings.Builder
	require.NoError(t, registry.Render(ctx, "/agents-files").Render(ctx, &page))

	require.Equal(t, "AGENTS.md", registry.Title(ctx, "/agents-files"))
	require.Equal(t, "§", registry.Icon("/agents-files"))
	require.Contains(t, page.String(), `<h1 class="page-title">AGENTS.md</h1>`)
	require.Contains(t, page.String(), noFilesText)
	require.Contains(t, page.String(), `hx-post="/agents-files/upload"`)
}

func TestUploadStoresTheFileAndSwapsTheListOutOfBand(t *testing.T) {
	s := newServer(t)

	recorder := s.upload("Reglas demo", "# reglas\n")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `hx-swap-oob="true"`)
	require.Contains(t, recorder.Body.String(), "Reglas demo")
	listed, err := s.module.List.Execute(context.Background())
	require.NoError(t, err)
	require.Len(t, listed, 1)
}

func TestUploadRejectsALongDescriptionWith400AndKeepsTheTypedText(t *testing.T) {
	s := newServer(t)
	long := strings.Repeat("a", 201)

	recorder := s.upload(long, "# x")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), spanishCardError)
	require.Contains(t, recorder.Body.String(), `value="`+long+`"`)
	listed, err := s.module.List.Execute(context.Background())
	require.NoError(t, err)
	require.Empty(t, listed, "a rejected upload must not be stored")
}

func TestUploadOverTheFileLimitKeepsTheTypedDescription(t *testing.T) {
	s := newServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("description", "mi descripción"))
	file, err := writer.CreateFormFile("file", "AGENTS.md")
	require.NoError(t, err)
	_, err = file.Write(bytes.Repeat([]byte("a"), 512*1024+1))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := s.send(http.MethodPost, "/agents-files/upload", &body, writer.FormDataContentType())

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "el fichero supera el tamaño máximo de 512 KiB")
	require.Contains(t, recorder.Body.String(), `value="mi descripción"`)
}

func TestUploadWithoutFileIsA400WithTheEmptyContentText(t *testing.T) {
	s := newServer(t)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("description", "d"))
	require.NoError(t, writer.Close())

	recorder := s.send(http.MethodPost, "/agents-files/upload", &body, writer.FormDataContentType())

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "error running server function: el fichero está vacío")
}

func TestUploadOfANonMultipartBodyIsA400(t *testing.T) {
	s := newServer(t)

	recorder := s.send(http.MethodPost, "/agents-files/upload", strings.NewReader("x=1"), "application/x-www-form-urlencoded")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "formulario de subida inválido: ")
}

func TestDeleteRemovesTheFileAndReturnsTheList(t *testing.T) {
	s := newServer(t)
	file := s.storedFile("Borrar demo")

	recorder := s.send(http.MethodDelete, "/agents-files/"+file.ID.String(), nil, "")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "Borrar demo")
	require.Contains(t, recorder.Body.String(), noFilesText)
}

func TestDeleteOfAnUnknownFileIsA404(t *testing.T) {
	s := newServer(t)
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)

	recorder := s.send(http.MethodDelete, "/agents-files/"+id.String(), nil, "")

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "fichero AGENTS.md no encontrado: "+id.String())
}

func TestDeleteWithAMalformedIdIsA400(t *testing.T) {
	s := newServer(t)

	recorder := s.send(http.MethodDelete, "/agents-files/nope", nil, "")

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "identificador de fichero AGENTS.md inválido: nope")
}

func TestApplyCommitsTheFileAndAsksBeforeOverwriting(t *testing.T) {
	root := gitRepo(t)
	s := newServer(t, models.ProjectRef{ID: "p1", Name: "demo", Path: root})
	file := s.storedFile("Aplicar demo")
	path := "/agents-files/" + file.ID.String() + "/apply"

	first := s.send(http.MethodPost, path, strings.NewReader("project_id=p1"), "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusOK, first.Code)
	require.Contains(t, first.Body.String(), committedText)
	require.Equal(t, "docs: add AGENTS.md", runGitCmd(t, root, "log", "-1", "--format=%s"))

	second := s.send(http.MethodPost, path, strings.NewReader("project_id=p1"), "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusOK, second.Code, "an existing file is a typed answer, not an error")
	require.Contains(t, second.Body.String(), overwriteText)
	require.Contains(t, second.Body.String(), "Sobrescribir")

	// Same content again: the overwrite happens but nothing changes, so nothing is committed.
	third := s.send(http.MethodPost, path, strings.NewReader("project_id=p1&overwrite=true"), "application/x-www-form-urlencoded")
	require.Equal(t, http.StatusOK, third.Code)
	require.Contains(t, third.Body.String(), uncommittedText)
}

func TestApplyToAnUnknownProjectIsA404InTheCard(t *testing.T) {
	s := newServer(t)
	file := s.storedFile("Sin proyecto")

	recorder := s.send(http.MethodPost, "/agents-files/"+file.ID.String()+"/apply", strings.NewReader("project_id=missing"), "application/x-www-form-urlencoded")

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "error running server function: proyecto no encontrado: missing")
	require.Contains(t, recorder.Body.String(), `id="agents-file-`+file.ID.String()+`"`)
}

func TestApplyButtonIsEnabledOnceAProjectIsSelected(t *testing.T) {
	s := newServer(t)
	id := s.storedFile("Botón").ID.String()

	withoutProject := s.send(http.MethodGet, "/agents-files/"+id+"/apply-button", nil, "")
	withProject := s.send(http.MethodGet, "/agents-files/"+id+"/apply-button?project_id=p1", nil, "")

	require.Regexp(t, disabledAttr, withoutProject.Body.String())
	require.NotRegexp(t, disabledAttr, withProject.Body.String())
}

func TestCardReturnsTheFileWithoutFeedbackAndKeepsTheSelectedProject(t *testing.T) {
	s := newServer(t, models.ProjectRef{ID: "p1", Name: "demo", Path: t.TempDir()})
	id := s.storedFile("Tarjeta").ID.String()

	recorder := s.send(http.MethodGet, "/agents-files/"+id+"/card?project_id=p1", nil, "")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `id="agents-file-`+id+`"`)
	require.Contains(t, recorder.Body.String(), `<option value="p1" selected`)
	require.NotContains(t, recorder.Body.String(), committedText)
	require.NotContains(t, recorder.Body.String(), overwriteText)
}

func TestCardOfAnUnknownFileIsA404(t *testing.T) {
	s := newServer(t)
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)

	recorder := s.send(http.MethodGet, "/agents-files/"+id.String()+"/card", nil, "")

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "fichero AGENTS.md no encontrado")
}
