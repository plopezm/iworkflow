package inbound

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/presentation/components"
)

const fileTestContent = "fn main() {}\n"

// taskFileReaderFake answers the file the editor loads and counts the reads.
type taskFileReaderFake struct {
	content models.TaskFileContent
	err     error
	path    string
	calls   int
}

func (f *taskFileReaderFake) Execute(_ context.Context, _ models.TaskID, rel string) (models.TaskFileContent, error) {
	f.calls++
	f.path = rel
	return f.content, f.err
}

// taskFileWriterFake records the save and answers whether a commit was created.
type taskFileWriterFake struct {
	committed bool
	err       error
	path      string
	content   string
	calls     int
}

func (f *taskFileWriterFake) Execute(_ context.Context, _ models.TaskID, rel string, content string) (bool, error) {
	f.calls++
	f.path = rel
	f.content = content
	return f.committed, f.err
}

func newTaskFileIDForTest(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}

// serveTaskFile sends one request to the file endpoints, as htmx does from the diff view.
func serveTaskFile(t *testing.T, h *TaskFileHandlers, locale i18n.Locale, method string, target string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	h.Mount(router)
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	request := httptest.NewRequest(method, target, body)
	request.Header.Set("HX-Request", "true")
	if form != nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	request = request.WithContext(inLanguage(locale))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// renderTemplForTest renders a component to the markup it produces in the request language of the test.
func renderTemplForTest(t *testing.T, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, component.Render(context.Background(), &out))
	return out.String()
}

func digestOf(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestTaskFileEditorLoadsTheContentAndItsDigest(t *testing.T) {
	read := &taskFileReaderFake{content: models.TaskFileContent{Path: "src/a.rs", Content: fileTestContent}}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: read, Write: &taskFileWriterFake{}})
	id := newTaskFileIDForTest(t)

	recorder := serveTaskFile(t, h, i18n.En, http.MethodGet, "/tasks/"+id.String()+"/file?scope="+id.String()+"&path=src%2Fa.rs", nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "src/a.rs", read.path)
	body := recorder.Body.String()
	assert.Contains(t, body, "<textarea")
	assert.Contains(t, body, "fn main() {}")
	assert.Contains(t, body, `name="base" value="`+digestOf(fileTestContent)+`"`)
}

func TestTaskFileEditorRejectsAMissingPathInTheRequestLanguage(t *testing.T) {
	read := &taskFileReaderFake{}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: read, Write: &taskFileWriterFake{}})
	id := newTaskFileIDForTest(t)

	recorder := serveTaskFile(t, h, i18n.Es, http.MethodGet, "/tasks/"+id.String()+"/file?scope="+id.String(), nil)

	assert.Equal(t, 0, read.calls, "a request without a path never reaches the use case")
	assert.Contains(t, recorder.Body.String(), "La ruta del fichero no es válida.")
}

func TestTaskFileEditorShowsTheGoneTaskWithoutAnEditor(t *testing.T) {
	read := &taskFileReaderFake{err: werrors.NotFound("gone")}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: read, Write: &taskFileWriterFake{}})
	id := newTaskFileIDForTest(t)

	recorder := serveTaskFile(t, h, i18n.En, http.MethodGet, "/tasks/"+id.String()+"/file?scope="+id.String()+"&path=a.rs", nil)

	assert.NotContains(t, recorder.Body.String(), "<textarea")
	assert.Contains(t, recorder.Body.String(), renderTemplForTest(t, components.TaskGone()))
}

func TestTaskFileSaveWritesThenAnswersTheCommitWithTheDiffEvent(t *testing.T) {
	write := &taskFileWriterFake{committed: true}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: &taskFileReaderFake{}, Write: write})
	id := newTaskFileIDForTest(t)
	form := url.Values{"scope": {id.String()}, "path": {"src/a.rs"}, "content": {"fn b() {}"}, "base": {digestOf(fileTestContent)}}

	recorder := serveTaskFile(t, h, i18n.En, http.MethodPost, "/tasks/"+id.String()+"/file", form)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "src/a.rs", write.path)
	assert.Equal(t, "fn b() {}", write.content)
	assert.Empty(t, strings.TrimSpace(recorder.Body.String()), "a saved editor closes")
	assert.Equal(t, DiffSavedEvent, recorder.Header().Get("HX-Trigger"))
}

