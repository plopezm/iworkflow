package dto

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestParseMessageModeAcceptsOnlyTheTwoQueueModes(t *testing.T) {
	t.Parallel()
	cases := map[string]models.MessageMode{
		"steer":     models.MessageModeSteer,
		"follow_up": models.MessageModeFollowUp,
	}
	for raw, want := range cases {
		got, err := ParseMessageMode(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	for _, raw := range []string{"", "Steer", "agent", "plan", "followup", "0"} {
		_, err := ParseMessageMode(raw)
		require.Error(t, err, "%q is not a message mode", raw)
	}
}

func TestParseMessageModeRefusalIsLocalizedInEveryLanguage(t *testing.T) {
	t.Parallel()
	_, err := ParseMessageMode("nope")
	var formErr *FormError
	require.True(t, errors.As(err, &formErr), "the refusal is a form error with its texts")
	for _, locale := range []i18n.Locale{i18n.En, i18n.Es, i18n.Zh, i18n.De, i18n.Fr} {
		ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale})
		assert.NotEmpty(t, formErr.Text(ctx), locale)
	}
	en := formErr.Text(i18n.WithState(context.Background(), i18n.State{Locale: i18n.En}))
	assert.Equal(t, "The message mode is not valid.", en)
}

func TestParseMessageFormReadsTheTextModeAndTheImages(t *testing.T) {
	t.Parallel()
	request := multipartRequest(t,
		map[string]string{"message": "  hola  ", "mode": "steer"},
		map[string][]byte{"images": pngBytes},
	)
	recorder := httptest.NewRecorder()

	form, err := ParseMessageForm(recorder, request)
	require.NoError(t, err)
	input, err := form.ToDomain()
	require.NoError(t, err)

	assert.Equal(t, "hola", input.Message, "the text is trimmed as the form sends it")
	assert.Equal(t, models.MessageModeSteer, input.Mode)
	require.Len(t, input.Images, 1)
	assert.Equal(t, "image/png", input.Images[0].MimeType, "the type is sniffed from the bytes")
	assert.Equal(t, base64.StdEncoding.EncodeToString(pngBytes), input.Images[0].Data, "images travel base64 encoded")
}

func TestParseMessageFormRefusesAMissingOrUnknownMode(t *testing.T) {
	t.Parallel()
	for _, fields := range []map[string]string{
		{"message": "hola"},
		{"message": "hola", "mode": "plan"},
	} {
		request := multipartRequest(t, fields, nil)
		form, err := ParseMessageForm(httptest.NewRecorder(), request)
		require.NoError(t, err, "the body itself is well formed")
		_, err = form.ToDomain()
		require.Error(t, err, "mode %q must be refused at the adapter", fields["mode"])
	}
}

func TestParseMessageFormRejectsANonMultipartBody(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodPost, "/tasks/x/messages", nil)
	request.Header.Set("Content-Type", "text/plain")

	_, err := ParseMessageForm(httptest.NewRecorder(), request)
	require.Error(t, err)
}
