package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
	"iworkflow/internal/core/i18n"
)

// renderHTML draws a component in English and returns its HTML.
func renderHTML(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

func TestBusyButtonIsBusyAndInertWhileWaiting(t *testing.T) {
	html := renderHTML(t, components.BusyButton(components.BusyButtonProps{
		Busy:  true,
		Attrs: templ.Attributes{"hx-post": "/save"},
	}))
	require.Contains(t, html, `class="btn-secondary is-busy"`)
	require.Contains(t, html, `aria-busy="true"`)
	require.Contains(t, html, `disabled`)
	require.Contains(t, html, `hx-post="/save"`)
}

func TestBusyButtonIdleKeepsItsBaseClass(t *testing.T) {
	html := renderHTML(t, components.BusyButton(components.BusyButtonProps{Class: "btn-primary"}))
	require.Contains(t, html, `class="btn-primary"`)
	require.NotContains(t, html, "disabled")
}

func TestSpinnerAndLoadingAreAccessible(t *testing.T) {
	require.Contains(t, renderHTML(t, components.Spinner()), `aria-label="Loading"`)
	require.Contains(t, renderHTML(t, components.Loading("Fetching")), "Fetching")
}

func TestDropdownMenuStartsClosedAndCarriesItsEntries(t *testing.T) {
	html := renderHTML(t, components.DropdownMenu(components.DropdownMenuProps{
		Label: "+", AriaLabel: "New tab", TriggerClass: "dock-icon-btn", MenuClass: "min-w-52",
	}))
	require.Contains(t, html, `data-menu-trigger`)
	require.Contains(t, html, `aria-label="New tab"`)
	require.Contains(t, html, `display: none`, "the panel starts hidden")
	require.Contains(t, html, `role="menu"`)
}

func TestCollapsePanelRendersTheCollapsedStateAndPersistKey(t *testing.T) {
	open := renderHTML(t, components.CollapsePanel(components.CollapsePanelProps{
		Side: components.LeftPanel, Open: true, Label: "Sidebar", PersistKey: components.SidebarStorageKey,
	}))
	require.Contains(t, open, `data-persist-key="iwf-sidebar-open"`)
	require.NotContains(t, open, "data-collapsed")
	require.Contains(t, open, `aria-expanded="true"`)

	closed := renderHTML(t, components.CollapsePanel(components.CollapsePanelProps{Side: components.RightPanel, Open: false}))
	require.Contains(t, closed, "data-collapsed")
	require.Contains(t, closed, "‹", "a closed right panel points left to expand")
}

func TestAppShellHasTheSidebarRailAndMobileHeader(t *testing.T) {
	html := renderHTML(t, components.AppShell(components.AppShellProps{
		Brand:     "iworkflow",
		HomeReuse: "board",
		Sidebar:   templ.Raw("<nav>sidebar</nav>"),
		Content:   templ.Raw("<div>content</div>"),
	}))
	require.Contains(t, html, "<nav>sidebar</nav>")
	require.Contains(t, html, "<div>content</div>")
	require.Contains(t, html, `data-sidebar-rail`)
	require.Contains(t, html, `data-dock-reuse="board"`)
	require.Contains(t, html, `aria-label="Open menu"`)
}

func TestModalIsHiddenUnlessOpen(t *testing.T) {
	closed := renderHTML(t, components.Modal(components.ModalProps{Title: "Delete", Open: false}))
	require.Contains(t, closed, "display: none")
	require.Contains(t, closed, `data-modal-close`)

	open := renderHTML(t, components.Modal(components.ModalProps{Title: "Delete", Open: true}))
	require.NotContains(t, open, "display: none")
	require.Contains(t, open, "<h2 id=\"modal-title\" class=\"text-lg font-semibold text-fg\">Delete</h2>")
}

func TestToastOutOfBandAppendsToTheStack(t *testing.T) {
	html := renderHTML(t, components.Toast(components.ToastProps{ID: "t1", Text: "Saved", OutOfBand: true}))
	require.Contains(t, html, `hx-swap-oob="beforeend:#toasts"`)
	require.Contains(t, html, `data-toast-timeout="8000"`)
	require.Contains(t, html, "Saved")
	require.Contains(t, renderHTML(t, components.ToastStack()), `id="toasts"`)
}

func TestTextFieldEscapesItsValue(t *testing.T) {
	html := renderHTML(t, components.TextField(components.FieldProps{Label: "Name", Name: "name", Value: `<b>x</b>`}))
	require.Contains(t, html, "&lt;b&gt;x&lt;/b&gt;")
	require.NotContains(t, html, "<b>x")
}

func TestSelectFieldMarksTheSelectedOption(t *testing.T) {
	html := renderHTML(t, components.SelectField(components.SelectFieldProps{
		Label: "Agent", Name: "agent", Value: "pi", Placeholder: "Default",
		Options: []components.FormOption{{Value: "claude", Label: "Claude"}, {Value: "pi", Label: "Pi"}},
	}))
	require.Contains(t, html, `<option value="">Default</option>`)
	require.Contains(t, html, `<option value="pi" selected>Pi</option>`)
	require.Contains(t, html, `<option value="claude">Claude</option>`)
}

func TestOptionSelectGroupsAndLabelsTheCurrentChoice(t *testing.T) {
	html := renderHTML(t, components.OptionSelect(components.OptionSelectProps{
		Label: "Model", Value: "opus", DefaultLabel: "Default",
		Options: []components.SelectOption{
			{Value: "opus", Label: "Opus", Group: "Claude"},
			{Value: "sonnet", Label: "Sonnet", Group: "Claude"},
		},
	}))
	require.Contains(t, html, `title="Opus"`)
	require.Contains(t, html, `<optgroup label="Claude">`)
	require.Contains(t, html, `<option value="opus" selected>Opus</option>`)
}

// The accessible name of the select is its label text: whitespace before the sr-only label makes the name
// start with a space, and a name matched from its start (getByLabel(/^Agente/)) finds nothing.
func TestOptionSelectLabelStartsTheAccessibleNameWithoutWhitespace(t *testing.T) {
	html := renderHTML(t, components.OptionSelect(components.OptionSelectProps{
		Label: "Agente", Icon: "<svg></svg>", DefaultLabel: "Default",
	}))
	require.Contains(t, html, `<label class="select-pill" title="Default"><span class="sr-only">Agente</span>`)
}

func TestSegmentedControlSelectsOneSegment(t *testing.T) {
	html := renderHTML(t, components.SegmentedControl(components.SegmentedControlProps{
		Options: []components.SegmentOption{
			{Value: "diff", Label: "Diff"},
			{Value: "tree", Label: "Tree"},
		},
		Selected:  "tree",
		AriaLabel: "View",
	}))
	require.Contains(t, html, `class="segment segment-selected"`)
	require.Contains(t, html, `aria-selected="true"`)
}

func TestKeepAliveHidesInactivePanes(t *testing.T) {
	inactive := renderHTML(t, components.KeepAlive(components.KeepAliveProps{Src: "/dock/pane?path=%2Fx"}))
	require.Contains(t, inactive, `class="hidden"`)
	require.Contains(t, inactive, `data-src="/dock/pane?path=%2Fx"`)

	active := renderHTML(t, components.KeepAlive(components.KeepAliveProps{Active: true, Src: "/x"}))
	require.Contains(t, active, `class="contents"`)
}

func TestCodeViewCarriesTheTextForHighlighting(t *testing.T) {
	html := renderHTML(t, components.CodeView(components.CodeViewProps{Text: "a\nb", Filename: "main.rs"}))
	require.Contains(t, html, `data-code-text="a`)
	require.Contains(t, html, `data-code-filename="main.rs"`)
	require.Contains(t, html, "1\n2", "the gutter numbers both lines")
}

func TestCodeEditorMarksChangedLines(t *testing.T) {
	html := renderHTML(t, components.CodeEditor(components.CodeEditorProps{
		Name: "content", Value: "one\ntwo", Marks: []components.LineMark{components.LineAdded, ""},
	}))
	require.Contains(t, html, `<span class="gm-added">1</span>`)
	require.Contains(t, html, `<span class="gm-none">2</span>`)
	require.Contains(t, html, `aria-label="File content"`)
}

func TestLanguagePromptOnlyShowsBeforeAChoice(t *testing.T) {
	attrs := func(locale i18n.Locale) templ.Attributes {
		return templ.Attributes{"hx-post": "/lang/" + locale.Code()}
	}
	require.Contains(t, renderHTML(t, components.LanguagePrompt(false, attrs)), "Choose your language")
	require.NotContains(t, renderHTML(t, components.LanguagePrompt(true, attrs)), "Choose your language")
}

func TestLanguageCardMarksTheActiveLanguage(t *testing.T) {
	attrs := func(locale i18n.Locale) templ.Attributes {
		return templ.Attributes{"hx-post": "/lang/" + locale.Code()}
	}
	html := renderHTML(t, components.LanguageCard(i18n.De, attrs))
	require.Contains(t, html, `lang="de"`)
	require.Contains(t, html, `class="chip chip-selected"`)
	require.Contains(t, html, `hx-post="/lang/fr"`)
}

func TestThemeToggleMarksTheStoredMode(t *testing.T) {
	html := renderHTML(t, components.ThemeToggle(components.ThemeToggleProps{Selected: components.ThemeDark}))
	require.Contains(t, html, `data-theme-choice="dark"`)
	require.Contains(t, html, `aria-pressed="true"`)
	require.Contains(t, html, "☾ Dark")
}

func TestCollapsiblePanelFoldsItsBody(t *testing.T) {
	html := renderHTML(t, components.CollapsiblePanel(components.CollapsiblePanelProps{
		Title: "Files", Count: 3, Collapsed: true, ExpandLabel: "Expand", CollapseLabel: "Collapse",
	}))
	require.Contains(t, html, "side-panel side-panel-collapsed")
	require.Contains(t, html, "hidden")
	require.Contains(t, html, `aria-label="Expand: Files"`)
}

func TestConfirmPanelCarriesItsBehaviours(t *testing.T) {
	html := renderHTML(t, components.ConfirmPanel("Delete?", "Delete", false,
		templ.Attributes{"hx-post": "/delete"}, templ.Attributes{"hx-get": "/cancel"}))
	require.Contains(t, html, "Delete?")
	require.Contains(t, html, `hx-post="/delete"`)
	require.Contains(t, html, `hx-get="/cancel"`)
}

func TestProgressDialogShowsTheBusyBar(t *testing.T) {
	html := renderHTML(t, components.ProgressDialog("Pulling", "Please wait", true, templ.Attributes{"hx-post": "/stop"}))
	require.Contains(t, html, `role="progressbar"`)
	require.Contains(t, html, "In progress")
	require.Contains(t, html, `hx-post="/stop"`)
}

func TestTreeIconsSwitchWithTheirState(t *testing.T) {
	require.Contains(t, renderHTML(t, components.TreeChevron(true)), "tree-chevron-open")
	require.NotContains(t, renderHTML(t, components.TreeChevron(false)), "tree-chevron-open")
	require.NotEqual(t, renderHTML(t, components.TreeIcon(true, true)), renderHTML(t, components.TreeIcon(true, false)))
}

func TestZoomControlsShowTheLevel(t *testing.T) {
	html := renderHTML(t, components.ZoomControls(components.ZoomControlsProps{Zoom: 0.5, ZoomInLabel: "Zoom in", ZoomOutLabel: "Zoom out"}))
	require.Contains(t, html, `<span class="zoom-readout" aria-live="polite">50%</span>`)
}
