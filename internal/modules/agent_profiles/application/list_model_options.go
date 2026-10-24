//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// ListModelOptions lists the models that the routing editors can select.
type ListModelOptions struct {
	models outbound.ModelOptionsPort
}

// NewListModelOptions builds the use case.
func NewListModelOptions(models outbound.ModelOptionsPort) *ListModelOptions {
	return &ListModelOptions{models: models}
}

// Execute returns the model options as the port provides them.
func (u *ListModelOptions) Execute(ctx context.Context) ([]outbound.ModelOption, error) {
	return u.models.List(ctx)
}
