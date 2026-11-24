package inbound_test

import (
	"strings"

	"github.com/go-chi/chi/v5"

	"iworkflow/internal/modules/workspace_groups/infrastructure/adapters/inbound"
)

func chiRouter(handlers *inbound.Handlers) chi.Router {
	router := chi.NewRouter()
	handlers.Mount(router)
	return router
}

func countOf(haystack string, needle string) int { return strings.Count(haystack, needle) }
