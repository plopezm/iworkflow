package components

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// render renders a component in Spanish, the language the e2e scripts run in.
func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

var scopeID = uuid.MustParse("00000000-0000-0000-0000-0000000000d1")

func workspace() models.ProjectScope {
	return models.WorkspaceScope(scopeID)
}

func TestTreeBodyLinksFilesAndExpandsFolders(t *testing.T) {
	view := state.NewRootView(workspace(), "demo", []models.TreeEntry{
		{Name: "src", Path: "src", Kind: models.TreeEntryDirectory},
		{Name: "README.md", Path: "README.md", Kind: models.TreeEntryFile},
	}, nil)

	body := render(t, TreeBody(view))

	assert.Contains(t, body, `data-section-toggle`)
	assert.Contains(t, body, `hx-get="/project-tree/directory?path=src&amp;scope=`+scopeID.String()+`"`)
	assert.Contains(t, body, `hx-trigger="click once"`)
	assert.Contains(t, body, `hx-target="next [data-section-body]"`)
	assert.Contains(t, body, `<ul data-section-body style="display: none">`)
	assert.Contains(t, body, `href="/workspaces/`+scopeID.String()+`/files/README.md"`)
	assert.Contains(t, body, `data-dock-link`)
	assert.Contains(t, body, `data-dock-beside="true"`)
}

func TestEmptyRootSaysTheFolderIsEmpty(t *testing.T) {
	body := render(t, TreeBody(state.NewRootView(workspace(), "demo", nil, nil)))

	assert.Contains(t, body, "Carpeta vacía.")
	assert.NotContains(t, body, "<ul>")
}

func TestErrorReplacesTheTreeRows(t *testing.T) {
	body := render(t, TreeBody(state.TreeErrorView(workspace(), "la ruta no existe: x")))

	assert.Contains(t, body, "la ruta no existe: x")
}

func TestChildrenOfAnEmptyFolderIndentTheMessage(t *testing.T) {
	view := state.NewChildrenView(workspace(), "a/b", nil, nil)

	body := render(t, TreeChildren(view))

	// "a/b" is at depth 1, so its children's message sits two levels deeper: depth 3.
	assert.Contains(t, body, `style="padding-left: 4.125rem;"`)
	assert.Contains(t, body, "Carpeta vacía.")
}

func TestChangesPanelCarriesTheContractIdsAndTheCommitForm(t *testing.T) {
	view := state.GitView{Scope: workspace(), Groups: state.GroupChanges([]models.GitChange{
		{Path: "README.md", Kind: models.GitChangeModified, Staged: false, Unstaged: true},
	})}

	body := render(t, GitChangesPanel(GitChangesPanelProps{View: view}))

	assert.Contains(t, body, `data-testid="git-changes"`)
	assert.Contains(t, body, `data-git-root`)
	assert.Contains(t, body, `[data-git-root]:not(:has([data-git-staged])) [data-git-commit] { display: none; }`,
		"the CSS rule must be rendered as CSS, not as escaped text")
	assert.Contains(t, body, `data-git-commit`)
	assert.Contains(t, body, `data-testid="git-commit-message"`)
	assert.Contains(t, body, `<section data-testid="git-section-unstaged">`)
	assert.Contains(t, body, `data-testid="git-row" data-path="README.md"`)
	assert.Contains(t, body, `data-testid="git-toggle"`)
	assert.NotContains(t, body, `data-git-staged=""`, "no staged change means no marker for the commit form")
	assert.NotContains(t, body, `git-section-staged`)
	assert.Contains(t, body, `hx-trigger="every 5s [this.offsetParent !== null]"`)
}

func TestStagedSectionMarksTheCommitFormVisible(t *testing.T) {
	view := state.GitView{Scope: workspace(), Groups: state.GroupChanges([]models.GitChange{
		{Path: "a.rs", Kind: models.GitChangeAdded, Staged: true, Unstaged: false},
	})}

	body := render(t, GitChangesBody(view))

	assert.Contains(t, body, `<section data-testid="git-section-staged" data-git-staged="">`)
	assert.Contains(t, body, `hx-post="/project-tree/git/unstage"`)
	assert.Contains(t, body, `name="paths" value="a.rs"`)
}

func TestEmptyChangesSaysTheTreeIsClean(t *testing.T) {
	body := render(t, GitChangesBody(state.GitView{Scope: workspace()}))

	assert.Contains(t, body, `data-testid="git-empty"`)
	assert.Contains(t, body, "Sin cambios")
}

