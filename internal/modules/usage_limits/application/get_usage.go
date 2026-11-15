// Package application holds the use cases of the usage_limits module.
package application

import (
	"context"
	"errors"
	"sync"

	usageerrors "iworkflow/internal/modules/usage_limits/domain/errors"
	"iworkflow/internal/modules/usage_limits/domain/models"
	"iworkflow/internal/modules/usage_limits/domain/ports/outbound"
)

// GetUsage asks every provider for its subscription usage.
type GetUsage struct {
	providers []outbound.UsageProviderPort
}

// NewGetUsage builds the use case over the providers, in the order they are shown.
func NewGetUsage(providers ...outbound.UsageProviderPort) *GetUsage {
	return &GetUsage{providers: providers}
}

type outcome struct {
	usage models.ProviderUsage
	err   error
}

// Execute reads the providers concurrently. A provider that is not configured is left out; one that fails is
// reported as unavailable so the rest still shows.
func (g *GetUsage) Execute(ctx context.Context) (models.Report, error) {
	outcomes := make([]outcome, len(g.providers))
	var wg sync.WaitGroup
	for index, provider := range g.providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			usage, err := provider.Fetch(ctx)
			outcomes[index] = outcome{usage: usage, err: err}
		}()
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return models.Report{}, err //nolint:wrapcheck // the context error is already self-describing
	}

	report := models.Report{}
	for index, result := range outcomes {
		switch {
		case result.err == nil:
			report.Usages = append(report.Usages, result.usage)
		case errors.Is(result.err, usageerrors.ErrNotConfigured):
		default:
			report.Unavailable = append(report.Unavailable, g.providers[index].ID())
		}
	}
	return report, nil
}
