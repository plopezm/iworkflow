// Package cli lists the models that each agent executable offers. Cursor's models come from its
// ACP server, because `agent --list-models` returns ids that `session/set_model` rejects.
package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

// cursorCatalogTimeout bounds the whole handshake: `session/new` takes several seconds.
const cursorCatalogTimeout = 30 * time.Second

const (
	cursorProvider   = "cursor"
	cursorAuthMethod = "cursor_login"
	cursorClientName = "iworkflow"
	// cursorClientVersion is reported to the agent during the ACP handshake.
	cursorClientVersion = "dev"
)

// acpStep is one request of the handshake up to `session/new`, whose result carries the models.
type acpStep struct {
	method string
	params map[string]any
}

// handshakeSteps returns the requests sent in order to the ACP agent.
func handshakeSteps() []acpStep {
	return []acpStep{
		{
			method: "initialize",
			params: map[string]any{
				"protocolVersion": 1,
				"clientCapabilities": map[string]any{
					"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
					"terminal": false,
				},
				"clientInfo": map[string]any{"name": cursorClientName, "version": cursorClientVersion},
			},
		},
		{method: "authenticate", params: map[string]any{"methodId": cursorAuthMethod}},
		{method: "session/new", params: map[string]any{"cwd": os.TempDir(), "mcpServers": []any{}}},
	}
}

// listCursorModels returns the models of `agent acp` for bin, bounded by timeout. The child is
// stopped before returning.
func listCursorModels(ctx context.Context, bin string, vars []string, timeout time.Duration) ([]models.ModelRef, error) {
	boundedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := queryModels(boundedCtx, bin, vars)
	if err != nil {
		if errors.Is(boundedCtx.Err(), context.DeadlineExceeded) {
			return nil, agenterrors.Catalog("tiempo de espera agotado listando modelos") //nolint:wrapcheck // domain constructor returns a domain error
		}
		return nil, err
	}
	return result, nil
}

func queryModels(ctx context.Context, bin string, vars []string) ([]models.ModelRef, error) {
	invocation := loginShellInvocation(bin, []string{"acp"}, vars)
	childCtx, kill := context.WithCancel(ctx)
	defer kill()
	command := exec.CommandContext(childCtx, invocation.Shell, invocation.Args...)
	command.Env = invocation.Env
	command.Dir = os.TempDir()
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, agenterrors.Catalog(fmt.Sprintf("no se pudo ejecutar %s: %v", bin, err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, agenterrors.Catalog(fmt.Sprintf("no se pudo ejecutar %s: %v", bin, err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	if err := command.Start(); err != nil {
		return nil, agenterrors.Catalog(fmt.Sprintf("no se pudo ejecutar %s: %v", bin, err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	reader := bufio.NewReader(stdout)
	sessionResult, err := runHandshake(stdin, reader, bin)
	// Closing stdin shuts the agent down; the kill covers an agent that does not exit by itself.
	_ = stdin.Close()
	kill()
	// The error of Wait is the expected kill signal, not a failure of the listing.
	_ = command.Wait()
	if err != nil {
		return nil, err
	}
	return parseModels(sessionResult), nil
}

// runHandshake sends each step and returns the result of the last one (`session/new`).
func runHandshake(stdin io.Writer, reader *bufio.Reader, bin string) (json.RawMessage, error) {
	var last json.RawMessage
	for index, step := range handshakeSteps() {
		id := index + 1
		request, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": id, "method": step.method, "params": step.params,
		})
		if err != nil {
			return nil, agenterrors.Catalog(fmt.Sprintf("no se pudo codificar %s: %v", step.method, err)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		if _, err := stdin.Write(append(request, '\n')); err != nil {
			return nil, agenterrors.Catalog(fmt.Sprintf("%s cerró stdin: %v", bin, err)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		last, err = responseTo(reader, id, step.method)
		if err != nil {
			return nil, err
		}
	}
	return last, nil
}

// responseTo reads lines until the response with id arrives. JSON-RPC errors become catalog errors.
func responseTo(reader *bufio.Reader, id int, method string) (json.RawMessage, error) {
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, agenterrors.Catalog(fmt.Sprintf("error leyendo ACP: %v", readErr)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		if record, parsed := parseRecord(line); parsed && string(record["id"]) == strconv.Itoa(id) {
			if rawError, failed := record["error"]; failed {
				return nil, agenterrors.Catalog(fmt.Sprintf( //nolint:wrapcheck // domain constructor returns a domain error
					"%s falló (¿has ejecutado `agent login`?): %s", method, errorMessage(rawError)))
			}
			return record["result"], nil
		}
		if errors.Is(readErr, io.EOF) {
			return nil, agenterrors.Catalog(fmt.Sprintf("el agente terminó antes de %s", method)) //nolint:wrapcheck // domain constructor returns a domain error
		}
	}
}

// parseRecord decodes one JSON object line; anything else is not a record.
func parseRecord(line []byte) (map[string]json.RawMessage, bool) {
	var record map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimRight(line, "\r\n"), &record); err != nil || record == nil {
		return nil, false
	}
	return record, true
}

// errorMessage returns error.message when it is a string, a generic text otherwise.
func errorMessage(raw json.RawMessage) string {
	const generic = "error desconocido"
	fields, ok := objectFields(raw)
	if !ok {
		return generic
	}
	var message *string
	if err := json.Unmarshal(fields["message"], &message); err != nil || message == nil {
		return generic
	}
	return *message
}

// parseModels reads models.availableModels[].modelId from the session/new result.
func parseModels(sessionNewResult json.RawMessage) []models.ModelRef {
	result := make([]models.ModelRef, 0)
	sessionModels, ok := objectFields(sessionNewResult)
	if !ok {
		return result
	}
	available, ok := objectFields(sessionModels["models"])
	if !ok {
		return result
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(available["availableModels"], &entries); err != nil {
		return result
	}
	for _, entry := range entries {
		fields, ok := objectFields(entry)
		if !ok {
			continue
		}
		var modelID *string
		if err := json.Unmarshal(fields["modelId"], &modelID); err != nil || modelID == nil {
			continue
		}
		result = append(result, models.ModelRef{Provider: cursorProvider, Model: *modelID})
	}
	return result
}

// objectFields decodes a JSON object into its raw fields; anything else yields false.
func objectFields(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false
	}
	return fields, true
}