func TestLoadErrorReplacesTheChangesList(t *testing.T) {
	body := render(t, GitChangesBody(state.GitView{Scope: workspace(), LoadError: "no se pudo leer"}))

	assert.Contains(t, body, "no se pudo leer")
	assert.NotContains(t, body, `data-testid="git-empty"`)
}

func TestActionResponseClearsTheMessageOnlyWhenAskedTo(t *testing.T) {
	view := state.GitView{Scope: workspace(), ActionError: "boom"}

	failed := render(t, GitActionResponse(view, false))
	assert.Contains(t, failed, `id="pt-git-error-`)
	assert.Contains(t, failed, "boom")
	assert.Contains(t, failed, `hx-swap-oob="true"`)
	assert.NotContains(t, failed, `<textarea`, "a failed commit keeps the typed message")

	cleared := render(t, GitActionResponse(state.GitView{Scope: workspace()}, true))
	assert.Contains(t, cleared, `<textarea`)
	assert.Contains(t, cleared, `hx-swap-oob="true"`)
}

func TestSectionTitleCountsItsRows(t *testing.T) {
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	assert.Equal(t, "Con stage 2", sectionTitle(ctx, msgStaged, 2))
}

func TestFileEditorIsAFormOverTheCodeEditorContract(t *testing.T) {
	view := state.FileView{Scope: workspace(), Path: "README.md", Draft: "hola\n", Loaded: "hola\n"}

	body := render(t, FileEditor(view))

	assert.Contains(t, body, `hx-post="/project-tree/file"`)
	assert.Contains(t, body, `data-code-editor`)
	assert.Contains(t, body, `name="content"`)
	assert.Contains(t, body, `hx-post="/project-tree/file/state"`)
	assert.Contains(t, body, `data-code-gutter`)
	assert.Contains(t, body, `class="code-gutter code-gutter-edit"`)
	assert.Contains(t, body, `disabled`, "Save is disabled while the draft equals the file on disk")
	assert.NotContains(t, body, "Cambios sin guardar")
}

func TestDirtyEditorEnablesSaveAndSaysSo(t *testing.T) {
	view := state.FileView{Scope: workspace(), Path: "README.md", Draft: "otro\n", Loaded: "hola\n"}

	body := render(t, FileToolbar(view, false))

	assert.Contains(t, body, "Cambios sin guardar")
	assert.NotContains(t, body, "disabled", "Save and Discard are enabled")
}

func TestFileStateUpdateReplacesToolbarAndGutterOutOfBand(t *testing.T) {
	base := "hola\n"
	view := state.FileView{
		Scope:  workspace(),
		Path:   "README.md",
		Draft:  "hola\nlínea nueva\n",
		Loaded: "hola\n",
		Marks:  state.MarksFor(&base, "hola\nlínea nueva\n"),
	}

	body := render(t, FileStateUpdate(view))

	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, `hx-swap-oob="innerHTML:#`+gutterID(view)+`"`)
	assert.Contains(t, body, `class="gm-added"`)
}

func TestFileEditorShowsTheSaveErrorAndKeepsTheDraft(t *testing.T) {
	view := state.FileView{Scope: workspace(), Path: "x", Draft: "mi texto", Loaded: "viejo", Error: "ruta inválida: x"}

	body := render(t, FileEditor(view))

	assert.Contains(t, body, "ruta inválida: x")
	assert.Contains(t, body, "mi texto")
}

func TestSearchResultsListOneItemPerPath(t *testing.T) {
	body := render(t, SearchResults([]string{"README.md", "src/main.rs"}))

	assert.Contains(t, body, `<li class="composer-menu-item" data-path="README.md">README.md</li>`)
	assert.Equal(t, 2, bytes.Count([]byte(body), []byte("composer-menu-item")))
}

func TestGutterNumbersTheDraftLinesWithTheirMarks(t *testing.T) {
	view := state.FileView{Draft: "a\nb\n", Marks: []coreui.LineMark{coreui.LineModified}}

	body := render(t, GutterSpans(view))

	assert.Contains(t, body, `<span class="gm-modified">1</span>`)
	assert.Contains(t, body, `<span class="gm-none">2</span>`)
	assert.Contains(t, body, `<span class="gm-none">3</span>`)
}
