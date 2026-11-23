package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarkdownPreviewRendersTheContentAsSafeHTML(t *testing.T) {
	form := url.Values{"content": {"# Title\n\n<script>x</script>"}}
	request := httptest.NewRequest(http.MethodPost, "/markdown/preview", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	MarkdownPreview(rec, request)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<h1")
	assert.NotContains(t, rec.Body.String(), "<script>")
}
