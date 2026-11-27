package pages

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// pathInputTag is the markup of the path field of the edit form.
var pathInputTag = regexp.MustCompile(`<input[^>]*name="path"[^>]*>`)

func renderIn(t *testing.T, locale i18n.Locale, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

func TestEditPageShowsTheStoredPathInAnEditableFieldWhenNothingBlocksIt(t *testing.T) {
	body := renderIn(t, i18n.En, WorkspaceEditPage(state.EditPage{ID: "w1", Name: "alpha", Path: "/repo/alpha"}))

	field := pathInputTag.FindString(body)
	require.NotEmpty(t, field, "the path is an input named path")
	assert.Contains(t, field, `value="/repo/alpha"`, "pre-filled with the stored path")
	assert.NotContains(t, field, "disabled", "an editable path is sent with the form")
	assert.NotContains(t, body, "<code", "the path is no longer a read-only code block")
	assert.NotContains(t, body, "data-path-locked")
}

func TestEditPageLocksThePathWithTheExactReasonWhileTasksAreUnfinished(t *testing.T) {
	reason := "The path cannot change while the workspace has unfinished tasks."
	body := renderIn(t, i18n.En, WorkspaceEditPage(state.EditPage{
		ID: "w1", Name: "alpha", Path: "/repo/alpha", PathLocked: true, PathLockReason: reason,
	}))

	field := pathInputTag.FindString(body)
	require.NotEmpty(t, field)
	assert.Contains(t, field, `value="/repo/alpha"`)
	assert.Contains(t, field, "disabled", "a locked path field cannot be edited")
	assert.Contains(t, body, `data-path-locked`)
	assert.Contains(t, body, reason, "the reason is shown under the field")
}

func TestEditPageKeepsTheGroupSelectorSlotWhenThePathIsLocked(t *testing.T) {
	body := renderIn(t, i18n.En, WorkspaceEditPage(state.EditPage{ID: "w1", PathLocked: true, PathLockReason: "locked"}))

	assert.Contains(t, body, `hx-get="/workspaces/w1/edit/groups"`, "the group selector slot is untouched by the path lock")
	assert.Contains(t, body, `hx-post="/workspaces/w1/update"`)
}

func TestEditRejectionShowsTheConfirmationCheckboxOnlyForAnUnconfirmedMove(t *testing.T) {
	unconfirmed := renderIn(t, i18n.Es, EditRejection(state.EditFailure{Rule: state.RuleSameProjectUnconfirmed}))
	assert.Contains(t, unconfirmed, `type="checkbox"`)
	assert.Contains(t, unconfirmed, `name="confirm_same_project"`)
	assert.Contains(t, unconfirmed, "Confirmo que es el mismo proyecto", "the checkbox label is the Spanish text")

	other := renderIn(t, i18n.En, EditRejection(state.EditFailure{Rule: state.RulePathNotSameProject, Detail: "/other"}))
	assert.NotContains(t, other, `name="confirm_same_project"`)
}

func TestEditFailureTextIsTheExactReasonOfEachRuleInEveryLanguage(t *testing.T) {
	tests := []struct {
		name    string
		failure state.EditFailure
		en      string
		es      string
	}{
		{
			name:    "tasks block the path",
			failure: state.EditFailure{Rule: state.RulePathBlocked},
			en:      "The path cannot change while the workspace has unfinished tasks.",
			es:      "La ruta no puede cambiar mientras el workspace tenga tareas sin terminar.",
		},
		{
			name:    "not absolute",
			failure: state.EditFailure{Rule: state.RulePathNotAbsolute, Detail: "relative/repo"},
			en:      "The path must be absolute: relative/repo",
			es:      "La ruta debe ser absoluta: relative/repo",
		},
		{
			name:    "folder missing",
			failure: state.EditFailure{Rule: state.RulePathNotFound, Detail: "/nowhere"},
			en:      "The folder does not exist: /nowhere",
			es:      "La carpeta no existe: /nowhere",
		},
		{
			name:    "not a git repository",
			failure: state.EditFailure{Rule: state.RuleNotAGitRepository, Detail: "/plain"},
			en:      "The folder is not a git repository: /plain",
			es:      "La carpeta no es un repositorio git: /plain",
		},
		{
			name:    "not the root",
			failure: state.EditFailure{Rule: state.RulePathNotRepositoryRoot, Detail: "/repo/sub"},
			en:      "The new path is not the root of a git repository: /repo/sub",
			es:      "La nueva ruta no es la raíz de un repositorio git: /repo/sub",
		},
		{
			name:    "another project",
			failure: state.EditFailure{Rule: state.RulePathNotSameProject, Detail: "/other"},
			en:      "The new path is not the same project as the workspace: /other",
			es:      "La nueva ruta no es el mismo proyecto que el workspace: /other",
		},
		{
			name:    "already registered",
			failure: state.EditFailure{Rule: state.RulePathAlreadyRegistered, Detail: "/moved"},
			en:      "The path is already registered by another workspace: /moved",
			es:      "La ruta ya está registrada por otro workspace: /moved",
		},
		{
			name:    "same project unconfirmed",
			failure: state.EditFailure{Rule: state.RuleSameProjectUnconfirmed},
			en:      "The previous folder no longer exists, so the new folder cannot be compared; confirm it is the same project",
			es:      "La carpeta anterior ya no existe, así que la nueva carpeta no se puede comparar; confirma que es el mismo proyecto",
		},
		{
			name:    "saved with a repair notice",
			failure: state.EditFailure{Rule: state.RuleWorktreeRepair, Detail: "boom"},
			en:      "The path was saved, but git could not repair the worktrees: boom",
			es:      "La ruta se guardó, pero git no pudo reparar los worktrees: boom",
		},
		{
			name:    "any other error is shown as it is",
			failure: state.EditFailure{Message: "error de git: unreadable"},
			en:      "error de git: unreadable",
			es:      "error de git: unreadable",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			en := renderIn(t, i18n.En, EditRejection(tc.failure))
			es := renderIn(t, i18n.Es, EditRejection(tc.failure))

			assert.Contains(t, en, templ.EscapeString(tc.en))
			assert.Contains(t, es, templ.EscapeString(tc.es))
		})
	}
}
