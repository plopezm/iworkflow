package components_test

import (
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func TestModalPanelKeepsTheDefaultIdWhenNoIDIsGiven(t *testing.T) {
	html := renderHTML(t, components.Modal(components.ModalProps{Title: "Pick", Open: true}))

	require.Contains(t, html, `id="modal-panel"`)
	require.Contains(t, html, `data-modal-panel`)
}

func TestModalPanelUsesTheIDItIsGiven(t *testing.T) {
	html := renderHTML(t, components.Modal(components.ModalProps{
		Title:      "Image",
		Open:       false,
		ID:         "chat-image-modal-panel",
		CloseAttrs: templ.Attributes{"data-chat-image-modal": ""},
	}))

	require.Contains(t, html, `id="chat-image-modal-panel"`)
	require.NotContains(t, html, `id="modal-panel"`)
	require.Contains(t, html, `data-modal-panel`, "the panel keeps the attribute the focus helpers use")
}
