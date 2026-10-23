package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestParseModelsKeepsTheExactACPModelID(t *testing.T) {
	result := json.RawMessage(`{"sessionId": "s", "models": {"currentModelId": "default[]",
		"availableModels": [
			{"modelId": "default[]", "name": "Auto"},
			{"modelId": "composer-2.5[fast=true]", "name": "composer-2.5"}
		]}}`)

	require.Equal(t, []models.ModelRef{
		{Provider: "cursor", Model: "default[]"},
		{Provider: "cursor", Model: "composer-2.5[fast=true]"},
	}, parseModels(result))
}

func TestAResultWithoutModelsIsAnEmptyCatalog(t *testing.T) {
	require.Empty(t, parseModels(json.RawMessage(`{"sessionId": "s"}`)))
	require.NotNil(t, parseModels(nil))
	require.Empty(t, parseModels(nil))
}

func TestEntriesWithoutAStringModelIDAreSkipped(t *testing.T) {
	result := json.RawMessage(`{"models": {"availableModels": [{"modelId": 7}, 3, {"modelId": "ok"}]}}`)

	require.Equal(t, []models.ModelRef{{Provider: "cursor", Model: "ok"}}, parseModels(result))
}

func TestRealSessionNewCaptureListsModels(t *testing.T) {
	recording, err := os.ReadFile(filepath.Join("testdata", "cursor_acp_session.jsonl"))
	require.NoError(t, err)
	var sessionNew json.RawMessage
	for _, line := range strings.Split(string(recording), "\n") {
		payload, isResponse := strings.CutPrefix(line, "<< ")
		if !isResponse {
			continue
		}
		record, parsed := parseRecord([]byte(payload))
		if parsed && string(record["id"]) == "3" {
			sessionNew = record["result"]
			break
		}
	}
	require.NotNil(t, sessionNew, "session/new response")

	list := parseModels(sessionNew)

	require.Greater(t, len(list), 3)
	require.Contains(t, modelIDs(list), "default[]")
}

func modelIDs(list []models.ModelRef) []string {
	ids := make([]string, 0, len(list))
	for _, model := range list {
		ids = append(ids, model.Model)
	}
	return ids
}

func TestErrorMessageFallsBackToAGenericText(t *testing.T) {
	require.Equal(t, "boom", errorMessage(json.RawMessage(`{"message": "boom"}`)))
	require.Equal(t, "error desconocido", errorMessage(json.RawMessage(`{"code": 1}`)))
	require.Equal(t, "error desconocido", errorMessage(json.RawMessage(`{"message": null}`)))
	require.Equal(t, "error desconocido", errorMessage(json.RawMessage(`"boom"`)))
}

// fakeACPAgent writes an executable script that answers the ACP handshake with the given session
// result. Lines of the handshake are read from stdin, so the script behaves like `agent acp`.
func fakeACPAgent(t *testing.T, sessionNewReply string) string {
	t.Helper()
	script := `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*) echo '{"jsonrpc":"2.0","id":1,"result":{}}' ;;
    *'"method":"authenticate"'*) echo '{"jsonrpc":"2.0","id":2,"result":{}}' ;;
    *'"method":"session/new"'*) echo '` + sessionNewReply + `' ;;
  esac
done
`
	return writeExecutable(t, script)
}

func writeExecutable(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o755)) //nolint:gosec // test helper executable
	return path
}

func TestListCursorModelsRunsTheHandshakeThroughTheAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := fakeACPAgent(t, `{"jsonrpc":"2.0","id":3,"result":{"models":{"availableModels":[{"modelId":"default[]"}]}}}`)

	result, err := listCursorModels(context.Background(), bin, os.Environ(), 10*time.Second)

	require.NoError(t, err)
	require.Equal(t, []models.ModelRef{{Provider: "cursor", Model: "default[]"}}, result)
}

func TestListCursorModelsReportsJSONRPCErrorsWithTheLoginHint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := fakeACPAgent(t, `{"jsonrpc":"2.0","id":3,"error":{"message":"not logged in"}}`)

	_, err := listCursorModels(context.Background(), bin, os.Environ(), 10*time.Second)

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
	require.EqualError(t, err,
		"error de catálogo: session/new falló (¿has ejecutado `agent login`?): not logged in")
}

func TestListCursorModelsTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake agents are POSIX shell scripts")
	}
	bin := writeExecutable(t, "#!/bin/sh\nsleep 30\n")

	_, err := listCursorModels(context.Background(), bin, os.Environ(), 200*time.Millisecond)

	require.EqualError(t, err, "error de catálogo: tiempo de espera agotado listando modelos")
}

func TestHandshakeFailsWhenTheAgentExitsEarly(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))

	_, err := responseTo(reader, 1, "initialize")

	require.EqualError(t, err, "error de catálogo: el agente terminó antes de initialize")
}

func TestHandshakeSkipsNoiseAndAcceptsAResponseWithoutTrailingNewline(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("not json\n{\"id\":1.0,\"result\":{}}\n{\"id\":1,\"result\":{\"ok\":true}}"))

	result, err := responseTo(reader, 1, "initialize")

	require.NoError(t, err)
	require.JSONEq(t, `{"ok":true}`, string(result))
}

func TestHandshakeRejectsAnIdThatOnlyLooksLikeTheExpectedOne(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("{\"id\":1.0,\"result\":{}}\n"))

	_, err := responseTo(reader, 1, "initialize")

	require.ErrorIs(t, err, agenterrors.ErrCatalog)
}

func TestHandshakeStepsKeepTheACPMethodOrder(t *testing.T) {
	steps := handshakeSteps()

	require.Len(t, steps, 3)
	require.Equal(t, "initialize", steps[0].method)
	require.Equal(t, "authenticate", steps[1].method)
	require.Equal(t, "session/new", steps[2].method)
}
