package application

import (
	"context"

	"iworkflow/internal/modules/engram/domain/models"
)

// fakeMemoryStore is a manual mock of outbound.MemoryStore. It records the Projects and Recent calls it receives.
type fakeMemoryStore struct {
	projects      []models.MemoryProject
	projectsErr   error
	projectsCalls int
	recent        map[string][]models.Memory
	recentErr     error
	recentCalls   []recentCall
}

type recentCall struct {
	project string
	limit   int
}

func (f *fakeMemoryStore) Projects(_ context.Context) ([]models.MemoryProject, error) {
	f.projectsCalls++
	if f.projectsErr != nil {
		return nil, f.projectsErr
	}
	return f.projects, nil
}

func (f *fakeMemoryStore) Recent(_ context.Context, project string, limit int) ([]models.Memory, error) {
	f.recentCalls = append(f.recentCalls, recentCall{project: project, limit: limit})
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	return f.recent[project], nil
}
