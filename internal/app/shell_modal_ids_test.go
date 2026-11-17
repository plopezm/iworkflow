package app_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The shell renders the chat image modal on every page. Its panel must not reuse the id of the other
// dialogs, so a page with one dialog open never holds two elements with the same id.
func TestShellPageHasNoDuplicatedModalPanelId(t *testing.T) {
	rec := get(t, newRouter(t), "/")

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Equal(t, 0, strings.Count(body, `id="modal-panel"`), "the shell must not claim the default dialog id")
	require.Equal(t, 1, strings.Count(body, `id="chat-image-modal-panel"`), "the chat image modal keeps its own panel id")
	require.Equal(t, 1, strings.Count(body, `data-chat-image-modal`), "the chat image modal is still rendered")
}
