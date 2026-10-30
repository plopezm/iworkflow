package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_profiles/presentation/components"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, component.Render(spanish(), &buffer))
	return buffer.String()
}

func TestProfilesListMarksSelectedAndActiveChips(t *testing.T) {
	list := state.ListView{
		ID:        "profiles-list",
		DialogID:  "profiles-dialog",
		SelectURL: "/agent-profiles/select",
		Items: []state.ListItem{
			{Name: "a", Active: true},
			{Name: "b", Selected: true},
		},
	}

	html := render(t, components.ProfilesList(list, false))

	require.Contains(t, html, `class="chip w-full justify-between"`)
	require.Contains(t, html, `class="chip chip-selected w-full justify-between"`)
	require.Contains(t, html, "Activo")
	require.Contains(t, html, `hx-vals="{&#34;selected&#34;:&#34;b&#34;}"`)
}

func TestProfilesListShowsEmptyText(t *testing.T) {
	html := render(t, components.ProfilesList(state.ListView{ID: "profiles-list", DialogID: "profiles-dialog"}, false))

	require.Contains(t, html, "Aún no hay perfiles.")
}

func TestRolesCardOutOfBandOnlyWhenRequested(t *testing.T) {
	roles := state.RolesView{Items: []state.RoleItem{{Name: "sdd-apply", Description: "Applies"}}}

	normal := render(t, components.RolesCard(roles, false))
	oob := render(t, components.RolesCard(roles, true))

	require.NotContains(t, normal, "hx-swap-oob")
	require.Contains(t, oob, `hx-swap-oob="true"`)
	require.Contains(t, normal, "sdd-apply")
}

func TestRoutingEditorKeepsUnknownValuesAndSelectors(t *testing.T) {
	editor := state.EditorView{
		FormID:      "profiles-routing",
		SaveURL:     "/save",
		RowsURL:     "/rows",
		AddSelectID: "profiles-add-agent",
		Rows:        []state.RoutingRow{{Agent: "orchestrator", Model: "legacy/model", Thinking: "high"}},
		Models:      []state.Option{{Value: "p/m", Label: "p/m"}},
		Levels:      []string{"low", "high"},
		Addable:     []string{"sdd-apply"},
	}

	html := render(t, components.RoutingEditor(editor))

	require.Contains(t, html, `<li class="card stack gap-2">`)
	require.Contains(t, html, `value="legacy/model" selected`)
	require.Contains(t, html, `value="high" selected`)
	require.Contains(t, html, `aria-label="Agente a añadir"`)
	require.Contains(t, html, "Añadir fila")
	require.Contains(t, html, "Guardar enrutado")
	require.Contains(t, html, `value="sdd-apply"`)
}

func TestDetailShowsDeleteConfirmationAndCatalogError(t *testing.T) {
	detail := state.DetailView{
		Present:       true,
		Name:          "fast",
		Active:        true,
		ConfirmDelete: true,
		Error:         "catálogo caído",
		Routing:       state.EditorView{FormID: "profiles-routing"},
		DeleteURL:     "/agent-profiles/profiles/fast",
		Selected:      "fast",
	}

	html := render(t, components.ProfileDetail(detail, false))

	require.Contains(t, html, "Eliminar el perfil «fast»?")
	require.Contains(t, html, `hx-delete="/agent-profiles/profiles/fast?selected=fast"`)
	require.Contains(t, html, "catálogo caído")
	require.Contains(t, html, "Es el perfil activo: no se puede eliminar.")
}

func TestDetailWithoutProfileShowsEmptyState(t *testing.T) {
	html := render(t, components.ProfileDetail(state.DetailView{}, true))

	require.Contains(t, html, `hx-swap-oob="true"`)
	require.Contains(t, html, "Crea o importa un perfil para empezar.")
}

func TestNameDialogDisablesSubmitWhileInvalid(t *testing.T) {
	invalid := render(t, components.NameDialog(state.NameDialogView{
		ID: "profiles-dialog", Title: "Nuevo perfil", SubmitLabel: "Aceptar", Action: "/p", Invalid: true,
	}))
	valid := render(t, components.NameDialog(state.NameDialogView{
		ID: "profiles-dialog", Title: "Nuevo perfil", SubmitLabel: "Aceptar", Action: "/p",
	}))

	require.Contains(t, invalid, `id="profiles-name-submit"`)
	require.Contains(t, invalid, "disabled")
	require.NotContains(t, valid, "disabled")
	require.Contains(t, valid, `name="name"`)
	require.Contains(t, valid, "Nombre del perfil")
}

func TestNameSubmitOutOfBandOnlyWhenRequested(t *testing.T) {
	inline := render(t, components.NameSubmit("profiles", "Aceptar", false, false))
	oob := render(t, components.NameSubmit("profiles", "Aceptar", true, true))

	require.NotContains(t, inline, "hx-swap-oob")
	require.Contains(t, oob, `hx-swap-oob="true"`)
	require.Contains(t, oob, "disabled")
}

func TestRoleDialogCarriesScopeAndHintTargets(t *testing.T) {
	html := render(t, components.RoleDialog(state.RoleDialogView{
		ID: "models-dialog", Scope: "models", SubmitLabel: "Crear rol", Action: "/agent-profiles/roles",
		HintURL: "/agent-profiles/role-hint", Invalid: true,
	}))

	require.Contains(t, html, `<input type="hidden" name="scope" value="models">`)
	require.Contains(t, html, `id="models-role-name-submit"`)
	require.Contains(t, html, `hx-target="#models-role-name-hint"`)
	require.Contains(t, html, "Nombre (slug)")
}

func TestJSONPanelIsReadOnly(t *testing.T) {
	html := render(t, components.JSONPanel(`{"kind":"x"}`))

	require.Contains(t, html, "readonly")
	require.Contains(t, html, `{&#34;kind&#34;:&#34;x&#34;}`)
}

// The live hint must fire once when htmx binds the field (trigger "load"), not only on "input": a fast
// fill right after the dialog appears happens before htmx is bound, and its input event is lost.
func TestNameHintFieldFiresOnLoadAndOnInput(t *testing.T) {
	profile := render(t, components.NameDialog(state.NameDialogView{
		ID: "profiles-dialog", Title: "Nuevo perfil", SubmitLabel: "Aceptar", Action: "/p", HintURL: "/agent-profiles/name-hint",
	}))
	role := render(t, components.RoleDialog(state.RoleDialogView{
		ID: "profiles-dialog", Scope: "profiles", SubmitLabel: "Crear rol", Action: "/r", HintURL: "/agent-profiles/role-hint",
	}))

	for name, html := range map[string]string{"profile": profile, "role": role} {
		require.Contains(t, html, `hx-trigger="input changed delay:150ms, load"`, name)
		require.Contains(t, html, `hx-get="/agent-profiles/`, name)
	}
}

// The submit button is rendered disabled for an empty name and stays enabled for a valid one; the hint
// reply swaps it out of band with the same rule.
func TestNameSubmitRendersDisabledOnlyWhenInvalid(t *testing.T) {
	empty := render(t, components.NameSubmit("profiles", "Aceptar", true, false))
	typed := render(t, components.NameSubmit("profiles", "Aceptar", false, false))

	require.Contains(t, empty, "disabled")
	require.NotContains(t, typed, "disabled")
}
