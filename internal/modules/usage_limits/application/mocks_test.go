package application_test

import (
	"context"

	"iworkflow/internal/modules/usage_limits/domain/models"
)

// fakeProvider answers a fixed usage or error.
type fakeProvider struct {
	id    models.ProviderID
	usage models.ProviderUsage
	err   error
}

func (f fakeProvider) ID() models.ProviderID { return f.id }

func (f fakeProvider) Fetch(context.Context) (models.ProviderUsage, error) { return f.usage, f.err }
