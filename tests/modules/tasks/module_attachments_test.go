package tasks_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound"
)

// The module wires the attachments end to end: an image given at creation is stored on disk, listed
// with its URL, served by the download handler, and removed with its task.
func TestModuleStoresServesAndRemovesAttachments(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/attachments")
	attachmentsDir := t.TempDir()
	module := newModuleIn(t, repo, attachmentsDir)
	ctx := context.Background()

	input := models.NewTask{WorkspaceID: workspaceID, Title: "con foto", Prompt: "mira la foto", Images: []models.ImageAttachment{storageImage("image/png")}}
	created, err := module.Create.Execute(ctx, input)
	require.NoError(t, err)

	listed, err := module.ListAttachments.Execute(ctx, created.Task.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "image/png", listed[0].MimeType)

	router := chi.NewRouter()
	router.Get(inbound.AttachmentRoute, module.Attachments.Serve)
	response := serve(router, created.Task.ID.String(), listed[0].ID)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, storageBytes, response.Body.Bytes())

	require.NoError(t, module.Delete.Execute(ctx, created.Task.ID))
	afterDelete, err := module.ListAttachments.Execute(ctx, created.Task.ID)
	require.NoError(t, err)
	assert.Empty(t, afterDelete, "the metadata goes with the task")
	assert.NoFileExists(t, filepath.Join(attachmentsDir, listed[0].Path), "the image file goes with the task")
}

// The file editor reads and saves a file of the worktree of an open task; the commit goes to the git port.
func TestModuleReadsAndSavesWorktreeFilesOfAnOpenTask(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/files")
	module := newModule(t, repo)
	ctx := context.Background()
	worktree := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(worktree, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "src", "a.rs"), []byte("fn a() {}\n"), 0o644))
	task := newTask(t, workspaceID, "abierta", models.StatusIdle)
	task.WorktreePath = worktree
	insertTask(t, repo, task)

	_, err := module.SaveFile.Execute(ctx, task.ID, "src/a.rs", "fn b() {}\n")
	require.NoError(t, err)
	file, err := module.GetFile.Execute(ctx, task.ID, "src/a.rs")

	require.NoError(t, err)
	assert.Equal(t, "fn b() {}\n", file.Content)
}
