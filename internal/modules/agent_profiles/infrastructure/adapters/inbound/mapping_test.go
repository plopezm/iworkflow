package inbound

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

func TestRoutingFromViewMapsBlankFieldsToUnset(t *testing.T) {
	routing := routingFromView(map[string]state.RoutingEntryView{
		"sdd-apply": {Model: "a/b"},
		"alpha":     {Thinking: "high"},
	})

	require.Equal(t, models.RoutingEntry{Model: strPtr("a/b")}, routing["sdd-apply"])
	require.Equal(t, models.RoutingEntry{Thinking: strPtr("high")}, routing["alpha"])
}

func TestRoutingToViewReadsUnsetAsBlank(t *testing.T) {
	views := routingToView(map[string]models.RoutingEntry{"x": {}})

	require.Equal(t, state.RoutingEntryView{}, views["x"])
}

func TestRowsFromFormPairsValuesByPosition(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{
		"agent":    {"orchestrator", "sdd-apply"},
		"model":    {"p/m", ""},
		"thinking": {"", "high"},
	}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	require.NoError(t, request.ParseForm())

	rows := rowsFromForm(request)

	require.Equal(t, []state.RoutingRow{
		{Agent: "orchestrator", Model: "p/m"},
		{Agent: "sdd-apply", Thinking: "high"},
	}, rows)
}

func TestParseScopeAcceptsOnlyTheTwoScreens(t *testing.T) {
	for _, scope := range []string{"profiles", "models"} {
		parsed, err := parseScope(scope)
		require.NoError(t, err)
		require.Equal(t, scope, parsed)
	}
	_, err := parseScope("elsewhere")
	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
}

func TestScopeDialogIDFollowsTheScreen(t *testing.T) {
	require.Equal(t, "models-dialog", scopeDialogID("models"))
	require.Equal(t, "profiles-dialog", scopeDialogID("profiles"))
}

func TestStatusForMapsBusinessErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"invalid name":      {agenterrors.InvalidProfileName("x"), http.StatusBadRequest},
		"invalid routing":   {agenterrors.InvalidRouting("x"), http.StatusBadRequest},
		"invalid import":    {agenterrors.InvalidImport("x"), http.StatusBadRequest},
		"missing profile":   {agenterrors.ProfileNotFound("x"), http.StatusNotFound},
		"taken name":        {agenterrors.ProfileAlreadyExists("x"), http.StatusConflict},
		"active profile":    {agenterrors.CannotDeleteActiveProfile("x"), http.StatusConflict},
		"role exists":       {agenterrors.RoleAlreadyExists("x"), http.StatusConflict},
		"storage failure":   {agenterrors.IO("disk"), http.StatusInternalServerError},
		"wrapped not found": {fmt.Errorf("load: %w", agenterrors.ProfileNotFound("x")), http.StatusNotFound},
		"unknown error":     {errors.New("boom"), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		require.Equal(t, tc.status, statusFor(tc.err), name)
	}
}

func TestReadImportFormReadsUploadedFileBeforePastedText(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("raw", "pasted"))
	file, err := writer.CreateFormFile("file", "fast.profile.json")
	require.NoError(t, err)
	_, err = file.Write([]byte(`{"from":"file"}`))
	require.NoError(t, err)
	require.NoError(t, writer.WriteField("rename_to", "renamed"))
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	input, err := readImportForm(request)

	require.NoError(t, err)
	require.Equal(t, `{"from":"file"}`, input.Raw)
	require.Equal(t, "renamed", input.RenameTo)
}

func TestReadImportFormKeepsPastedTextWhenNoFileWasChosen(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	require.NoError(t, writer.WriteField("raw", "pasted"))
	_, err := writer.CreateFormFile("file", "")
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	input, err := readImportForm(request)

	require.NoError(t, err)
	require.Equal(t, "pasted", input.Raw)
}

func TestReadImportFormAcceptsPlainFormWithPastedText(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"raw": {"{}"}}.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	input, err := readImportForm(request)

	require.NoError(t, err)
	require.Equal(t, "{}", input.Raw)
}

func strPtr(value string) *string {
	return &value
}
