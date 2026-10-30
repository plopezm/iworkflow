package inbound

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// insertPattern reads the reference each menu item inserts: data-insert="@<text>".
var insertPattern = regexp.MustCompile(`data-insert="@([^"]*)"`)

// insertedRefs are the references the menu offers, in the order the menu lists them.
func insertedRefs(body string) []string {
	matches := insertPattern.FindAllStringSubmatch(body, -1)
	refs := make([]string, 0, len(matches))
	for _, match := range matches {
		refs = append(refs, match[1])
	}
	return refs
}

// fakeWorkspaces resolves the workspaces of a test. An id in failing answers a lookup error; an unknown id has no workspace.
type fakeWorkspaces struct {
	byID    map[string]models.WorkspaceRef
	failing map[string]bool
}

func (f *fakeWorkspaces) Find(_ context.Context, workspaceID string) (*models.WorkspaceRef, error) {
	if f.failing[workspaceID] {
		return nil, errors.New("workspace lookup broke")
	}
	ref, ok := f.byID[workspaceID]
	if !ok {
		return nil, nil
	}
	return &ref, nil
}

func (f *fakeWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) {
	return nil, nil
}

// fakeFiles answers the files of each workspace and records the workspaces it was asked about, in order.
type fakeFiles struct {
	files   map[string][]string
	failing map[string]bool
	asked   []string
}

func (f *fakeFiles) search(_ context.Context, workspaceID string, _ string) ([]string, error) {
	f.asked = append(f.asked, workspaceID)
	if f.failing[workspaceID] {
		return nil, errors.New("file search broke")
	}
	return f.files[workspaceID], nil
}

// fakeTaskReader answers the task of the test, or its error.
type fakeTaskReader struct {
	task models.Task
	err  error
}

func (f *fakeTaskReader) Execute(context.Context, models.TaskID) (models.Task, error) {
	return f.task, f.err
}

// fakeStoredProjects answers the stored related projects of the task, or its error.
type fakeStoredProjects struct {
	refs []models.WorkspaceRef
	err  error
}

func (f *fakeStoredProjects) Execute(context.Context, models.TaskID) ([]models.WorkspaceRef, error) {
	return f.refs, f.err
}

// currentFiles is the file search of the task's own project, used by the detail composer.
func currentFiles(paths ...string) FileSearch {
	return func(context.Context, models.TaskID, string) ([]string, error) {
		return paths, nil
	}
}

// relatedWorkspaces is the lookup of the test workspaces: the own one, the API and the web projects, and a broken one.
func relatedWorkspaces() *fakeWorkspaces {
	return &fakeWorkspaces{
		byID: map[string]models.WorkspaceRef{
			"own":    {ID: "own", Name: "iworkflow", Path: "/repo/iworkflow"},
			"ws-api": {ID: "ws-api", Name: "api", Path: "/repo/api"},
			"ws-web": {ID: "ws-web", Name: "web", Path: "/repo/web"},
		},
		failing: map[string]bool{},
	}
}

// relatedRouter mounts the composer handlers over the fakes, the way the module does.
func relatedRouter(uc ComposerUseCases, current FileSearch, files *fakeFiles, workspaces outbound.WorkspaceLookup) chi.Router {
	handlers := NewComposerHandlers(uc, current, files.search, workspaces)
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}

// relatedFixtures are the files of the own project and of the two related projects.
func relatedFixtures() *fakeFiles {
	return &fakeFiles{
		files: map[string][]string{
			"own":    {"README.md"},
			"ws-api": {"src/main.go"},
			"ws-web": {"index.ts"},
		},
		failing: map[string]bool{},
	}
}

func createMenu(t *testing.T, target string, files *fakeFiles, workspaces outbound.WorkspaceLookup) (*httptest.ResponseRecorder, string) {
	t.Helper()
	router := relatedRouter(ComposerUseCases{}, nil, files, workspaces)
	return serve(router, httptest.NewRequest(http.MethodGet, target, nil))
}

func TestCreateMenuListsTheOwnFilesThenEachRelatedProjectInOrder(t *testing.T) {
	t.Parallel()
	recorder, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&q=&related_form=1&related=ws-api&related=ws-web",
		relatedFixtures(), relatedWorkspaces())

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{"README.md", "api:src/main.go", "web:index.ts"}, insertedRefs(body))
	assert.Equal(t, 2, countOf(body, `composer-menu-hint`), "each related file shows its project as a badge")
	assert.NotContains(t, body, "text-error")
}

