package inbound_test

import (
	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound"
)

func chiRouter(handlers *inbound.Handlers) chi.Router {
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}
