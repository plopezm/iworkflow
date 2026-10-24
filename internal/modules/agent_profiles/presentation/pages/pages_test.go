package pages_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agent_profiles/presentation/pages"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

func TestProfilesPageHostsDialogAndDetail(t *testing.T) {
	ctx := spanish()
	var buffer bytes.Buffer

	err := pages.ProfilesPage(state.ProfilesView{}).Render(ctx, &buffer)

	require.NoError(t, err)
	html := buffer.String()
	require.Contains(t, html, `<div id="profiles-dialog"></div>`)
	require.Contains(t, html, `id="profiles-detail"`)
	require.Contains(t, html, `<h1 class="page-title">Perfiles</h1>`)
}

func TestModelsPageRendersGlobalRouting(t *testing.T) {
	ctx := spanish()
	var buffer bytes.Buffer

	err := pages.ModelsPage(state.ModelsView{
		Editor:     state.EditorView{FormID: "models-routing", SaveURL: "/s", RowsURL: "/r", AddSelectID: "models-add-agent"},
		NewRoleURL: "/agent-profiles/dialogs/role?scope=models",
	}).Render(ctx, &buffer)

	require.NoError(t, err)
	html := buffer.String()
	require.Contains(t, html, `<h1 class="page-title">Modelos</h1>`)
	require.Contains(t, html, "Routing global")
	require.Contains(t, html, `id="models-routing"`)
	require.Contains(t, html, `<div id="models-dialog"></div>`)
	require.Contains(t, html, "Crear rol")
}
