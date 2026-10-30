package dto_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

func formRequest(method string, target string, form url.Values) *http.Request {
	if form == nil {
		return httptest.NewRequest(method, target, nil)
	}
	request := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestDecodeTaskFileFormReadsTheSaveFields(t *testing.T) {
	t.Parallel()
	form := url.Values{"scope": {"s1"}, "path": {"src/a.rs"}, "content": {"fn b() {}"}, "base": {"abc"}}

	got, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodPost, "/tasks/t/file", form))

	require.NoError(t, err)
	assert.Equal(t, dto.TaskFileForm{Scope: "s1", Path: "src/a.rs", Content: "fn b() {}", Base: "abc"}, got)
}

func TestDecodeTaskFileFormReadsTheLoadFieldsFromTheQuery(t *testing.T) {
	t.Parallel()

	got, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodGet, "/tasks/t/file?scope=s2&path=a%20b.rs", nil))

	require.NoError(t, err)
	assert.Equal(t, dto.TaskFileForm{Scope: "s2", Path: "a b.rs"}, got)
}

func TestDecodeTaskFileFormRejectsContentAboveTheFileLimit(t *testing.T) {
	t.Parallel()
	form := url.Values{"path": {"big.txt"}, "content": {strings.Repeat("a", models.MaxTaskFileBytes+1)}}

	_, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodPost, "/tasks/t/file", form))

	assert.ErrorIs(t, err, dto.ErrTaskFileTooLarge)
}

func TestDecodeTaskFileFormCapsTheBodyBeforeReadingIt(t *testing.T) {
	t.Parallel()
	form := url.Values{"path": {"big.txt"}, "content": {strings.Repeat("a", int(dto.TaskFileBodyLimit))}}

	_, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodPost, "/tasks/t/file", form))

	assert.ErrorIs(t, err, dto.ErrTaskFileTooLarge)
}

func TestTaskFileBodyLimitFitsTheLargestFileEncodedAsUTF8Form(t *testing.T) {
	t.Parallel()
	// A three-byte character is nine bytes once form-encoded, so the largest file must still fit.
	form := url.Values{"path": {"cjk.txt"}, "content": {strings.Repeat("中", models.MaxTaskFileBytes/3)}}

	_, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodPost, "/tasks/t/file", form))

	assert.NoError(t, err)
}

// A save above the limit is refused, and the form it sent is still returned, so the editor can show what was typed.
func TestDecodeTaskFileFormKeepsTheTypedFormWhenTheContentIsTooLarge(t *testing.T) {
	t.Parallel()
	form := url.Values{"scope": {"s1"}, "path": {"big.txt"}, "content": {"keep me" + strings.Repeat("a", models.MaxTaskFileBytes)}, "base": {"b1"}}

	got, err := dto.DecodeTaskFileForm(httptest.NewRecorder(), formRequest(http.MethodPost, "/tasks/t/file", form))

	assert.ErrorIs(t, err, dto.ErrTaskFileTooLarge)
	assert.Equal(t, "big.txt", got.Path)
	assert.Equal(t, "s1", got.Scope)
	assert.Equal(t, "b1", got.Base)
	assert.True(t, strings.HasPrefix(got.Content, "keep me"))
}
