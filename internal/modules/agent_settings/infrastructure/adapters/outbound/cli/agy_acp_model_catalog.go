// Package cli lists the models that each agent executable offers.
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

// agyCatalogTimeout bounds the handshake with `agy acp`.
const agyCatalogTimeout = 30 * time.Second

const (
	agyProvider      = "agy"
	agyClientName    = "iworkflow"
	agyClientVersion = "dev"
)

type agyAcpStep struct {
	method string
	params map[string]any
}

func agyHandshakeSteps() []agyAcpStep {
	return []agyAcpStep{
		{
			method: "initialize",
			params: map[string]any{
				"protocolVersion": 1,
				"clientCapabilities": map[string]any{
					"fs":       map[string]any{"readTextFile": false, "writeTextFile": false},
					"terminal": false,
				},
				"clientInfo": map[string]any{"name": agyClientName, "version": agyClientVersion},
			},
		},
		{method: "session/new", params: map[string]any{"cwd": os.TempDir(), "mcpServers": []any{}}},
	}
}

// listAgyModels returns the models of `agy acp` for bin, bounded by timeout.
func listAgyModels(ctx context.Context, bin string, vars []string, timeout time.Duration) ([]models.ModelRef, error) {
	boundedCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := queryAgyModels(boundedCtx, bin, vars)
	if err != nil {
		if errors.Is(boundedCtx.Err(), context.DeadlineExceeded) {
			return nil, agenterrors.Catalog("tiempo de espera agotado listando modelos") //nolint:wrapcheck // domain constructor returns a domain error
		}
		return nil, err
	}
	return result, nil
}

func queryAgyModels(ctx context.Context, bin string, vars []string) ([]models.ModelRef, error) {
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
	sessionResult, err := runAgyHandshake(stdin, reader, bin)
	_ = stdin.Close()
	kill()
	_ = command.Wait()
	if err != nil {
		return nil, err
	}
	return parseAgyModels(sessionResult), nil
}

func runAgyHandshake(stdin io.Writer, reader *bufio.Reader, bin string) (json.RawMessage, error) {
	var last json.RawMessage
	for index, step := range agyHandshakeSteps() {
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
		last, err = agyResponseTo(reader, id, step.method)
		if err != nil {
			return nil, err
		}
	}
	return last, nil
}

func agyResponseTo(reader *bufio.Reader, id int, method string) (json.RawMessage, error) {
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, agenterrors.Catalog(fmt.Sprintf("error leyendo ACP: %v", readErr)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		if record, parsed := parseAgyRecord(line); parsed && string(record["id"]) == strconv.Itoa(id) {
			if rawError, failed := record["error"]; failed {
				return nil, agenterrors.Catalog(fmt.Sprintf( //nolint:wrapcheck // domain constructor returns a domain error
					"%s falló: %s", method, errorMessage(rawError)))
			}
			return record["result"], nil
		}
		if errors.Is(readErr, io.EOF) {
			return nil, agenterrors.Catalog(fmt.Sprintf("el agente terminó antes de %s", method)) //nolint:wrapcheck // domain constructor returns a domain error
		}
	}
}

func parseAgyRecord(line []byte) (map[string]json.RawMessage, bool) {
	var record map[string]json.RawMessage
	if err := json.Unmarshal(bytes.TrimRight(line, "\r\n"), &record); err != nil || record == nil {
		return nil, false
	}
	return record, true
}

func parseAgyModels(sessionResult json.RawMessage) []models.ModelRef {
	var payload struct {
		Models struct {
			AvailableModels []struct {
				ModelID string `json:"modelId"`
			} `json:"availableModels"`
		} `json:"models"`
	}
	if err := json.Unmarshal(sessionResult, &payload); err != nil {
		return make([]models.ModelRef, 0)
	}
	result := make([]models.ModelRef, 0, len(payload.Models.AvailableModels))
	for _, m := range payload.Models.AvailableModels {
		result = append(result, models.ModelRef{Provider: agyProvider, Model: m.ModelID})
	}
	return result
}
