package contentsearch

import (
	"os/exec"

	"iworkflow/internal/modules/project_tree/domain/ports/outbound"
)

// New returns the ripgrep searcher when rg is on PATH and the Go walker otherwise.
func New() outbound.ContentSearchPort {
	return choose(exec.LookPath)
}

// choose is New with the executable lookup injected, so both branches are testable.
func choose(lookPath func(string) (string, error)) outbound.ContentSearchPort {
	if binary, err := lookPath("rg"); err == nil {
		return NewRipgrepSearcher(binary)
	}
	return NewWalkSearcher()
}
