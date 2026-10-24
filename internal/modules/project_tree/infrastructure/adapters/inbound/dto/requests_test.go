package dto

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/project_tree/domain/models"
)

var workspace = uuid.MustParse("00000000-0000-0000-0000-0000000000c1")

func postForm(t *testing.T, values url.Values) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestParseScopeReadsBothWireForms(t *testing.T) {
	scope, err := ParseScope(workspace.String())
	require.NoError(t, err)
	require.Equal(t, models.WorkspaceScope(workspace), scope)

	scope, err = ParseScope("task:" + workspace.String())
	require.NoError(t, err)
	require.Equal(t, models.TaskScope(workspace), scope)
}

func TestParseScopeRejectsGarbageWithTheRustText(t *testing.T) {
	_, err := ParseScope("nope")
	require.ErrorIs(t, err, ErrInvalidScope)
	require.Equal(t, "id de workspace inválido", err.Error())
}

func TestDecodeTargetReadsTheQueryString(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x?"+url.Values{"scope": {workspace.String()}, "path": {"src/a b.rs"}}.Encode(), nil)

	target, err := DecodeTarget(req)

	require.NoError(t, err)
	require.Equal(t, models.WorkspaceScope(workspace), target.Scope)
	require.Equal(t, "src/a b.rs", target.Path)
}

func TestDecodeFileFormNormalizesCRLFToLF(t *testing.T) {
	form, err := DecodeFileForm(postForm(t, url.Values{
		"scope":   {"task:" + workspace.String()},
		"path":    {"a.txt"},
		"content": {"uno\r\ndos\n"},
	}))

	require.NoError(t, err)
	require.Equal(t, models.TaskScope(workspace), form.Scope)
	require.Equal(t, "a.txt", form.Path)
	require.Equal(t, "uno\ndos\n", form.Content)
}

func TestDecodePathsFormKeepsEveryRepeatedPath(t *testing.T) {
	form, err := DecodePathsForm(postForm(t, url.Values{
		"scope": {workspace.String()},
		"paths": {"a.rs", "b/c.rs"},
	}))

	require.NoError(t, err)
	require.Equal(t, []string{"a.rs", "b/c.rs"}, form.Paths)
}

func TestDecodePathsFormWithoutPathsMeansEverything(t *testing.T) {
	form, err := DecodePathsForm(postForm(t, url.Values{"scope": {workspace.String()}}))

	require.NoError(t, err)
	require.Empty(t, form.Paths)
}

func TestDecodersRejectAnInvalidScope(t *testing.T) {
	bad := url.Values{"scope": {"nope"}, "message": {"m"}, "q": {"x"}}

	_, err := DecodeFileForm(postForm(t, bad))
	require.ErrorIs(t, err, ErrInvalidScope)
	_, err = DecodePathsForm(postForm(t, bad))
	require.ErrorIs(t, err, ErrInvalidScope)
	_, err = DecodeCommitForm(postForm(t, bad))
	require.ErrorIs(t, err, ErrInvalidScope)
	_, err = DecodeSearchQuery(postForm(t, bad))
	require.ErrorIs(t, err, ErrInvalidScope)
}

func TestDecodeCommitAndSearchReadTheirFields(t *testing.T) {
	commit, err := DecodeCommitForm(postForm(t, url.Values{"scope": {workspace.String()}, "message": {"fix: x"}}))
	require.NoError(t, err)
	require.Equal(t, "fix: x", commit.Message)

	search, err := DecodeSearchQuery(postForm(t, url.Values{"scope": {workspace.String()}, "q": {"READ"}}))
	require.NoError(t, err)
	require.Equal(t, "READ", search.Query)
}
