package projecttree_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/core/web"
	"iworkflow/internal/modules/project_tree/infrastructure/di"
)

// httpModule builds the module with one workspace whose root is dir, and mounts its routes.
func httpModule(t *testing.T, id uuid.UUID, dir string) (*di.Module, http.Handler) {
	t.Helper()
	module := di.NewModule(di.Deps{
		Workspaces: mapLookup{id: {Name: "demo", Path: dir}},
		Tasks:      mapLookup{},
	})
	router := chi.NewRouter()
	router.Use(web.Locale)
	module.Routes(router)
	return module, router
}

// send performs one request against the router. Form values are sent as a urlencoded body for POST.
func send(router http.Handler, method string, target string, form url.Values) *httptest.ResponseRecorder {
	var body *bytes.Reader
	if form != nil {
		body = bytes.NewReader([]byte(form.Encode()))
	} else {
		body = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, body)
	// The e2e scripts run in Spanish; the cookie is the one the shell sets.
	req.Header.Set("Cookie", "iw-lang=es")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// wsParam is the scope query value of a workspace id.
func wsParam(id uuid.UUID) url.Values {
	return url.Values{"scope": {id.String()}}
}

func TestTreeRouteRendersTheRootListing(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola\n"), 0o600))
	id := uuid.New()
	_, router := httpModule(t, id, dir)

	recorder := send(router, http.MethodGet, "/project-tree/tree?"+wsParam(id).Encode(), nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `href="/workspaces/`+id.String()+`/files/README.md"`)
}

func TestDirectoryRouteListsTheChildrenOfAFolder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.rs"), nil, 0o600))
	id := uuid.New()
	_, router := httpModule(t, id, dir)
	query := wsParam(id)
	query.Set("path", "src")

	recorder := send(router, http.MethodGet, "/project-tree/directory?"+query.Encode(), nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `href="/workspaces/`+id.String()+`/files/src/main.rs"`)
}

func TestDirectoryRouteRefusesTraversalAndSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "escape")))
	id := uuid.New()
	_, router := httpModule(t, id, root)

	traversal := wsParam(id)
	traversal.Set("path", "../x")
	recorder := send(router, http.MethodGet, "/project-tree/directory?"+traversal.Encode(), nil)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "ruta inválida: ../x")

	escape := wsParam(id)
	escape.Set("path", "escape")
	recorder = send(router, http.MethodGet, "/project-tree/directory?"+escape.Encode(), nil)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "la ruta queda fuera del proyecto: escape")
}

func TestUnknownWorkspaceIsNotFoundAndBadScopeIsBadRequest(t *testing.T) {
	id := uuid.New()
	_, router := httpModule(t, id, t.TempDir())

	unknown := send(router, http.MethodGet, "/project-tree/tree?scope="+uuid.New().String(), nil)
	assert.Equal(t, http.StatusNotFound, unknown.Code)
	assert.Contains(t, unknown.Body.String(), "el workspace no existe")

	bad := send(router, http.MethodGet, "/project-tree/tree?scope=nope", nil)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Contains(t, bad.Body.String(), "id de workspace inválido")
}

func TestFileBodyShowsTheFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola disco\n"), 0o600))
	id := uuid.New()
	_, router := httpModule(t, id, dir)
	query := wsParam(id)
	query.Set("path", "README.md")

	recorder := send(router, http.MethodGet, "/project-tree/file/body?"+query.Encode(), nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "hola disco")
}

func TestSaveWritesTheFileAndRendersAClosedEditor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(path, []byte("viejo\n"), 0o600))
	id := uuid.New()
	_, router := httpModule(t, id, dir)
	form := wsParam(id)
	form.Set("path", "README.md")
	form.Set("content", "módulo nuevo\r\n")

	recorder := send(router, http.MethodPost, "/project-tree/file", form)

	require.Equal(t, http.StatusOK, recorder.Code)
	disk, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "módulo nuevo\n", string(disk), "CRLF from the browser is saved as LF")
	assert.Contains(t, recorder.Body.String(), "módulo nuevo")
	assert.NotContains(t, recorder.Body.String(), "Cambios sin guardar", "a saved draft is not dirty")
}

func TestSaveOutsideTheRootKeepsTheDraftAndTheOutsideFile(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("intacto"), 0o600))
	require.NoError(t, os.Symlink(secret, filepath.Join(root, "link")))
	id := uuid.New()
	_, router := httpModule(t, id, root)
	form := wsParam(id)
	form.Set("path", "link")
	form.Set("content", "mi borrador")

	recorder := send(router, http.MethodPost, "/project-tree/file", form)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "la ruta queda fuera del proyecto: link")
	assert.Contains(t, recorder.Body.String(), "mi borrador", "the draft stays in the editor")
	disk, err := os.ReadFile(secret)
	require.NoError(t, err)
	assert.Equal(t, "intacto", string(disk), "the file outside the root must not change")
}

func TestStateAnswersWithMarksAndDirtinessOutOfBand(t *testing.T) {
	root := newRepo(t)
	id := uuid.New()
	_, router := httpModule(t, id, root)
	form := wsParam(id)
	form.Set("path", "kept.txt")
	form.Set("content", "one\nnueva línea\n")

	recorder := send(router, http.MethodPost, "/project-tree/file/state", form)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, "Cambios sin guardar")
	assert.Contains(t, body, `class="gm-added"`)
	assert.NotContains(t, body, "<form", "the editor itself is not swapped while typing")
}

