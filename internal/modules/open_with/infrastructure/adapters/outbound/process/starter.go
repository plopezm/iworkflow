// Package process starts the applications of the launchers as detached processes.
package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
)

// Starter runs a launch spec in its own session, so the application outlives the request and does not receive
// the signals of the server. It owns one goroutine per started process that only reaps it; Wait joins them.
type Starter struct {
	reapers sync.WaitGroup
}

var _ outbound.ProcessStarter = (*Starter)(nil)

// NewStarter builds the starter.
func NewStarter() *Starter { return &Starter{} }

// Start launches the process with the server environment plus the extra variables, in spec.Dir, without
// standard streams. It returns as soon as the process started. The context only bounds the call: the process
// is deliberately not tied to it.
func (s *Starter) Start(ctx context.Context, spec models.LaunchSpec) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", openerrors.ErrLaunchFailed, err)
	}
	info, err := os.Stat(spec.Dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: the directory %s does not exist", openerrors.ErrLaunchFailed, spec.Dir)
	}
	cmd := exec.Command(spec.Path, spec.Args...) //nolint:gosec,noctx // the executable is configured by the user on purpose; the process must outlive the request
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	for _, variable := range spec.Env {
		cmd.Env = append(cmd.Env, variable.Name+"="+variable.Value)
	}
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %w", openerrors.ErrLaunchFailed, err)
	}
	s.reapers.Go(func() {
		_ = cmd.Wait() //nolint:errcheck // the exit status of a detached application is not reported anywhere
	})
	return nil
}

// Wait blocks until every started process has exited. Tests use it; the server does not stop the applications.
func (s *Starter) Wait() { s.reapers.Wait() }