func TestTaskFileSaveWithoutChangesKeepsTheEditorOpenAndSaysSo(t *testing.T) {
	write := &taskFileWriterFake{committed: false}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: &taskFileReaderFake{}, Write: write})
	id := newTaskFileIDForTest(t)
	form := url.Values{"scope": {id.String()}, "path": {"a.rs"}, "content": {"same"}, "base": {digestOf("same")}}

	recorder := serveTaskFile(t, h, i18n.Es, http.MethodPost, "/tasks/"+id.String()+"/file", form)

	assert.Contains(t, recorder.Body.String(), "Sin cambios: el fichero ya tenía ese contenido, no se creó ningún commit.")
	assert.Contains(t, recorder.Body.String(), "<textarea")
	assert.Empty(t, recorder.Header().Get("HX-Trigger"), "nothing was committed, so the diff is not reloaded")
}

func TestTaskFileSaveRejectsContentAboveTheLimitBeforeWriting(t *testing.T) {
	write := &taskFileWriterFake{}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: &taskFileReaderFake{}, Write: write})
	id := newTaskFileIDForTest(t)
	big := strings.Repeat("a", models.MaxTaskFileBytes+1)
	form := url.Values{"scope": {id.String()}, "path": {"big.txt"}, "content": {big}, "base": {""}}

	recorder := serveTaskFile(t, h, i18n.Es, http.MethodPost, "/tasks/"+id.String()+"/file", form)

	assert.Equal(t, 0, write.calls)
	assert.Contains(t, recorder.Body.String(), "1 MiB")
}

func TestTaskFileSaveLocalizesDomainErrorsAndHidesTheRest(t *testing.T) {
	write := &taskFileWriterFake{err: werrors.FileNotEditable("a.bin")}
	h := NewTaskFileHandlers(TaskFileUseCases{Read: &taskFileReaderFake{}, Write: write})
	id := newTaskFileIDForTest(t)
	form := url.Values{"scope": {id.String()}, "path": {"a.bin"}, "content": {"x"}, "base": {""}}

	recorder := serveTaskFile(t, h, i18n.Zh, http.MethodPost, "/tasks/"+id.String()+"/file", form)
	assert.Contains(t, recorder.Body.String(), "该文件不能编辑。")

	write.err = errors.New("boom: /secret/path")
	recorder = serveTaskFile(t, h, i18n.De, http.MethodPost, "/tasks/"+id.String()+"/file", form)
	assert.Contains(t, recorder.Body.String(), "Die Datei konnte nicht gespeichert werden.")
	assert.NotContains(t, recorder.Body.String(), "boom")
	assert.NotContains(t, recorder.Body.String(), "/secret/path")
}

func TestTaskFileStateReportsUnsavedChangesAgainstTheLoadedDigest(t *testing.T) {
	h := NewTaskFileHandlers(TaskFileUseCases{Read: &taskFileReaderFake{}, Write: &taskFileWriterFake{}})
	id := newTaskFileIDForTest(t)
	const unsaved = "Cambios sin guardar"

	edited := serveTaskFile(t, h, i18n.Es, http.MethodPost, "/tasks/"+id.String()+"/file/state", url.Values{
		"scope": {id.String()}, "path": {"a.rs"}, "content": {"changed"}, "base": {digestOf("loaded")},
	})
	clean := serveTaskFile(t, h, i18n.Es, http.MethodPost, "/tasks/"+id.String()+"/file/state", url.Values{
		"scope": {id.String()}, "path": {"a.rs"}, "content": {"loaded"}, "base": {digestOf("loaded")},
	})

	assert.Contains(t, edited.Body.String(), unsaved)
	assert.NotContains(t, clean.Body.String(), unsaved)
}
