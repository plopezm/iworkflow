package dto

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// dialogAnswerRequest is the request a dialog form posts, form-encoded as the browser sends it.
func dialogAnswerRequest(form url.Values) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/tasks/t/dialogs/d/answer", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestParseDialogAnswerFormReadsEachKindOfAnswer(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		form url.Values
		want models.DialogAnswer
	}{
		"value of a select":          {url.Values{"kind": {"value"}, "value": {"Sí"}}, models.ValueAnswer{Value: "Sí"}},
		"empty value of an input":    {url.Values{"kind": {"value"}, "value": {""}}, models.ValueAnswer{Value: ""}},
		"multi-line value of editor": {url.Values{"kind": {"value"}, "value": {"uno\ndos"}}, models.ValueAnswer{Value: "uno\ndos"}},
		"confirmation yes":           {url.Values{"kind": {"confirmed"}, "confirmed": {"true"}}, models.ConfirmedAnswer{Confirmed: true}},
		"confirmation no":            {url.Values{"kind": {"confirmed"}, "confirmed": {"false"}}, models.ConfirmedAnswer{Confirmed: false}},
		"cancelled":                  {url.Values{"kind": {"cancelled"}}, models.CancelledAnswer{}},
	}
	for name, tc := range cases {
		w := httptest.NewRecorder()
		got, err := ParseDialogAnswerForm(w, dialogAnswerRequest(tc.form))
		require.NoError(t, err, name)
		assert.Equal(t, tc.want, got, name)
	}
}

func TestParseDialogAnswerFormRefusesAnUnknownOrIncompleteAnswer(t *testing.T) {
	t.Parallel()
	cases := map[string]url.Values{
		"no kind":             {"value": {"x"}},
		"unknown kind":        {"kind": {"maybe"}},
		"value without field": {"kind": {"value"}},
		"confirmation empty":  {"kind": {"confirmed"}},
		"confirmation text":   {"kind": {"confirmed"}, "confirmed": {"sí"}},
	}
	for name, form := range cases {
		w := httptest.NewRecorder()
		_, err := ParseDialogAnswerForm(w, dialogAnswerRequest(form))
		require.Error(t, err, name)
		var formErr *FormError
		require.True(t, errors.As(err, &formErr), name)
	}
}

// Every refusal of the answer has its text in the five languages.
func TestDialogAnswerFormRefusalIsLocalized(t *testing.T) {
	t.Parallel()
	texts := map[i18n.Locale]string{
		i18n.En: "The answer to the dialog is not valid.",
		i18n.Es: "La respuesta al diálogo no es válida.",
		i18n.Zh: "对话框的回答无效。",
		i18n.De: "Die Antwort auf den Dialog ist ungültig.",
		i18n.Fr: "La réponse à la boîte de dialogue n’est pas valide.",
	}
	_, err := ParseDialogAnswerForm(httptest.NewRecorder(), dialogAnswerRequest(url.Values{"kind": {"maybe"}}))
	require.Error(t, err)
	var formErr *FormError
	require.True(t, errors.As(err, &formErr))
	for locale, want := range texts {
		ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
		assert.Equal(t, want, formErr.Text(ctx), locale)
	}
}

// fillReader yields an endless run of the byte x, so an oversized body is streamed instead of allocated.
type fillReader struct{}

func (fillReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// An answer whose body is over the form cap is refused as too large. Only a file part reaches the cap: net/http
// refuses a urlencoded body and a text part of a multipart body earlier, with its own limits.
func TestParseDialogAnswerFormRefusesABodyOverTheCap(t *testing.T) {
	t.Parallel()
	head := "--b\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.txt\"\r\n\r\n"
	body := io.MultiReader(strings.NewReader(head), io.LimitReader(fillReader{}, maxFormBytes+1))
	r := httptest.NewRequest(http.MethodPost, "/tasks/t/dialogs/d/answer", body)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=b")

	_, err := ParseDialogAnswerForm(httptest.NewRecorder(), r)

	assert.ErrorIs(t, err, ErrFormTooLarge)
}