func TestRelatedProjectsWithTheSameNameGetDistinctLabels(t *testing.T) {
	t.Parallel()
	workspaces := &fakeWorkspaces{
		byID: map[string]models.WorkspaceRef{
			"11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa": {ID: "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Name: "api"},
			"22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb": {ID: "22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Name: "api"},
		},
	}
	files := &fakeFiles{files: map[string][]string{
		"11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa": {"main.go"},
		"22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb": {"main.go"},
	}}
	target := "/board/tasks/composer/files?related_form=1" +
		"&related=11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa&related=22222222-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

	_, body := createMenu(t, target, files, workspaces)

	assert.Equal(t, []string{"api-11111111:main.go", "api-22222222:main.go"}, insertedRefs(body),
		"a label names exactly one project of the menu")
}

func TestTheOwnWorkspaceIsLeftOutOfTheRelatedProjects(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()

	_, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1&related=own&related=ws-api",
		files, relatedWorkspaces())

	assert.Equal(t, []string{"README.md", "api:src/main.go"}, insertedRefs(body))
	assert.Equal(t, []string{"own", "ws-api"}, files.asked, "the own project is searched once, as the current project")
}

func TestUnknownRelatedIdsAreSkippedSilently(t *testing.T) {
	t.Parallel()
	recorder, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1&related=ghost&related=&related=ws-api",
		relatedFixtures(), relatedWorkspaces())

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{"README.md", "api:src/main.go"}, insertedRefs(body))
	assert.NotContains(t, body, "text-error", "an unknown id is not an error the user sees")
}

func TestAFailingRelatedSearchIsSkippedAndTheMenuStillAnswers(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()
	files.failing["ws-api"] = true

	recorder, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1&related=ws-api&related=ws-web",
		files, relatedWorkspaces())

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{"README.md", "web:index.ts"}, insertedRefs(body))
	assert.NotContains(t, body, "text-error")
}

func TestARelatedProjectListedTwiceIsSearchedOnce(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()

	_, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1&related=ws-api&related=ws-api",
		files, relatedWorkspaces())

	assert.Equal(t, []string{"README.md", "api:src/main.go"}, insertedRefs(body))
	assert.Equal(t, []string{"own", "ws-api"}, files.asked)
}

func TestRelatedProjectsStopAtTheLimit(t *testing.T) {
	t.Parallel()
	workspaces := &fakeWorkspaces{byID: map[string]models.WorkspaceRef{}}
	files := &fakeFiles{files: map[string][]string{}}
	target := "/board/tasks/composer/files?related_form=1"
	for index := range 12 {
		id := "ws-p" + string(rune('a'+index))
		workspaces.byID[id] = models.WorkspaceRef{ID: id, Name: "p" + string(rune('a'+index))}
		files.files[id] = []string{"f.go"}
		target += "&related=" + id
	}

	_, body := createMenu(t, target, files, workspaces)

	refs := insertedRefs(body)
	require.Len(t, refs, relatedProjectLimit, "only the first ten related projects are offered")
	assert.Equal(t, "pa:f.go", refs[0])
	assert.Equal(t, "pj:f.go", refs[len(refs)-1])
}

func TestWithoutRelatedFormTheCreateMenuOffersOnlyTheOwnFiles(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()

	_, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related=ws-api", files, relatedWorkspaces())

	assert.Equal(t, []string{"README.md"}, insertedRefs(body), "related ids are only read with related_form")
	assert.Equal(t, []string{"own"}, files.asked)
}

func TestARelatedFormWithNoCheckedProjectsOffersOnlyTheOwnFiles(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()

	_, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1", files, relatedWorkspaces())

	assert.Equal(t, []string{"README.md"}, insertedRefs(body))
	assert.Equal(t, []string{"own"}, files.asked)
}

