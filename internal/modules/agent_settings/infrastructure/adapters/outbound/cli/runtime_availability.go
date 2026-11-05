package cli

import (
	"context"
	"os"
	"os/exec"
	"time"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

const (
	installedProbeTimeout = 5 * time.Second
	// installedCacheTTL: probing starts a login shell per runtime and an executable rarely appears or
	// disappears, so the answer is reused far longer than a model listing.
	installedCacheTTL = 10 * time.Minute
)

var _ outbound.RuntimeAvailabilityPort = (*AgentCLIModelCatalogAdapter)(nil)

// installedEntry is the answer of a probe and when it was taken.
type installedEntry struct {
	checkedAt time.Time
	found     bool
}

// Installed reports whether the executable of runtime resolves in the login shell, the same
// environment that launches it. A runtime without a registered binary is not installed. The answer
// is reused for installedCacheTTL because every page render asks for it.
func (a *AgentCLIModelCatalogAdapter) Installed(ctx context.Context, runtime models.AgentRuntime) bool {
	bin, registered := a.bins[runtime]
	if !registered || bin == "" {
		return false
	}
	a.mu.Lock()
	entry, found := a.installed[runtime]
	a.mu.Unlock()
	if found && a.clock().Sub(entry.checkedAt) < installedCacheTTL {
		return entry.found
	}
	resolved := probeInstalled(ctx, bin, os.Environ())
	a.mu.Lock()
	a.installed[runtime] = installedEntry{checkedAt: a.clock(), found: resolved}
	a.mu.Unlock()
	return resolved
}

// probeInstalled asks the login shell whether bin resolves. A shell that cannot even start counts as not found.
func probeInstalled(ctx context.Context, bin string, vars []string) bool {
	boundedCtx, cancel := context.WithTimeout(ctx, installedProbeTimeout)
	defer cancel()
	flavor := nativeFlavor()
	invocation := loginShellInvocationFor(flavor, bin, nil, vars)
	invocation.Args = probeArgs(flavor, bin)
	command := exec.CommandContext(boundedCtx, invocation.Shell, invocation.Args...)
	command.Env = invocation.Env
	command.WaitDelay = time.Second
	return command.Run() == nil
}
