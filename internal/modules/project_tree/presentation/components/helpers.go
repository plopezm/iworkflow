package components

import (
	"context"
	"strconv"
	"strings"

	"github.com/a-h/templ"

	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/presentation/state"
)

// toggle is what a stage or unstage control does: the endpoint it posts to and its labels.
type toggle struct {
	url     string
	symbol  string
	bulk    i18n.Msg
	perFile i18n.Msg
}

// stageToggle stages paths; the posted form carries the scope and the paths.
func stageToggle() toggle {
	return toggle{url: state.PathGitStage, symbol: "+", bulk: msgStageAll, perFile: msgStageFile}
}

// unstageToggle removes paths from the index; the symbol is a true minus sign.
func unstageToggle() toggle {
	return toggle{url: state.PathGitUnstage, symbol: "−", bulk: msgUnstageAll, perFile: msgUnstageFile}
}

// oobAttrs marks an element as an out-of-band swap when oob is set. Spread it into the element.
func oobAttrs(oob bool) templ.Attributes {
	if oob {
		return templ.Attributes{"hx-swap-oob": "true"}
	}
	return nil
}

// sectionTitle is a section heading with its count, for example "Staged 2".
func sectionTitle(ctx context.Context, title i18n.Msg, count int) string {
	return title.T(ctx) + " " + strconv.Itoa(count)
}

// statusOf is the CSS class of a row whose status is known.
func statusOf(kind models.GitChangeKind) string {
	return state.StatusClass(kind, true)
}

// lineNumbers is the gutter number of every line of text.
func lineNumbers(text string) []string {
	return strings.Split(coreui.LineNumbers(text), "\n")
}

// wireOf is the scope of a view in its wire form.
func wireOf(v state.GitView) string {
	return v.Scope.Wire()
}

// gitRootID, gitBodyID, gitErrorID and gitMessageID identify the parts of one changes panel.
func gitRootID(v state.GitView) string    { return state.ID("pt-git", wireOf(v)) }
func gitBodyID(v state.GitView) string    { return state.ID("pt-git-body", wireOf(v)) }
func gitErrorID(v state.GitView) string   { return state.ID("pt-git-error", wireOf(v)) }
func gitMessageID(v state.GitView) string { return state.ID("pt-git-message", wireOf(v)) }

// pollTrigger refetches the changes while the panel is displayed: a tab hidden by the dock has no
// offset parent, so it stays quiet.
func pollTrigger() string {
	return "every " + strconv.Itoa(state.PollSeconds) + "s [this.offsetParent !== null]"
}

// stagedAttrs marks the section that holds staged changes; the panel CSS shows the commit form only
// when such a section exists.
func stagedAttrs(staged bool) templ.Attributes {
	if staged {
		return templ.Attributes{"data-git-staged": ""}
	}
	return nil
}

// rowLabel is the accessible name of a per-file stage or unstage control.
func rowLabel(ctx context.Context, t toggle, path string) string {
	return t.perFile.T(ctx) + " " + path
}

// toolbarID, editorID and gutterID identify the parts of one file editor.
func toolbarID(v state.FileView) string { return state.ID("pt-toolbar", v.ScopeWire(), v.Path) }
func gutterID(v state.FileView) string  { return state.ID("pt-gutter", v.ScopeWire(), v.Path) }
