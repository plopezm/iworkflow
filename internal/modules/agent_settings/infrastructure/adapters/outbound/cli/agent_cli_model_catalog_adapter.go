package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

const (
	listModelsTimeout = 10 * time.Second
	listModelsFlag    = "--list-models"
	// modelsCacheTTL: listing models starts an agent process (and with it its MCP servers), so the
	// result is reused for this long instead of spawning one process per query.
	modelsCacheTTL = 60 * time.Second
	// claudeProvider and claudeModelAliases: Claude Code has no `--list-models`; its models are
	// chosen by alias (`claude --model`).
	claudeProvider = "anthropic"
)

var claudeModelAliases = []string{"fable", "opus", "sonnet", "haiku"}

// timeoutMessage is the text of the catalog error when a listing runs out of time.
const timeoutMessage = "tiempo de espera agotado listando modelos"

// cacheEntry is a model listing and when it was fetched.
type cacheEntry struct {
	fetchedAt time.Time
	models    []models.ModelRef
}

// AgentCLIModelCatalogAdapter lists the models of each runtime by running `<bin> --list-models`
// (Cursor uses ACP and Claude a static list). A runtime without a registered binary has no models.
type AgentCLIModelCatalogAdapter struct {
	bins    map[models.AgentRuntime]string
	timeout time.Duration
	clock   func() time.Time

	mu    sync.Mutex
	cache map[models.AgentRuntime]cacheEntry
}

var _ outbound.ModelCatalogPort = (*AgentCLIModelCatalogAdapter)(nil)

// NewAgentCLIModelCatalogAdapter builds the adapter with the binary of each runtime.
func NewAgentCLIModelCatalogAdapter(bins map[models.AgentRuntime]string) *AgentCLIModelCatalogAdapter {
	copied := make(map[models.AgentRuntime]string, len(bins))
	for runtime, bin := range bins {
		copied[runtime] = bin
	}
	return &AgentCLIModelCatalogAdapter{
		bins:    copied,
		timeout: listModelsTimeout,
		clock:   time.Now,
		cache:   make(map[models.AgentRuntime]cacheEntry),
	}
}

// List returns the models available for runtime.
func (a *AgentCLIModelCatalogAdapter) List(ctx context.Context, runtime models.AgentRuntime) ([]models.ModelRef, error) {
	if runtime == models.RuntimeClaude {
		return claudeModels(), nil
	}
	bin, registered := a.bins[runtime]
	if !registered {
		return make([]models.ModelRef, 0), nil
	}
	if runtime == models.RuntimeCursor {
		return a.cached(runtime, func() ([]models.ModelRef, error) {
			return listCursorModels(ctx, bin, os.Environ(), cursorCatalogTimeout)
		})
	}
	return a.cached(runtime, func() ([]models.ModelRef, error) {
		return a.runListModels(ctx, bin)
	})
}

// cached returns the models of runtime while they are fresh; otherwise it runs fetch and keeps the
// result. The lock is held only to read and to write: holding it while the CLI runs would make
// every other listing wait for the whole timeout. Failures are not cached.
func (a *AgentCLIModelCatalogAdapter) cached(runtime models.AgentRuntime, fetch func() ([]models.ModelRef, error)) ([]models.ModelRef, error) {
	a.mu.Lock()
	entry, found := a.cache[runtime]
	a.mu.Unlock()
	if found && a.clock().Sub(entry.fetchedAt) < modelsCacheTTL {
		return cloneModels(entry.models), nil
	}
	fetched, err := fetch()
	if err != nil {
		return nil, err //nolint:wrapcheck // catalog errors are domain errors; passed unchanged
	}
	a.mu.Lock()
	a.cache[runtime] = cacheEntry{fetchedAt: a.clock(), models: cloneModels(fetched)}
	a.mu.Unlock()
	return fetched, nil
}

// runListModels runs `<bin> --list-models` through the login shell and parses its table.
func (a *AgentCLIModelCatalogAdapter) runListModels(ctx context.Context, bin string) ([]models.ModelRef, error) {
	boundedCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	command := listModelsCommand(boundedCtx, bin, os.Environ())
	output, err := command.Output()
	if err != nil {
		if errors.Is(boundedCtx.Err(), context.DeadlineExceeded) {
			return nil, agenterrors.Catalog(timeoutMessage) //nolint:wrapcheck // domain constructor returns a domain error
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, agenterrors.Catalog(fmt.Sprintf("%s --list-models falló: %s", bin, exitErr.Stderr)) //nolint:wrapcheck // domain constructor returns a domain error
		}
		return nil, agenterrors.Catalog(fmt.Sprintf("no se pudo ejecutar %s: %v", bin, err)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return parseModelTable(string(output)), nil
}

// listModelsCommand is `<bin> --list-models` run by the login shell with a clean environment that
// only keeps the session variables of vars. The process is killed when ctx ends.
func listModelsCommand(ctx context.Context, bin string, vars []string) *exec.Cmd {
	invocation := loginShellInvocation(bin, []string{listModelsFlag}, vars)
	command := exec.CommandContext(ctx, invocation.Shell, invocation.Args...)
	command.Env = invocation.Env
	// A pipe inherited by a grandchild must not keep Output waiting after the kill.
	command.WaitDelay = time.Second
	return command
}

// parseModelTable reads the `--list-models` table: a header `provider model context max-out
// thinking images` followed by one row per model. Rows without two columns are skipped.
func parseModelTable(text string) []models.ModelRef {
	result := make([]models.ModelRef, 0)
	lines := splitLines(text)
	if len(lines) > 0 {
		lines = lines[1:] // header
	}
	for _, line := range lines {
		columns := strings.Fields(line)
		if len(columns) < 2 {
			continue
		}
		result = append(result, models.ModelRef{Provider: columns[0], Model: columns[1]})
	}
	return result
}

// splitLines splits like Rust's str::lines: a trailing newline does not add an empty line, and a
// carriage return before the newline is dropped.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSuffix(line, "\r")
	}
	return lines
}

func claudeModels() []models.ModelRef {
	result := make([]models.ModelRef, 0, len(claudeModelAliases))
	for _, alias := range claudeModelAliases {
		result = append(result, models.ModelRef{Provider: claudeProvider, Model: alias})
	}
	return result
}

func cloneModels(source []models.ModelRef) []models.ModelRef {
	result := make([]models.ModelRef, len(source))
	copy(result, source)
	return result
}
