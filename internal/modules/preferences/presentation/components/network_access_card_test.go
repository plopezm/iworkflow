package components_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/preferences/presentation/components"
	"iworkflow/internal/modules/preferences/presentation/state"
)

// render renders a component in Spanish, the language the e2e script drives.
func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buf bytes.Buffer
	require.NoError(t, component.Render(ctx, &buf))
	return buf.String()
}

func privateView() state.NetworkAccessView {
	return state.NewNetworkAccessView(dto.NewNetworkAccessDTO(false, false, false, 3000))
}

func TestPrivateCardHasTheStatusToggleAndPortButNoTokenSection(t *testing.T) {
	html := render(t, components.NetworkAccessCard(privateView()))

	require.Contains(t, html, `id="network-access"`)
	require.Contains(t, html, "Solo este equipo")
	require.Contains(t, html, "Permitir el acceso desde otros dispositivos de la red")
	require.Contains(t, html, `hx-post="/preferences/network"`)
	require.Contains(t, html, "Puerto del servidor")
	require.Contains(t, html, `id="network-port"`)
	require.Contains(t, html, "Guardar puerto")
	require.NotContains(t, html, "Generar un token nuevo")
	require.NotContains(t, html, "Exigir el token de acceso")
	require.NotContains(t, html, "Reiniciar ahora")
}

func TestEnvironmentLockDisablesTheControlsAndExplainsWhy(t *testing.T) {
	view := state.NewNetworkAccessView(dto.NewNetworkAccessDTO(false, false, true, 3000))

	html := render(t, components.NetworkAccessCard(view))

	require.Contains(t, html, "LEPTOS_SITE_ADDR")
	require.Contains(t, html, "disabled")
	require.NotContains(t, html, "Reiniciar ahora", "a pinned address never asks for a restart")
}

func TestPendingRestartOffersTheButton(t *testing.T) {
	view := state.NewNetworkAccessView(dto.NewNetworkAccessDTO(true, false, false, 3000))

	html := render(t, components.NetworkAccessCard(view))

	require.Contains(t, html, "Reinicia el servidor para aplicar el cambio.")
	require.Contains(t, html, `hx-post="/preferences/restart"`)
	require.Contains(t, html, "Reiniciar ahora")
}

func TestRestartingCardPollsAndHidesTheButton(t *testing.T) {
	view := state.NewNetworkAccessView(dto.NewNetworkAccessDTO(true, false, false, 3000)).WithRestarting()

	html := render(t, components.NetworkAccessCard(view))

	require.Contains(t, html, `hx-trigger="every 2s"`)
	require.Contains(t, html, "Reiniciando…")
	require.NotContains(t, html, "Reiniciar ahora")
}

func TestReachableCardShowsTheLinkQRAndTokenSection(t *testing.T) {
	token := "ab" + strings.Repeat("cd", 31)
	svg := `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
	access := dto.NewNetworkAccessDTO(true, true, false, 3000).
		WithTokenState(true, true).
		WithTargets([]dto.AccessTargetDTO{{
			Interface: "eth0",
			Address:   "192.168.1.5",
			AccessURL: "http://192.168.1.5:3000/?token=" + token,
			QRSVG:     &svg,
		}}).
		WithToken(&token)

	html := render(t, components.NetworkAccessCard(state.NewNetworkAccessView(access)))

	require.Contains(t, html, `<option value="192.168.1.5" selected>eth0 — 192.168.1.5</option>`)
	require.Contains(t, html, svg, "the QR is inlined unescaped")
	require.Contains(t, html, `role="img"`)
	require.Contains(t, html, "Generar un token nuevo")
	require.Contains(t, html, `hx-post="/preferences/token/regenerate"`)
	require.Contains(t, html, token)
	require.Contains(t, html, "Este enlace contiene el token de acceso")
}

func TestTokenOffShowsTheWarning(t *testing.T) {
	access := dto.NewNetworkAccessDTO(true, true, false, 3000).WithTokenState(false, true)

	html := render(t, components.NetworkAccessCard(state.NewNetworkAccessView(access)))

	require.Contains(t, html, `role="alert"`)
	require.Contains(t, html, "Sin el token, cualquiera en la red puede usar iworkflow")
}

func TestPlainAddressHintWhenNoInterfaceWasDetected(t *testing.T) {
	access := dto.NewNetworkAccessDTO(true, false, false, 3000)

	html := render(t, components.NetworkAccessCard(state.NewNetworkAccessView(access)))

	require.Contains(t, html, "http://&lt;IP&gt;:3000")
}

func TestInterfaceNamesAreEscaped(t *testing.T) {
	access := dto.NewNetworkAccessDTO(true, false, false, 3000).WithTargets([]dto.AccessTargetDTO{{
		Interface: `<script>alert(1)</script>`,
		Address:   "10.0.0.2",
		AccessURL: "http://10.0.0.2:3000",
	}})

	html := render(t, components.NetworkAccessCard(state.NewNetworkAccessView(access)))

	require.NotContains(t, html, "<script>alert(1)</script>")
	require.Contains(t, html, "&lt;script&gt;")
}

func TestErrorIsShownInsideTheCard(t *testing.T) {
	view := privateView().WithError("puerto no válido: 80 (usa un puerto entre 1024 y 65535)")

	html := render(t, components.NetworkAccessCard(view))

	require.Contains(t, html, "puerto no válido: 80")
	require.Contains(t, html, "text-error")
}

func TestPlaceholderLoadsTheCardOnceTheSectionIsShown(t *testing.T) {
	html := render(t, components.NetworkAccessPlaceholder())

	require.Contains(t, html, `hx-get="/preferences/network"`)
	require.Contains(t, html, `hx-trigger="load"`)
	require.Contains(t, html, `id="network-access"`)
	require.Contains(t, html, "Cargando…")
}

func TestErrorFragmentKeepsTheSwapTargetId(t *testing.T) {
	html := render(t, components.NetworkAccessError("error de almacenamiento: closed"))

	require.Contains(t, html, `id="network-access"`)
	require.Contains(t, html, "error de almacenamiento: closed")
}