func TestStateOfAMissingFileHasNothingToUpdate(t *testing.T) {
	id := uuid.New()
	_, router := httpModule(t, id, t.TempDir())
	form := wsParam(id)
	form.Set("path", "missing.txt")
	form.Set("content", "x")

	recorder := send(router, http.MethodPost, "/project-tree/file/state", form)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Empty(t, recorder.Body.String())
}

func TestStageUnstageAndCommitFollowTheGitState(t *testing.T) {
	root := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "edited.txt"), []byte("a\nB\n"), 0o600))
	id := uuid.New()
	_, router := httpModule(t, id, root)

	stage := wsParam(id)
	stage.Add("paths", "edited.txt")
	staged := send(router, http.MethodPost, "/project-tree/git/stage", stage)
	require.Equal(t, http.StatusOK, staged.Code)
	assert.Contains(t, staged.Body.String(), `data-testid="git-section-staged"`)
	assert.Equal(t, "edited.txt", gitOut(t, root, "diff", "--cached", "--name-only"))

	unstage := wsParam(id)
	unstage.Add("paths", "edited.txt")
	unstaged := send(router, http.MethodPost, "/project-tree/git/unstage", unstage)
	require.Equal(t, http.StatusOK, unstaged.Code)
	assert.Empty(t, gitOut(t, root, "diff", "--cached", "--name-only"))

	require.NoError(t, os.WriteFile(filepath.Join(root, "other.txt"), []byte("z\n"), 0o600))
	require.NoError(t, runGitCommand(root, "add", "edited.txt"))

	blank := wsParam(id)
	blank.Set("message", "   ")
	refused := send(router, http.MethodPost, "/project-tree/git/commit", blank)
	assert.Equal(t, http.StatusBadRequest, refused.Code)
	assert.Contains(t, refused.Body.String(), "el mensaje del commit no puede estar vacío")

	commit := wsParam(id)
	commit.Set("message", "feat: edit")
	committed := send(router, http.MethodPost, "/project-tree/git/commit", commit)
	require.Equal(t, http.StatusOK, committed.Code)
	// other.txt was never staged, so it is still listed as untracked; the list is not empty.
	assert.NotContains(t, committed.Body.String(), `data-testid="git-empty"`)
	assert.Contains(t, committed.Body.String(), `data-testid="git-section-untracked"`)
	assert.Contains(t, committed.Body.String(), `id="pt-git-message-`, "a successful commit clears the message")
	assert.Equal(t, "feat: edit", gitOut(t, root, "log", "-1", "--format=%s"))

	nothing := wsParam(id)
	nothing.Set("message", "again")
	empty := send(router, http.MethodPost, "/project-tree/git/commit", nothing)
	assert.Equal(t, http.StatusUnprocessableEntity, empty.Code)
	assert.Contains(t, empty.Body.String(), "no se pudo hacer commit")
	assert.NotContains(t, empty.Body.String(), `<textarea`, "a failed commit keeps the typed message")
}

func TestSearchFindsFilesByFragment(t *testing.T) {
	root := newRepo(t)
	id := uuid.New()
	_, router := httpModule(t, id, root)
	query := wsParam(id)
	query.Set("q", "KEPT")

	recorder := send(router, http.MethodGet, "/project-tree/search?"+query.Encode(), nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `data-path="kept.txt"`)
	assert.False(t, strings.Contains(recorder.Body.String(), "edited.txt"))
}

func TestDockTabsOpenTheRootAndEachFile(t *testing.T) {
	id := uuid.New()
	module, _ := httpModule(t, id, t.TempDir())
	registry, err := dock.NewRegistry(module.Tabs()...)
	require.NoError(t, err)

	root, params, ok := registry.Match("/workspaces/" + id.String())
	require.True(t, ok)
	assert.Equal(t, "project-tree", root.Name)
	assert.Equal(t, "Project", root.Title(context.Background(), params))

	file, fileParams, ok := registry.Match("/workspaces/" + id.String() + "/files/src/a%20b.rs")
	require.True(t, ok)
	assert.Equal(t, "project-file", file.Name)
	assert.Equal(t, "src/a b.rs", fileParams["path"], "dock params arrive decoded")
	assert.Equal(t, "a b.rs", file.Title(context.Background(), fileParams))
}

// runGitCommand runs git in dir and reports its error.
func runGitCommand(dir string, args ...string) error {
	full := append([]string{"-C", dir}, args...)
	return execGit(full...)
}

// execGit runs git with the given arguments and returns its error.
func execGit(args ...string) error {
	return exec.CommandContext(context.Background(), "git", args...).Run()
}

func TestTabPagesRenderTheTreeAndTheEditorOfTheWorkspace(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola\n"), 0o600))
	id := uuid.New()
	module, _ := httpModule(t, id, dir)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})

	var tree bytes.Buffer
	require.NoError(t, module.RenderTree(ctx, id.String()).Render(ctx, &tree))
	assert.Contains(t, tree.String(), "demo", "the root tab is titled with the workspace name")
	assert.Contains(t, tree.String(), `href="/workspaces/`+id.String()+`/files/README.md"`)
	assert.Contains(t, tree.String(), `href="/workspaces/`+id.String()+`/terminal"`)

	var file bytes.Buffer
	require.NoError(t, module.RenderFile(ctx, id.String(), "README.md").Render(ctx, &file))
	assert.Contains(t, file.String(), `data-code-editor`)
	assert.Contains(t, file.String(), "hola")
}