func TestAFailingCurrentProjectSearchStillFailsTheMenuWithoutSearchingRelated(t *testing.T) {
	t.Parallel()
	files := relatedFixtures()
	files.failing["own"] = true

	recorder, body := createMenu(t, "/board/tasks/composer/files?workspace_id=own&related_form=1&related=ws-api",
		files, relatedWorkspaces())

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, body, "text-error")
	assert.Empty(t, insertedRefs(body))
	assert.Equal(t, []string{"own"}, files.asked)
}

func TestTheTaskMenuOffersTheStoredRelatedProjectsWithoutTheOwnOne(t *testing.T) {
	t.Parallel()
	uc := ComposerUseCases{
		Task:            &fakeTaskReader{task: models.Task{ID: taskID(t), WorkspaceID: "own"}},
		RelatedProjects: &fakeStoredProjects{refs: []models.WorkspaceRef{{ID: "ws-api", Name: "api"}, {ID: "own", Name: "iworkflow"}, {ID: "ws-web", Name: "web"}}},
	}
	files := relatedFixtures()
	router := relatedRouter(uc, currentFiles("main.go"), files, relatedWorkspaces())

	_, body := serve(router, httptest.NewRequest(http.MethodGet, "/tasks/"+taskID(t).String()+"/composer/files?q=", nil))

	assert.Equal(t, []string{"main.go", "api:src/main.go", "web:index.ts"}, insertedRefs(body))
	assert.Equal(t, []string{"ws-api", "ws-web"}, files.asked, "the own project is not searched as a related one")
}

func TestTheTaskMenuRelatedFormOverridesTheStoredList(t *testing.T) {
	t.Parallel()
	uc := ComposerUseCases{
		Task:            &fakeTaskReader{task: models.Task{ID: taskID(t), WorkspaceID: "own"}},
		RelatedProjects: &fakeStoredProjects{refs: []models.WorkspaceRef{{ID: "ws-api", Name: "api"}}},
	}
	router := relatedRouter(uc, currentFiles("main.go"), relatedFixtures(), relatedWorkspaces())

	_, body := serve(router, httptest.NewRequest(http.MethodGet,
		"/tasks/"+taskID(t).String()+"/composer/files?q=&related_form=1&related=ws-web", nil))

	assert.Equal(t, []string{"main.go", "web:index.ts"}, insertedRefs(body))
}

func TestTheTaskMenuWithAnEmptyRelatedFormIgnoresTheStoredList(t *testing.T) {
	t.Parallel()
	uc := ComposerUseCases{
		Task:            &fakeTaskReader{task: models.Task{ID: taskID(t), WorkspaceID: "own"}},
		RelatedProjects: &fakeStoredProjects{refs: []models.WorkspaceRef{{ID: "ws-api", Name: "api"}}},
	}
	router := relatedRouter(uc, currentFiles("main.go"), relatedFixtures(), relatedWorkspaces())

	_, body := serve(router, httptest.NewRequest(http.MethodGet,
		"/tasks/"+taskID(t).String()+"/composer/files?q=&related_form=1", nil))

	assert.Equal(t, []string{"main.go"}, insertedRefs(body))
}

func TestTheTaskMenuKeepsItsFilesWhenTheTaskOrTheStoredListCannotBeRead(t *testing.T) {
	t.Parallel()
	tests := map[string]ComposerUseCases{
		"the task cannot be read": {
			Task:            &fakeTaskReader{err: errors.New("task is gone")},
			RelatedProjects: &fakeStoredProjects{refs: []models.WorkspaceRef{{ID: "ws-api", Name: "api"}}},
		},
		"the stored list cannot be read": {
			Task:            &fakeTaskReader{task: models.Task{ID: taskID(t), WorkspaceID: "own"}},
			RelatedProjects: &fakeStoredProjects{err: errors.New("list is gone")},
		},
	}
	for name, uc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := relatedRouter(uc, currentFiles("main.go"), relatedFixtures(), relatedWorkspaces())

			recorder, body := serve(router, httptest.NewRequest(http.MethodGet, "/tasks/"+taskID(t).String()+"/composer/files?q=", nil))

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, []string{"main.go"}, insertedRefs(body))
			assert.NotContains(t, body, "text-error")
		})
	}
}

// countOf counts the occurrences of substring in body.
func countOf(body string, substring string) int {
	return len(regexp.MustCompile(regexp.QuoteMeta(substring)).FindAllStringIndex(body, -1))
}
