package tasks_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
)

// attachmentServer serves the attachment route over the given use case, the way the composition root will.
func attachmentServer(get *application.GetTaskAttachment) http.Handler {
	router := chi.NewRouter()
	router.Get(inbound.AttachmentRoute, inbound.NewAttachmentHandler(get).Serve)
	return router
}

// storedImage writes the image file and its metadata for a fresh task, and returns the task and the metadata.
func storedImage(t *testing.T, repo *Repo, adapter interface {
	Save(context.Context, models.TaskID, string, models.ImageAttachment) (string, error)
}) (models.Task, models.StoredAttachment) {
	t.Helper()
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/http"), "http", models.StatusBacklog)
	insertTask(t, repo, task)
	attachmentID := uuid.NewString()
	path, err := adapter.Save(bg(), task.ID, attachmentID, storageImage("image/png"))
	require.NoError(t, err)
	stored := models.StoredAttachment{ID: attachmentID, Name: "foto", MimeType: "image/png", Path: path, SizeBytes: uint64(len(storageBytes))}
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{stored}))
	return task, stored
}

// serve performs a GET on the route and returns the recorded response.
func serve(handler http.Handler, taskID string, attachmentID string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/tasks/"+taskID+"/attachments/"+attachmentID, nil))
	return recorder
}

// Rust: attachment_http::found_attachment_has_content_type_and_immutable_cache_headers
func TestAttachmentResponseHasContentTypeCacheAndNoSniffHeaders(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, _ := newStorage(t)
	task, stored := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), stored.ID)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "image/png", response.Header().Get("Content-Type"))
	assert.Equal(t, "private, max-age=31536000, immutable", response.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
}

// Rust: attachment_http::found_attachment_body_is_the_raw_bytes
func TestAttachmentResponseBodyIsTheRawBytes(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, _ := newStorage(t)
	task, stored := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), stored.ID)

	assert.Equal(t, storageBytes, response.Body.Bytes())
	assert.Equal(t, strconv.Itoa(len(storageBytes)), response.Header().Get("Content-Length"))
}

// Rust: attachment_http::not_found_is_404_without_cache_headers
func TestAttachmentNotFoundIs404WithoutCacheHeaders(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, _ := newStorage(t)
	task, _ := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), uuid.NewString())

	assert.Equal(t, http.StatusNotFound, response.Code)
	assert.Empty(t, response.Header().Get("Cache-Control"))
	assert.Empty(t, response.Header().Get("X-Content-Type-Options"))
}

// Rust: attachment_http::other_errors_are_500. A file that is gone is NotFound (404, as in Rust); a path
// that is no longer a regular file is a storage failure (500).
func TestAttachmentStorageFailureIs500AndMissingFileIs404(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, dir := newStorage(t)
	task, stored := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	require.NoError(t, os.Remove(filepath.Join(dir, stored.Path)))
	assert.Equal(t, http.StatusNotFound, serve(handler, task.ID.String(), stored.ID).Code)

	require.NoError(t, os.Mkdir(filepath.Join(dir, stored.Path), 0o700))
	assert.Equal(t, http.StatusInternalServerError, serve(handler, task.ID.String(), stored.ID).Code)
}

// Rust: attachment_http::ids_must_be_uuids
func TestAttachmentIDsMustBeUUIDs(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, _ := newStorage(t)
	task, stored := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	for _, bad := range []struct{ task, attachment string }{
		{"..", stored.ID},
		{task.ID.String(), ".."},
		{task.ID.String(), "../../secret"},
		{"nope", stored.ID},
	} {
		response := serve(handler, bad.task, bad.attachment)
		assert.Equal(t, http.StatusNotFound, response.Code, "%+v", bad)
	}
}

// Rust: attachment_http::attachment_id_is_normalised_to_canonical_form
func TestAttachmentIDIsNormalisedToCanonicalForm(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, _ := newStorage(t)
	task, stored := storedImage(t, repo, adapter)
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), strings.ToUpper(stored.ID))

	assert.Equal(t, http.StatusOK, response.Code)
}

// A stored row whose MIME type is not an accepted image type is never served: the browser could run it.
func TestAttachmentWithANonImageContentTypeIsNeverServed(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, dir := newStorage(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/html"), "html", models.StatusBacklog)
	insertTask(t, repo, task)
	relative := filepath.Join(task.ID.String(), uuid.NewString()+".html")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, task.ID.String()), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, relative), []byte("<script>alert(1)</script>"), 0o600))
	stored := models.StoredAttachment{ID: uuid.NewString(), Name: "x", MimeType: "text/html", Path: relative, SizeBytes: 25}
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{stored}))
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), stored.ID)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), "script")
	assert.Empty(t, response.Header().Get("Content-Type"))
}

// A metadata row whose path is a symbolic link out of the folder must not serve the file it points to.
func TestAttachmentPathThroughASymbolicLinkIsNotServed(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	adapter, dir := newStorage(t)
	task := newTask(t, insertWorkspace(t, repo.DB, "/repos/link"), "link", models.StatusBacklog)
	insertTask(t, repo, task)
	secret := filepath.Join(filepath.Dir(dir), "secret.png")
	require.NoError(t, os.WriteFile(secret, []byte("top secret"), 0o600))
	relative := filepath.Join(task.ID.String(), uuid.NewString()+".png")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, task.ID.String()), 0o700))
	require.NoError(t, os.Symlink(secret, filepath.Join(dir, relative)))
	stored := models.StoredAttachment{ID: uuid.NewString(), Name: "x", MimeType: "image/png", Path: relative, SizeBytes: 10}
	require.NoError(t, repo.Attachments.InsertMany(bg(), task.ID, []models.StoredAttachment{stored}))
	handler := attachmentServer(application.NewGetTaskAttachment(repo.Attachments, adapter))

	response := serve(handler, task.ID.String(), stored.ID)

	assert.Equal(t, http.StatusInternalServerError, response.Code)
	assert.NotContains(t, response.Body.String(), "top secret")
}
